package pint

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ScaledUnits is an expression result: a numeric scale times a unit product.
type ScaledUnits struct {
	Scale float64
	Units UnitsContainer
}

func dimensionlessScale(s float64) ScaledUnits {
	return ScaledUnits{Scale: s}
}

func (s ScaledUnits) Mul(o ScaledUnits) ScaledUnits {
	return ScaledUnits{Scale: s.Scale * o.Scale, Units: s.Units.Mul(o.Units)}
}

func (s ScaledUnits) Div(o ScaledUnits) ScaledUnits {
	return ScaledUnits{Scale: s.Scale / o.Scale, Units: s.Units.Div(o.Units)}
}

func (s ScaledUnits) Pow(p float64) ScaledUnits {
	return ScaledUnits{Scale: math.Pow(s.Scale, p), Units: s.Units.Pow(p)}
}

func (s ScaledUnits) Neg() ScaledUnits {
	return ScaledUnits{Scale: -s.Scale, Units: s.Units}
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokIdent
	tokDim
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokPower
	tokLParen
	tokRParen
)

type token struct {
	kind tokenKind
	num  float64
	text string
}

type lexer struct {
	s   string
	i   int
	tok token
	err error
}

func (l *lexer) peek() rune {
	if l.i >= len(l.s) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.s[l.i:])
	return r
}

func (l *lexer) nextRune() rune {
	if l.i >= len(l.s) {
		return 0
	}
	r, n := utf8.DecodeRuneInString(l.s[l.i:])
	l.i += n
	return r
}

func (l *lexer) skipSpace() {
	for l.i < len(l.s) {
		r, n := utf8.DecodeRuneInString(l.s[l.i:])
		if !unicode.IsSpace(r) {
			return
		}
		l.i += n
	}
}

func (l *lexer) next() {
	if l.err != nil {
		l.tok = token{kind: tokEOF}
		return
	}
	l.skipSpace()
	if l.i >= len(l.s) {
		l.tok = token{kind: tokEOF}
		return
	}
	r := l.peek()
	switch r {
	case '+':
		l.nextRune()
		l.tok = token{kind: tokPlus}
		return
	case '-':
		l.nextRune()
		l.tok = token{kind: tokMinus}
		return
	case '/':
		l.nextRune()
		l.tok = token{kind: tokSlash}
		return
	case '(':
		l.nextRune()
		l.tok = token{kind: tokLParen}
		return
	case ')':
		l.nextRune()
		l.tok = token{kind: tokRParen}
		return
	case '*':
		l.nextRune()
		if l.peek() == '*' {
			l.nextRune()
			l.tok = token{kind: tokPower}
			return
		}
		l.tok = token{kind: tokStar}
		return
	case '^':
		l.nextRune()
		l.tok = token{kind: tokPower}
		return
	case '[':
		start := l.i
		l.nextRune()
		for {
			r2 := l.peek()
			if r2 == 0 {
				l.err = fmt.Errorf("unterminated dimension")
				l.tok = token{kind: tokEOF}
				return
			}
			l.nextRune()
			if r2 == ']' {
				break
			}
		}
		l.tok = token{kind: tokDim, text: l.s[start:l.i]}
		return
	}
	if unicode.IsDigit(r) || r == '.' {
		start := l.i
		l.nextRune()
		sawDot := r == '.'
		sawE := false
		for {
			r2 := l.peek()
			if unicode.IsDigit(r2) {
				l.nextRune()
				continue
			}
			if r2 == '.' && !sawDot && !sawE {
				sawDot = true
				l.nextRune()
				continue
			}
			if (r2 == 'e' || r2 == 'E') && !sawE {
				save := l.i
				l.nextRune()
				r3 := l.peek()
				if r3 == '+' || r3 == '-' {
					l.nextRune()
					r3 = l.peek()
				}
				if unicode.IsDigit(r3) {
					sawE = true
					for unicode.IsDigit(l.peek()) {
						l.nextRune()
					}
					continue
				}
				l.i = save
				break
			}
			break
		}
		num, err := strconv.ParseFloat(l.s[start:l.i], 64)
		if err != nil {
			l.err = err
			l.tok = token{kind: tokEOF}
			return
		}
		l.tok = token{kind: tokNumber, num: num, text: l.s[start:l.i]}
		return
	}
	if isIdentStart(r) {
		start := l.i
		l.nextRune()
		for isIdentPart(l.peek()) {
			l.nextRune()
		}
		text := l.s[start:l.i]
		switch strings.ToLower(text) {
		case "inf", "infinity":
			l.tok = token{kind: tokNumber, num: math.Inf(1), text: text}
			return
		case "nan":
			l.tok = token{kind: tokNumber, num: math.NaN(), text: text}
			return
		case "pi":
			// keep as ident so it can resolve to the defined constant when present
		}
		l.tok = token{kind: tokIdent, text: text}
		return
	}
	l.err = fmt.Errorf("unexpected character %q", r)
	l.tok = token{kind: tokEOF}
}

