package parser

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"unicode/utf8"

	"github.com/matthewmueller/enroute/ast"
	"github.com/matthewmueller/enroute/internal/lexer"
	"github.com/matthewmueller/enroute/internal/token"
	"slices"
)

func New(l *lexer.Lexer) *Parser {
	return &Parser{l: l}
}

func Parse(input string) (*ast.Route, error) {
	p := New(lexer.New(input))
	return p.Parse()
}

type Parser struct {
	l *lexer.Lexer
}

func (p *Parser) Parse() (*ast.Route, error) {
	return p.parseRoute()
}

func (p *Parser) tokenText() string {
	return p.l.Token.Text
}

func (p *Parser) tokenType() token.Type {
	return p.l.Token.Type
}

func (p *Parser) parseRoute() (*ast.Route, error) {
	route := new(ast.Route)
	for p.next() {
		section, err := p.parseSection()
		if err != nil {
			return nil, err
		}
		route.Sections = append(route.Sections, section)
	}
	return route, nil
}

func (p *Parser) parseSection() (ast.Section, error) {
	switch p.tokenType() {
	case token.Error:
		return nil, errors.New(p.tokenText())
	case token.Slash:
		return p.parseSlash()
	case token.Path:
		return p.parsePath()
	case token.OpenCurly:
		return p.parseSlot()
	default:
		return nil, fmt.Errorf("unexpected token %s", p.tokenType())
	}
}

func (p *Parser) parseSlash() (*ast.Slash, error) {
	return &ast.Slash{Value: "/"}, nil
}

func (p *Parser) parsePath() (*ast.Path, error) {
	return &ast.Path{
		Value: p.tokenText(),
	}, nil
}

func (p *Parser) parseSlot() (ast.Slot, error) {
	if err := p.expect(token.Slot); err != nil {
		return nil, err
	}
	key := p.tokenText()
	switch {
	case p.accept(token.Question):
		return p.parseOptionalSlot(key)
	case p.accept(token.Star):
		return p.parseWildcardSlot(key)
	case p.accept(token.Pipe):
		return p.parseRegexpSlot(key)
	default:
		return p.parseRequiredSlot(key)
	}
}

func (p *Parser) parseOptionalSlot(key string) (*ast.OptionalSlot, error) {
	node := &ast.OptionalSlot{
		Key: key,
		Delimiters: map[byte]bool{
			'/': true,
		},
	}
	if err := p.expect(token.CloseCurly); err != nil {
		return nil, err
	}
	if err := p.expect(token.End); err != nil {
		return nil, fmt.Errorf("optional slots must be at the end of the path")
	}
	switch tok := p.l.Peak(1); tok.Type {
	case token.Path:
		node.Delimiters[tok.Text[0]] = true
	case token.OpenCurly:
		return nil, &ErrSlotAfterSlot{key}
	}
	return node, nil
}

func (p *Parser) parseWildcardSlot(key string) (*ast.WildcardSlot, error) {
	node := &ast.WildcardSlot{
		Key: key,
		Delimiters: map[byte]bool{
			'/': true,
		},
	}
	if err := p.expect(token.CloseCurly); err != nil {
		return nil, err
	}
	if err := p.expect(token.End); err != nil {
		return nil, fmt.Errorf("wildcard slots must be at the end of the path")
	}
	switch tok := p.l.Peak(1); tok.Type {
	case token.Path:
		node.Delimiters[tok.Text[0]] = true
	case token.OpenCurly:
		return nil, &ErrSlotAfterSlot{key}
	}
	return node, nil
}

func (p *Parser) parseRegexpSlot(key string) (*ast.RegexpSlot, error) {
	node := &ast.RegexpSlot{
		Key: key,
		Delimiters: map[byte]bool{
			'/': true,
		},
	}
	if err := p.expect(token.Regexp); err != nil {
		return nil, err
	}
	pattern := p.tokenText()
	// Trim leading ^ and trailing $ if present
	if pattern[0] == '^' {
		pattern = pattern[1:]
	}
	if pattern[len(pattern)-1] == '$' {
		pattern = pattern[:len(pattern)-1]
	}
	regex, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return nil, err
	}
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	if minInputLen(re) == 0 {
		return nil, fmt.Errorf("regexp %q must match at least one character", pattern)
	}
	// Disallow '/' in the regexp
	if hasSlash(re) {
		return nil, fmt.Errorf("regexp %q can't contain '/'", pattern)
	}
	node.Pattern = regex
	if err := p.expect(token.CloseCurly); err != nil {
		return nil, err
	}
	switch tok := p.l.Peak(1); tok.Type {
	case token.Path:
		node.Delimiters[tok.Text[0]] = true
	case token.OpenCurly:
		return nil, &ErrSlotAfterSlot{key}
	}
	return node, nil
}

