package ast

import (
	"maps"
	"regexp"
	"slices"
	"strings"
)

type Node interface {
	String() string
}

var (
	_ Node = (*Route)(nil)
	_ Node = (*Slash)(nil)
	_ Node = (*Path)(nil)
	_ Node = (*RequiredSlot)(nil)
	_ Node = (*OptionalSlot)(nil)
	_ Node = (*WildcardSlot)(nil)
	_ Node = (*RegexpSlot)(nil)
)

type Routes []Route

type Route struct {
	Sections Sections
}

func (r *Route) String() string {
	return r.Sections.String()
}

func (r *Route) Precedence() int {
	last := r.Sections[len(r.Sections)-1]
	switch last.(type) {
	case *WildcardSlot, *OptionalSlot:
		return last.Priority()
	default:
		return 0
	}
}

func trimRightSlash(r *Route) *Route {
	for i := len(r.Sections) - 1; i >= 0; i-- {
		if _, ok := r.Sections[i].(*Slash); !ok {
			r.Sections = r.Sections[:i+1]
			break
		}
	}
	return r
}

func (r *Route) Expand() (routes []*Route) {
	// Clone the route
	route := &Route{
		Sections: slices.Clone(r.Sections),
	}
	for i, section := range route.Sections {
		switch s := section.(type) {
		case *OptionalSlot:
			// Create route before the optional slot
			routes = append(routes, trimRightSlash(&Route{
				Sections: route.Sections[:i],
			}))
			// Create a new route with the slot required
			route.Sections[i] = &RequiredSlot{
				Key:        s.Key,
				Delimiters: s.Delimiters,
			}
		case *WildcardSlot:
			// Create route before the wildcard slot
			routes = append(routes, trimRightSlash(&Route{
				Sections: route.Sections[:i],
			}))
		}
	}
	routes = append(routes, route)
	return routes
}

// Section of the route
type Section interface {
	Node
	Len() int
	Compare(s Section) (index int, equal bool)
	Match(path string) (index int, slots []string)
	Priority() int
}

type Sections []Section

func (sections Sections) LongestCommonPrefix(secs Sections) (lcp int) {
	max := min(len(sections), len(secs))
	for i := 0; i < max; i++ {
		index, equal := sections[i].Compare(secs[i])
		lcp += index + 1
		if !equal {
			return lcp
		}
	}
	return lcp
}

// MergeDelimiters adds the delimiters of each slot in secs to the matching slot
// in sections, up to the first section that differs.
func (sections Sections) MergeDelimiters(secs Sections) {
	for i := 0; i < min(len(sections), len(secs)); i++ {
		if _, equal := sections[i].Compare(secs[i]); !equal {
			return
		}
		if s, ok := sections[i].(Slot); ok {
			maps.Copy(s.delimiters(), secs[i].(Slot).delimiters())
		}
	}
}

func (sections Sections) At(n int) string {
	for _, section := range sections {
		switch s := section.(type) {
		case *Slash:
			if n == 0 {
				return "/"
			}
			n--
		case *Path:
			for _, char := range s.Value {
				if n == 0 {
					return string(char)
				}
				n--
			}
		case *RequiredSlot, *OptionalSlot:
			if n == 0 {
				return "{slot}"
			}
			n--
		case *WildcardSlot:
			if n == 0 {
				return "{slot*}"
			}
			n--
		case *RegexpSlot:
			if n == 0 {
				return "{slot|" + s.Pattern.String() + "}"
			}
			n--
		}
	}
	return ""
}

func (sections Sections) Len() (n int) {
	for _, section := range sections {
		n += section.Len()
	}
	return n
}

func (sections Sections) Split(at int) []Sections {
	sections = slices.Clone(sections)
	for i, section := range sections {
		if at == 0 {
			if i == 0 {
				return []Sections{sections}
			}
			return []Sections{sections[:i], sections[i:]}
		}
		p, ok := section.(*Path)
		if !ok {
			at--
			continue
		}
		for j := range p.Value {
			if at == 0 {
				left := append(slices.Clone(sections[:i]), &Path{Value: p.Value[:j]})
				right := append(Sections{&Path{Value: p.Value[j:]}}, sections[i+1:]...)
				return []Sections{left, right}
			}
			at--
		}
	}
	return []Sections{sections}
}

func (sections Sections) String() string {
	s := new(strings.Builder)
	for _, section := range sections {
		s.WriteString(section.String())
	}
	return s.String()
}

var (
	_ Section = (*Slash)(nil)
	_ Section = (*Path)(nil)
	_ Section = (*OptionalSlot)(nil)
	_ Section = (*WildcardSlot)(nil)
	_ Section = (*RegexpSlot)(nil)
)

type Slash struct {
	Value string
}

func (s *Slash) Compare(sec Section) (index int, equal bool) {
	if _, ok := sec.(*Slash); !ok {
		return -1, false
	}
	return 0, true
}

func (s *Slash) String() string {
	return "/"
}

func (p *Slash) Len() int {
	return 1
}

func (p *Slash) Match(path string) (index int, slots []string) {
	if path[0] == '/' {
		index++
	}
	return index, slots
}

func (p *Slash) Priority() int {
	return 2
}

type Path struct {
	Value string
}

func (p *Path) String() string {
	return p.Value
}

