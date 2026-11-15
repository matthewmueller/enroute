package enroute

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/matthewmueller/enroute/ast"
	"github.com/matthewmueller/enroute/internal/parser"
)

var ErrDuplicate = errors.New("route")
var ErrNoMatch = errors.New("no match")
var ErrInvalid = errors.New("invalid route")

func New() *Tree {
	return &Tree{}
}

// Parse a route
func Parse(route string) (*ast.Route, error) {
	r, err := parser.Parse(route)
	if err != nil {
		return nil, fmt.Errorf("enroute: %w %q: %w", ErrInvalid, route, err)
	}
	return r, nil
}

type Tree struct {
	root *Node
}

// MustInsert panics if the route is invalid
func (t *Tree) MustInsert(route string, key string) {
	if err := t.Insert(route, key); err != nil {
		panic(err)
	}
}

// Insert a route that maps to a key into the tree
func (t *Tree) Insert(route string, key string) error {
	r, err := Parse(trimTrailingSlash(route))
	if err != nil {
		return err
	}
	label, precedence := r.String(), r.Precedence()
	// Expand optional and wildcard routes
	for _, route := range r.Expand() {
		leaf := &Node{Label: label, Value: key, precedence: precedence, route: route, sections: route.Sections}
		if t.root == nil {
			t.root = leaf
			continue
		}
		if err := t.root.insert(leaf); err != nil {
			return err
		}
	}
	return nil
}

type Node struct {
	Label      string
	Value      string
	precedence int
	route      *ast.Route
	sections   ast.Sections
	children   nodes
}

func (n *Node) priority() (priority int) {
	if len(n.sections) == 0 {
		return 0
	}
	return n.sections[0].Priority()
}

type nodes []*Node

func sortNodes(n nodes) {
	slices.SortFunc(n, func(a, b *Node) int { return cmp.Compare(b.priority(), a.priority()) })
}

// assign makes n the node for leaf's route
func (n *Node) assign(leaf *Node) {
	n.Label, n.Value, n.precedence, n.route = leaf.Label, leaf.Value, leaf.precedence, leaf.route
}

// insert leaf below n. leaf.sections holds the sections still to be placed.
func (n *Node) insert(leaf *Node) error {
	sections := leaf.sections
	lcp := n.sections.LongestCommonPrefix(sections)
	n.sections.MergeDelimiters(sections)
	if lcp < n.sections.Len() {
		// Split the node's sections
		parts := n.sections.Split(lcp)
		// Move the node's route into a child with the sections after the lcp.
		splitChild := *n
		splitChild.sections = parts[1]
		n.sections = parts[0]
		n.children = nodes{&splitChild}
		if lcp < sections.Len() {
			// Add a new child if we have more sections left.
			leaf.sections = sections.Split(lcp)[1]
			n.children = append(n.children, leaf)
			n.Label, n.Value, n.precedence, n.route = "", "", 0, nil
		} else {
			// Otherwise this route matches the parent.
			n.assign(leaf)
		}
		sortNodes(n.children)
		return nil
	}
	// Route already exists
	if lcp == sections.Len() {
		// This node was for a split in the tree, but doesn't have a route yet
		if n.route == nil {
			n.assign(leaf)
			return nil
		}
		oldRoute := n.route.String()
		newRoute := leaf.route.String()
		// The higher precedence route takes the node
		if n.precedence != leaf.precedence {
			if n.precedence < leaf.precedence {
				n.assign(leaf)
			}
			return nil
		}
		if newRoute == oldRoute {
			return fmt.Errorf("%w already exists %q", ErrDuplicate, oldRoute)
		}
		return fmt.Errorf("%w %q is ambiguous with %q", ErrDuplicate, leaf.Label, n.Label)
	}
	// Check children for a match
	leaf.sections = sections.Split(lcp)[1]
	for _, child := range n.children {
		if child.sections.At(0) == leaf.sections.At(0) {
			return child.insert(leaf)
		}
	}
	n.children = append(n.children, leaf)
	sortNodes(n.children)
	return nil
}

type Slot struct {
	Key   string
	Value string
}

func createSlots(r *ast.Route, slotValues []string) (slots []*Slot) {
	index := 0
	for _, section := range r.Sections {
		switch s := section.(type) {
		case ast.Slot:
			slots = append(slots, &Slot{
				Key:   s.Slot(),
				Value: slotValues[index],
			})
			index++
		}
	}
	return slots
}

// Match represents a route that matches a path
type Match struct {
	Route string
	Path  string
	Slots []*Slot
	Value string
}

func (m *Match) String() string {
	s := new(strings.Builder)
	s.WriteString(m.Route)
	if len(m.Slots) > 0 {
		s.WriteString(" ")
		for i, slot := range m.Slots {
			if i > 0 {
				s.WriteString("&")
			}
			s.WriteString(slot.Key)
			s.WriteString("=")
			s.WriteString(slot.Value)
		}
	}
	return s.String()
}

