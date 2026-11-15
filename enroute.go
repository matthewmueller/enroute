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

func New[V any]() *Tree[V] {
	return &Tree[V]{}
}

// Parse a route
func Parse(route string) (*ast.Route, error) {
	r, err := parser.Parse(route)
	if err != nil {
		return nil, fmt.Errorf("enroute: %w %q: %w", ErrInvalid, route, err)
	}
	return r, nil
}

type Tree[V any] struct {
	root   *Node[V]
	routes []*Route[V]
}

// Route is a route as it was inserted
type Route[V any] struct {
	Route string
	Value V
}

// MustInsert panics if the route is invalid
func (t *Tree[V]) MustInsert(route string, value V) {
	if err := t.Insert(route, value); err != nil {
		panic(err)
	}
}

// Insert a route that maps to a value into the tree
func (t *Tree[V]) Insert(route string, value V) error {
	r, err := Parse(trimTrailingSlash(route))
	if err != nil {
		return err
	}
	label, precedence := r.String(), r.Precedence()
	// Expand optional and wildcard routes
	routes := r.Expand()
	// Check every expanded route before inserting any, so a failed insert
	// leaves the tree unchanged
	for _, route := range routes {
		n := t.find(route.Sections)
		if n == nil || n.precedence != precedence {
			continue
		}
		if n.route.String() == route.String() {
			return fmt.Errorf("%w already exists %q", ErrDuplicate, route.String())
		}
		return fmt.Errorf("%w %q is ambiguous with %q", ErrDuplicate, label, n.Label)
	}
	for _, route := range routes {
		leaf := &Node[V]{Label: label, Value: value, precedence: precedence, route: route, sections: route.Sections}
		if t.root == nil {
			t.root = leaf
			continue
		}
		t.root.insert(leaf)
	}
	t.routes = append(t.routes, &Route[V]{Route: label, Value: value})
	return nil
}

// Routes returns each route in the order it was inserted
func (t *Tree[V]) Routes() []*Route[V] {
	return slices.Clone(t.routes)
}

type Node[V any] struct {
	Label      string
	Value      V
	precedence int
	route      *ast.Route
	sections   ast.Sections
	children   []*Node[V]
}

func (n *Node[V]) priority() (priority int) {
	if len(n.sections) == 0 {
		return 0
	}
	return n.sections[0].Priority()
}

func sortNodes[V any](n []*Node[V]) {
	slices.SortFunc(n, func(a, b *Node[V]) int { return cmp.Compare(b.priority(), a.priority()) })
}

// assign makes n the node for leaf's route
func (n *Node[V]) assign(leaf *Node[V]) {
	n.Label, n.Value, n.precedence, n.route = leaf.Label, leaf.Value, leaf.precedence, leaf.route
}

// insert leaf below n. leaf.sections holds the sections still to be placed.
// Conflicts are checked by Tree.Insert beforehand.
func (n *Node[V]) insert(leaf *Node[V]) {
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
		n.children = []*Node[V]{&splitChild}
		if lcp < sections.Len() {
			// Add a new child if we have more sections left.
			leaf.sections = sections.Split(lcp)[1]
			n.children = append(n.children, leaf)
			var zero V
			n.Label, n.Value, n.precedence, n.route = "", zero, 0, nil
		} else {
			// Otherwise this route matches the parent.
			n.assign(leaf)
		}
		sortNodes(n.children)
		return
	}
	// The route lands on this node. The higher precedence route takes it.
	if lcp == sections.Len() {
		if n.route == nil || n.precedence < leaf.precedence {
			n.assign(leaf)
		}
		return
	}
	// Check children for a match
	leaf.sections = sections.Split(lcp)[1]
	for _, child := range n.children {
		if child.sections.At(0) == leaf.sections.At(0) {
			child.insert(leaf)
			return
		}
	}
	n.children = append(n.children, leaf)
	sortNodes(n.children)
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
type Match[V any] struct {
	Route string
	Path  string
	Slots []*Slot
	Value V
}

func (m *Match[V]) String() string {
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
func (t *Tree[V]) Match(input string) (*Match[V], error) {
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

func (n *Node[V]) match(path string, slotValues []string) (*Match[V], bool) {
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
		return &Match[V]{
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

// Find the node for a route that was inserted
func (t *Tree[V]) Find(route string) (*Node[V], error) {
	r, err := Parse(trimTrailingSlash(route))
	if err != nil {
		return nil, err
	}
	// A higher precedence route can take over some of the expanded routes, so
	// check each one for a node that still belongs to this route.
	label, routes := r.String(), r.Expand()
	for i := len(routes) - 1; i >= 0; i-- {
		if n := t.find(routes[i].Sections); n != nil && n.Label == label {
			return n, nil
		}
	}
	return nil, fmt.Errorf("%w for %s", ErrNoMatch, route)
}

// find the routable node with exactly these sections
func (t *Tree[V]) find(sections ast.Sections) *Node[V] {
	if t.root == nil {
		return nil
	}
	return t.root.find(sections)
}

func (n *Node[V]) find(sections ast.Sections) *Node[V] {
	lcp := n.sections.LongestCommonPrefix(sections)
	if lcp < n.sections.Len() {
		return nil
	}
	if lcp == sections.Len() {
		if n.route == nil {
			return nil
		}
		return n
	}
	rest := sections.Split(lcp)[1]
	for _, child := range n.children {
		if child.sections.At(0) == rest.At(0) {
			return child.find(rest)
		}
	}
	return nil
}

// FindByPrefix finds a node by a prefix
func (t *Tree[V]) FindByPrefix(prefix string) (*Node[V], error) {
	route, err := Parse(trimTrailingSlash(prefix))
	if err != nil {
		return nil, err
	} else if t.root == nil {
		return nil, fmt.Errorf("%w for %s", ErrNoMatch, route)
	}
	return t.root.findByPrefix(prefix, route.Sections)
}

func (n *Node[V]) findByPrefix(prefix string, sections ast.Sections) (*Node[V], error) {
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

func (t *Tree[V]) String() string {
	if t.root == nil {
		return ""
	}
	return t.string(t.root, "")
}

func (t *Tree[V]) string(n *Node[V], indent string) string {
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
func (t *Tree[V]) Each(fn func(n *Node[V]) (next bool)) {
	if t.root == nil {
		return
	}
	t.each(t.root, fn)
}

func (t *Tree[V]) each(n *Node[V], fn func(n *Node[V]) (next bool)) {
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
