package enroute

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/matthewmueller/enroute/ast"
	"github.com/matthewmueller/enroute/internal/parser"
)

var ErrDuplicate = fmt.Errorf("route")
var ErrNoMatch = fmt.Errorf("no match")

func New() *Tree {
	return &Tree{}
}

// Parse a route
func Parse(route string) (*ast.Route, error) {
	return parser.Parse(route)
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
	r, err := parser.Parse(trimTrailingSlash(route))
	if err != nil {
		return err
	}
	// Expand optional and wildcard routes
	for _, route := range r.Expand() {
		if err := t.insert(route, key); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tree) insert(route *ast.Route, value string) error {
	if t.root == nil {
		t.root = &Node{
			Route:    route.String(),
			Value:    value,
			route:    route,
			sections: route.Sections,
		}
		return nil
	}
	return t.root.insert(route, value, route.Sections)
}

type Node struct {
	Route    string
	Value    string
	route    *ast.Route
	sections ast.Sections
	children nodes
}

func (n *Node) priority() (priority int) {
	if len(n.sections) == 0 {
		return 0
	}
	return n.sections[0].Priority()
}

type nodes []*Node

var _ sort.Interface = (*nodes)(nil)

func (n nodes) Len() int {
	return len(n)
}

func (n nodes) Less(i, j int) bool {
	return n[i].priority() > n[j].priority()
}

func (n nodes) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

func (n *Node) insert(route *ast.Route, path string, sections ast.Sections) error {
	lcp := n.sections.LongestCommonPrefix(sections)
	if lcp < n.sections.Len() {
		// Split the node's sections
		parts := n.sections.Split(lcp)
		// Create a new node with the parent's sections after the lcp.
		splitChild := &Node{
			Route:    n.Route,
			Value:    n.Value,
			route:    n.route,
			sections: parts[1],
			children: n.children,
		}
		n.sections = parts[0]
		n.children = nodes{splitChild}
		// Add a new child if we have more sections left.
		if lcp < sections.Len() {
			newChild := &Node{
				Route:    route.String(),
				route:    route,
				Value:    path,
				sections: sections.Split(lcp)[1],
			}
			// Replace the parent's sections with the lcp.
			n.children = append(n.children, newChild)
			n.Route = ""
			n.route = nil
			n.Value = ""
		} else {
			// Otherwise this route matches the parent. Update the parent's route and
			// path.
			n.Route = route.String()
			n.route = route
			n.Value = path
		}
		sort.Sort(n.children)
		return nil
	}
	// Route already exists
	if lcp == sections.Len() {
		if n.route == nil {
			n.Route = route.String()
			n.route = route
			n.Value = path
			return nil
		}
		oldRoute := n.Route
		newRoute := route.String()
		if oldRoute == newRoute {
			return fmt.Errorf("%w already exists %q", ErrDuplicate, oldRoute)
		}
		return fmt.Errorf("%w %q is ambiguous with %q", ErrDuplicate, newRoute, oldRoute)
	}
	// Check children for a match
	remainingSections := sections.Split(lcp)[1]
	for _, child := range n.children {
		if child.sections.At(0) == remainingSections.At(0) {
			return child.insert(route, path, remainingSections)
		}
	}
	n.children = append(n.children, &Node{
		Route:    route.String(),
		route:    route,
		Value:    path,
		sections: remainingSections,
	})
	sort.Sort(n.children)
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
	query := url.Values{}
	for _, slot := range m.Slots {
		query.Add(slot.Key, slot.Value)
	}
	if len(query) > 0 {
		s.WriteString(" ")
		s.WriteString(query.Encode())
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
		if n.Route == "" {
			return nil, false
		}
		return &Match{
			Route: n.Route,
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
	r, err := parser.Parse(trimTrailingSlash(route))
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
		if n.Route == "" {
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
	route, err := parser.Parse(trimTrailingSlash(prefix))
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
		if n.Route == "" {
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
	var mods []string
	if n.Route != "" {
		mods = append(mods, "routable="+n.Route)
	}
	mod := ""
	if len(mods) > 0 {
		mod = " [" + strings.Join(mods, ", ") + "]"
	}
	out := fmt.Sprintf("%s%s%s\n", indent, route, mod)
	for i := 0; i < len(route); i++ {
		indent += "•"
	}
	for _, child := range n.children {
		out += t.string(child, indent)
	}
	return out
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
	if input == "/" {
		return input
	}
	return strings.TrimRight(input, "/")
}
