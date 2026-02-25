package pipeline

import (
	"fmt"
	"strings"
)

// EvalCondition evaluates a condition expression string against a context map.
//
// Supported grammar:
//
//	<expr>  ::= <or>
//	<or>    ::= <and> ( "||" <and> )*
//	<and>   ::= <atom> ( "&&" <atom> )*
//	<atom>  ::= "!" <atom> | "(" <expr> ")" | <key> "==" <value> | <key> "!=" <value> | <key>
//	<key>   ::= alphanumeric + _ + .
//	<value> ::= single-quoted | double-quoted | bare word
//
// A bare key is truthy if its value in ctx is non-empty.
func EvalCondition(expr string, ctx map[string]any) (bool, error) {
	p := &condParser{input: strings.TrimSpace(expr), ctx: ctx}
	result, err := p.parseOr()
	if err != nil {
		return false, fmt.Errorf("condition %q: %w", expr, err)
	}
	return result, nil
}

type condParser struct {
	input string
	pos   int
	ctx   map[string]any
}

func (p *condParser) peek() string {
	if p.pos >= len(p.input) {
		return ""
	}
	return p.input[p.pos:]
}

func (p *condParser) skipWS() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}

func (p *condParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for {
		p.skipWS()
		if !strings.HasPrefix(p.peek(), "||") {
			break
		}
		p.pos += 2
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

func (p *condParser) parseAnd() (bool, error) {
	left, err := p.parseAtom()
	if err != nil {
		return false, err
	}
	for {
		p.skipWS()
		if !strings.HasPrefix(p.peek(), "&&") {
			break
		}
		p.pos += 2
		right, err := p.parseAtom()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

func (p *condParser) parseAtom() (bool, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return false, fmt.Errorf("unexpected end of expression")
	}
	// Negation
	if p.input[p.pos] == '!' {
		p.pos++
		v, err := p.parseAtom()
		return !v, err
	}
	// Parenthesised group
	if p.input[p.pos] == '(' {
		p.pos++
		v, err := p.parseOr()
		if err != nil {
			return false, err
		}
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return false, fmt.Errorf("expected ')'")
		}
		p.pos++
		return v, nil
	}
	// Key (possibly followed by == or !=)
	key := p.parseKey()
	if key == "" {
		return false, fmt.Errorf("expected identifier at pos %d in %q", p.pos, p.input)
	}
	p.skipWS()
	if strings.HasPrefix(p.peek(), "==") {
		p.pos += 2
		p.skipWS()
		val := p.parseValue()
		ctxVal := fmt.Sprintf("%v", p.ctx[key])
		return ctxVal == val, nil
	}
	if strings.HasPrefix(p.peek(), "!=") {
		p.pos += 2
		p.skipWS()
		val := p.parseValue()
		ctxVal := fmt.Sprintf("%v", p.ctx[key])
		return ctxVal != val, nil
	}
	// Bare key: truthy if value is non-empty
	v, ok := p.ctx[key]
	if !ok {
		return false, nil
	}
	return fmt.Sprintf("%v", v) != "", nil
}

func (p *condParser) parseKey() string {
	start := p.pos
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '.' {
			p.pos++
		} else {
			break
		}
	}
	return p.input[start:p.pos]
}

func (p *condParser) parseValue() string {
	if p.pos >= len(p.input) {
		return ""
	}
	quote := p.input[p.pos]
	if quote == '\'' || quote == '"' {
		p.pos++
		start := p.pos
		for p.pos < len(p.input) && p.input[p.pos] != quote {
			p.pos++
		}
		val := p.input[start:p.pos]
		if p.pos < len(p.input) {
			p.pos++ // consume closing quote
		}
		return val
	}
	// Bare word
	return p.parseKey()
}