func (p *Path) Compare(sec Section) (index int, equal bool) {
	index = -1
	p2, ok := sec.(*Path)
	if !ok {
		return index, false
	}
	r1 := []rune(p.Value)
	r2 := []rune(p2.Value)
	l1 := len(r1)
	l2 := len(r2)
	max := min(l1, l2)
	for i := 0; i < max; i++ {
		if r1[i] != r2[i] {
			return index, false
		}
		index++
	}
	return index, l1 == l2
}

func (p *Path) Len() int {
	return len(p.Value)
}

func (p *Path) Match(path string) (index int, slots []string) {
	valueLen := p.Len()
	if len(path) < valueLen {
		return index, slots
	}
	prefix := strings.ToLower(path[:valueLen])
	if prefix != p.Value {
		return index, slots
	}
	return valueLen, slots
}

func (p *Path) Priority() int {
	return 2
}

type Slot interface {
	Node
	Section
	Slot() string
	delimiters() map[byte]bool
}

var (
	_ Slot = (*RequiredSlot)(nil)
	_ Slot = (*OptionalSlot)(nil)
	_ Slot = (*WildcardSlot)(nil)
	_ Slot = (*RegexpSlot)(nil)
)

type RequiredSlot struct {
	Key        string
	Delimiters map[byte]bool
}

func (s *RequiredSlot) delimiters() map[byte]bool {
	return s.Delimiters
}

// Compare a slot to another section
func (s *RequiredSlot) Compare(sec Section) (index int, equal bool) {
	switch sec.(type) {
	case *RequiredSlot, *OptionalSlot:
	default:
		return -1, false
	}
	// Different keys don't matter for comparison
	// and slots count as one character
	return 0, true
}

func (s *RequiredSlot) Len() int {
	return 1
}

func (s *RequiredSlot) Slot() string {
	return s.Key
}

func (s *RequiredSlot) String() string {
	return "{" + s.Key + "}"
}

func (s *RequiredSlot) Match(path string) (index int, slots []string) {
	i := delimiterAt(s.Delimiters, path)
	if i == 0 {
		return 0, nil
	}
	return i, []string{path[:i]}
}

func (p *RequiredSlot) Priority() int {
	return 0
}

type OptionalSlot struct {
	Key        string
	Delimiters map[byte]bool
}

func (s *OptionalSlot) delimiters() map[byte]bool {
	return s.Delimiters
}

func (s *OptionalSlot) Len() int {
	return 1
}

// Compare a slot to another section
func (s *OptionalSlot) Compare(sec Section) (index int, equal bool) {
	if _, ok := sec.(Slot); !ok {
		return -1, false
	}
	// Different keys don't matter for comparison
	// and slots count as one character.
	return 0, true
}

func (s *OptionalSlot) Slot() string {
	return s.Key
}

func (o *OptionalSlot) String() string {
	return "{" + o.Key + "?}"
}

func (s *OptionalSlot) Match(path string) (index int, slots []string) {
	return 0, slots
}

func (p *OptionalSlot) Priority() int {
	return -1
}

type WildcardSlot struct {
	Key        string
	Delimiters map[byte]bool
}

func (s *WildcardSlot) delimiters() map[byte]bool {
	return s.Delimiters
}

func (s *WildcardSlot) Len() int {
	return 1
}

// Compare a slot to another section
func (s *WildcardSlot) Compare(sec Section) (index int, equal bool) {
	if _, ok := sec.(*WildcardSlot); !ok {
		return -1, false
	}
	// Different keys don't matter for comparison
	// and slots count as one character.
	return 0, true
}

func (s *WildcardSlot) Slot() string {
	return s.Key
}

func (w *WildcardSlot) String() string {
	return "{" + w.Key + "*}"
}

func (s *WildcardSlot) Match(path string) (index int, slots []string) {
	slots = append(slots, path)
	return len(path), slots
}

func (p *WildcardSlot) Priority() int {
	return -2
}

type RegexpSlot struct {
	Key        string
	Pattern    *regexp.Regexp
	Delimiters map[byte]bool
}

func (s *RegexpSlot) delimiters() map[byte]bool {
	return s.Delimiters
}

func (s *RegexpSlot) Len() int {
	return 1
}

// Compare a slot to another section
func (s *RegexpSlot) Compare(sec Section) (index int, equal bool) {
	s2, ok := sec.(*RegexpSlot)
	if !ok || s.Pattern.String() != s2.Pattern.String() {
		return -1, false
	}
	// Different keys don't matter for comparison
	// and slots count as one character.
	return 0, true
}

func (s *RegexpSlot) Slot() string {
	return s.Key
}

func (r *RegexpSlot) String() string {
	return "{" + r.Key + "|" + r.Pattern.String() + "}"
}

func (s *RegexpSlot) Match(path string) (index int, slots []string) {
	i := delimiterAt(s.Delimiters, path)
	prefix := path[:i]
	if !s.Pattern.MatchString(prefix) {
		return 0, slots
	}
	slots = append(slots, prefix)
	return i, slots
}

func (p *RegexpSlot) Priority() int {
	return 1
}

func delimiterAt(delimiters map[byte]bool, path string) int {
	for i := 0; i < len(path); i++ {
		if delimiters[path[i]] {
			return i
		}
	}
	return len(path)
}