func (p *Parser) parseRequiredSlot(key string) (*ast.RequiredSlot, error) {
	node := &ast.RequiredSlot{
		Key: key,
		Delimiters: map[byte]bool{
			'/': true,
		},
	}
	if err := p.expect(token.CloseCurly); err != nil {
		return nil, err
	}
	switch tok := p.l.Peak(1); tok.Type {
	case token.Path:
		node.Delimiters[tok.Text[0]] = true
	case token.OpenCurly:
		return nil, &ErrSlotAfterSlot{key}
	}
	return node, nil
}

func (p *Parser) next() bool {
	return p.l.Next()
}

// Returns true if all the given tokens are next
func (p *Parser) check(tokens ...token.Type) bool {
	for i, token := range tokens {
		if p.l.Peak(i+1).Type != token {
			return false
		}
	}
	return true
}

// Returns true and advances lexer if all the given tokens are next
func (p *Parser) accept(tokens ...token.Type) bool {
	if !p.check(tokens...) {
		return false
	}
	for i := 0; i < len(tokens); i++ {
		p.l.Next()
	}
	return true
}

// Returns an error if all the given tokens are not next
func (p *Parser) expect(tokens ...token.Type) error {
	for i, tok := range tokens {
		peaked := p.l.Peak(i + 1)
		if peaked.Type == token.Error {
			return fmt.Errorf(peaked.Text)
		} else if peaked.Type != tok {
			return fmt.Errorf("expected %s, got %s", tok, peaked.Type)
		}
	}
	for i := 0; i < len(tokens); i++ {
		p.l.Next()
	}
	return nil
}

type ErrSlotAfterSlot struct {
	Slot string
}

func (e *ErrSlotAfterSlot) Error() string {
	return fmt.Sprintf("slot %q can't have another slot after", e.Slot)
}

// regexpMinMatchLen returns the fewest runes a string must have to match pattern.
// minInputLen walks the regexp to find the minimum length of any matchable input.
func minInputLen(re *syntax.Regexp) int {
	switch re.Op {
	default:
		return 0
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL, syntax.OpCharClass:
		return 1
	case syntax.OpLiteral:
		l := 0
		for _, r := range re.Rune {
			if r == utf8.RuneError {
				l++
			} else {
				l += utf8.RuneLen(r)
			}
		}
		return l
	case syntax.OpCapture, syntax.OpPlus:
		return minInputLen(re.Sub[0])
	case syntax.OpRepeat:
		return re.Min * minInputLen(re.Sub[0])
	case syntax.OpConcat:
		l := 0
		for _, sub := range re.Sub {
			l += minInputLen(sub)
		}
		return l
	case syntax.OpAlternate:
		l := minInputLen(re.Sub[0])
		var lnext int
		for _, sub := range re.Sub[1:] {
			lnext = minInputLen(sub)
			if lnext < l {
				l = lnext
			}
		}
		return l
	}
}

// hasSlash checks if a regexp contains the '/' character, which is disallowed
// within path segments defined by regular expressions.
func hasSlash(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpLiteral:
		// Check if any literal rune is '/'
		if slices.Contains(re.Rune, '/') {
			return true
		}

	case syntax.OpCharClass:
		// Check if the character class includes '/'
		// re.Rune contains pairs of [lo, hi] runes.
		for i := 0; i < len(re.Rune); i += 2 {
			lo, hi := re.Rune[i], re.Rune[i+1]
			if lo <= '/' && '/' <= hi {
				return true
			}
		}
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		// '.' and '[^\\n]' match '/', so they are disallowed.
		return true

	// Recursive cases: check subexpressions
	case syntax.OpCapture, syntax.OpPlus, syntax.OpRepeat, syntax.OpConcat, syntax.OpAlternate, syntax.OpQuest, syntax.OpStar:
		if slices.ContainsFunc(re.Sub, hasSlash) {
			return true
		}
	}

	return false
}