type exprParser struct {
	lx lexer
}

func parseScaledUnits(s string) (ScaledUnits, error) {
	s = strings.TrimSpace(stringPreprocessor(s))
	if s == "" {
		return dimensionlessScale(1), nil
	}
	p := &exprParser{lx: lexer{s: s}}
	p.lx.next()
	if p.lx.err != nil {
		return ScaledUnits{}, p.lx.err
	}
	v, err := p.parseExpr(0)
	if err != nil {
		return ScaledUnits{}, err
	}
	p.lx.skipSpace()
	if p.lx.tok.kind != tokEOF {
		return ScaledUnits{}, fmt.Errorf("unexpected token after expression")
	}
	if p.lx.err != nil {
		return ScaledUnits{}, p.lx.err
	}
	return v, nil
}

func parseNumeric(s string) (float64, error) {
	su, err := parseScaledUnits(s)
	if err != nil {
		return 0, err
	}
	if !su.Units.Empty() {
		return 0, fmt.Errorf("numeric expression expected, got units %s", su.Units)
	}
	return su.Scale, nil
}

func (p *exprParser) parseExpr(minBP int) (ScaledUnits, error) {
	left, err := p.parseUnary()
	if err != nil {
		return ScaledUnits{}, err
	}
	for {
		op, bp, implicit, ok := p.infixBP()
		if !ok || bp < minBP {
			return left, nil
		}
		if !implicit {
			p.lx.next()
		}
		var right ScaledUnits
		if op == tokPower {
			right, err = p.parseExpr(bp) // right-associative
		} else {
			right, err = p.parseExpr(bp + 1)
		}
		if err != nil {
			return ScaledUnits{}, err
		}
		switch op {
		case tokPlus:
			if !left.Units.Equal(right.Units) {
				return ScaledUnits{}, fmt.Errorf("cannot add %s and %s", left.Units, right.Units)
			}
			left.Scale += right.Scale
		case tokMinus:
			if !left.Units.Equal(right.Units) {
				return ScaledUnits{}, fmt.Errorf("cannot subtract %s and %s", left.Units, right.Units)
			}
			left.Scale -= right.Scale
		case tokStar:
			left = left.Mul(right)
		case tokSlash:
			left = left.Div(right)
		case tokPower:
			if !right.Units.Empty() {
				return ScaledUnits{}, fmt.Errorf("exponent must be dimensionless")
			}
			left = left.Pow(right.Scale)
		}
		if p.lx.err != nil {
			return ScaledUnits{}, p.lx.err
		}
	}
}

func (p *exprParser) infixBP() (kind tokenKind, bp int, implicit, ok bool) {
	switch p.lx.tok.kind {
	case tokPlus, tokMinus:
		return p.lx.tok.kind, 10, false, true
	case tokStar, tokSlash:
		return p.lx.tok.kind, 20, false, true
	case tokPower:
		return tokPower, 30, false, true
	case tokIdent, tokDim, tokNumber, tokLParen:
		return tokStar, 20, true, true
	default:
		return 0, 0, false, false
	}
}

func (p *exprParser) parseUnary() (ScaledUnits, error) {
	switch p.lx.tok.kind {
	case tokPlus:
		p.lx.next()
		return p.parseUnary()
	case tokMinus:
		p.lx.next()
		v, err := p.parseUnary()
		if err != nil {
			return ScaledUnits{}, err
		}
		return v.Neg(), nil
	case tokNumber:
		n := p.lx.tok.num
		p.lx.next()
		return dimensionlessScale(n), nil
	case tokIdent:
		name := p.lx.tok.text
		p.lx.next()
		return ScaledUnits{Scale: 1, Units: unitPair(name, 1)}, nil
	case tokDim:
		name := p.lx.tok.text
		p.lx.next()
		return ScaledUnits{Scale: 1, Units: unitPair(name, 1)}, nil
	case tokLParen:
		p.lx.next()
		v, err := p.parseExpr(0)
		if err != nil {
			return ScaledUnits{}, err
		}
		if p.lx.tok.kind != tokRParen {
			return ScaledUnits{}, fmt.Errorf("missing closing parenthesis")
		}
		p.lx.next()
		return v, nil
	case tokEOF:
		return ScaledUnits{}, fmt.Errorf("unexpected end of expression")
	default:
		return ScaledUnits{}, fmt.Errorf("unexpected token in expression")
	}
}