// Match a input path to a route
func (t *Tree) Match(input string) (*Match, error) {
	input = trimTrailingSlash(input)
	// A tree without any routes shouldn't panic
	if t.root == nil || len(input) == 0 || input[0] != '/' {
		return nil, fmt.Errorf("%w for %q", ErrNoMatch, input)
	}
	match, ok := t.root.match(input, []string{})
	if !ok {
		return nil, fmt.Errorf("%w for %q", ErrNoMatch, input)
	}
	match.Path = input
	return match, nil
}

func (n *Node) match(path string, slotValues []string) (*Match, bool) {
	for _, section := range n.sections {
		if len(path) == 0 {
			return nil, false
		}
		index, slots := section.Match(path)
		if index <= 0 {
			return nil, false
		}
		path = path[index:]
		slotValues = append(slotValues, slots...)
	}
	if len(path) == 0 {
		// We've reached a non-routable node
		if n.Label == "" {
			return nil, false
		}
		return &Match{
			Route: n.Label,
			Value: n.Value,
			Slots: createSlots(n.route, slotValues),
		}, true
	}
	for _, child := range n.children {
		if match, ok := child.match(path, slotValues); ok {
			return match, true
		}
	}
	return nil, false
}

// Find by a route
func (t *Tree) Find(route string) (*Node, error) {
	r, err := Parse(trimTrailingSlash(route))
	if err != nil {
		return nil, err
	} else if t.root == nil {
		return nil, fmt.Errorf("%w for %s", ErrNoMatch, route)
	}
	return t.root.find(route, r.Sections)
}

// Find by a route
func (n *Node) find(route string, sections ast.Sections) (*Node, error) {
	lcp := n.sections.LongestCommonPrefix(sections)
	if lcp < n.sections.Len() {
		return nil, fmt.Errorf("%w for %s", ErrNoMatch, route)
	}
	if lcp == sections.Len() {
		if n.Label == "" {
			return nil, fmt.Errorf("%w for %s", ErrNoMatch, route)
		}
		return n, nil
	}
	remainingSections := sections.Split(lcp)[1]
	for _, child := range n.children {
		if child.sections.At(0) == remainingSections.At(0) {
			return child.find(route, remainingSections)
		}
	}
	return nil, fmt.Errorf("%w for %s", ErrNoMatch, route)
}

// FindByPrefix finds a node by a prefix
func (t *Tree) FindByPrefix(prefix string) (*Node, error) {
	route, err := Parse(trimTrailingSlash(prefix))
	if err != nil {
		return nil, err
	} else if t.root == nil {
		return nil, fmt.Errorf("%w for %s", ErrNoMatch, route)
	}
	return t.root.findByPrefix(prefix, route.Sections)
}

func (n *Node) findByPrefix(prefix string, sections ast.Sections) (*Node, error) {
	if n.sections.Len() > sections.Len() {
		return nil, fmt.Errorf("%w for %s", ErrNoMatch, prefix)
	}
	lcp := n.sections.LongestCommonPrefix(sections)
	if lcp == 0 {
		return nil, fmt.Errorf("%w for %s", ErrNoMatch, sections)
	}
	if lcp == sections.Len() {
		if n.Label == "" {
			return nil, fmt.Errorf("%w for %s", ErrNoMatch, prefix)
		}
		return n, nil
	}
	remainingSections := sections.Split(lcp)[1]
	for _, child := range n.children {
		if child.sections.At(0) == remainingSections.At(0) && child.sections.Len() <= remainingSections.Len() {
			return child.findByPrefix(prefix, remainingSections)
		}
	}
	if lcp < n.sections.Len() {
		return nil, fmt.Errorf("%w for %s", ErrNoMatch, prefix)
	}
	return n, nil
}

func (t *Tree) String() string {
	if t.root == nil {
		return ""
	}
	return t.string(t.root, "")
}

func (t *Tree) string(n *Node, indent string) string {
	route := n.sections.String()
	mod := ""
	if n.Label != "" {
		mod = " [from=" + n.Label + "]"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%s%s%s\n", indent, route, mod)
	indent += strings.Repeat("•", len(route))
	for _, child := range n.children {
		out.WriteString(t.string(child, indent))
	}
	return out.String()
}

// Traverse the tree in depth-first order
func (t *Tree) Each(fn func(n *Node) (next bool)) {
	if t.root == nil {
		return
	}
	t.each(t.root, fn)
}

func (t *Tree) each(n *Node, fn func(n *Node) (next bool)) {
	if !fn(n) {
		return
	}
	for _, child := range n.children {
		t.each(child, fn)
	}
}

// trimTrailingSlash strips any trailing slash (e.g. /users/ => /users)
func trimTrailingSlash(input string) string {
	input = strings.TrimRight(input, "/")
	if len(input) == 0 {
		return "/"
	}
	return input
}
