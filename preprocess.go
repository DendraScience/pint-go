package pint

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	reDegreeSign = strings.NewReplacer("°", "degree")
	rePer        = strings.NewReplacer(" per ", "/")
	reTimes      = strings.NewReplacer("×", "*")
	rePercent    = strings.NewReplacer("%", " percent ")
	rePermille   = strings.NewReplacer("‰", " permille ")

	reMultiSpace = regexp.MustCompile(`([\w.\-+\*\\\^])\s+`)
	reSquared    = regexp.MustCompile(`([_a-zA-Z][_a-zA-Z0-9]*) squared`)
	reCubed      = regexp.MustCompile(`([_a-zA-Z][_a-zA-Z0-9]*) cubed`)
	reCubic      = regexp.MustCompile(`cubic ([_a-zA-Z][_a-zA-Z0-9]*)`)
	reSquare     = regexp.MustCompile(`square ([_a-zA-Z][_a-zA-Z0-9]*)`)
	reSq         = regexp.MustCompile(`sq ([_a-zA-Z][_a-zA-Z0-9]*)`)

	rePrettyExp = regexp.MustCompile(`(⁻?[⁰¹²³⁴⁵⁶⁷⁸⁹]+(?:[.⋅][⁰¹²³⁴⁵⁶⁷⁸⁹]+)?(?:⸍[⁰¹²³⁴⁵⁶⁷⁸⁹]+)?)`)
)

var prettySuper = map[rune]rune{
	'⁰': '0', '¹': '1', '²': '2', '³': '3', '⁴': '4',
	'⁵': '5', '⁶': '6', '⁷': '7', '⁸': '8', '⁹': '9', '⁻': '-',
}

func convertPrettyExponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if t, ok := prettySuper[r]; ok {
			b.WriteRune(t)
			continue
		}
		switch r {
		case '⋅':
			b.WriteByte('.')
		case '⸍':
			b.WriteByte('/')
		default:
			b.WriteRune(r)
		}
	}
	return "**(" + b.String() + ")"
}

func stringPreprocessor(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	s = rePer.Replace(s)
	s = reDegreeSign.Replace(s)
	s = reTimes.Replace(s)
	s = rePercent.Replace(s)
	s = rePermille.Replace(s)
	s = reMultiSpace.ReplaceAllString(s, "$1 ")
	s = reSquared.ReplaceAllString(s, "${1}**2")
	s = reCubed.ReplaceAllString(s, "${1}**3")
	s = reCubic.ReplaceAllString(s, "${1}**3")
	s = reSquare.ReplaceAllString(s, "${1}**2")
	s = reSq.ReplaceAllString(s, "${1}**2")
	s = splitNumberLetter(s)
	s = rePrettyExp.ReplaceAllStringFunc(s, convertPrettyExponent)
	s = strings.Map(func(r rune) rune {
		if r == '·' || r == '⋅' {
			return '*'
		}
		return r
	}, s)
	s = strings.ReplaceAll(s, "^", "**")
	return s
}

// splitNumberLetter inserts a space between a magnitude and a following unit
// (1hour → "1 hour") so the lexer can parse implicit multiply. It must not
// split inside an identifier: catalog names such as water_density_4C and
// meter_H2O are one token. Python Pint never runs this preprocessor on
// attribute access (ureg.water_density_4C).
func splitNumberLetter(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	runes := []rune(s)
	inIdent := false
	inNumber := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case inIdent && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'):
			// stay in the identifier
		case inNumber && (unicode.IsDigit(r) || r == '.'):
			// stay in the magnitude
		case inNumber && (r == 'e' || r == 'E') && i+1 < len(runes) &&
			(unicode.IsDigit(runes[i+1]) || runes[i+1] == '+' || runes[i+1] == '-'):
			// scientific exponent marker
		case inNumber && (r == '+' || r == '-') && i > 0 && (runes[i-1] == 'e' || runes[i-1] == 'E'):
			// sign of a scientific exponent
		case unicode.IsLetter(r) || r == '_':
			inIdent = true
			inNumber = false
		case unicode.IsDigit(r) || r == '.':
			inIdent = false
			inNumber = true
		default:
			inIdent = false
			inNumber = false
		}
		b.WriteRune(r)
		if inIdent {
			continue
		}
		if unicode.IsDigit(r) || r == '.' {
			if i+1 >= len(runes) {
				continue
			}
			n := runes[i+1]
			if !unicode.IsLetter(n) {
				continue
			}
			if isScientificExp(runes, i) {
				continue
			}
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func isScientificExp(runes []rune, i int) bool {
	n := runes[i+1]
	if n != 'e' && n != 'E' {
		return false
	}
	if i+2 >= len(runes) {
		return false
	}
	n2 := runes[i+2]
	return unicode.IsDigit(n2) || n2 == '+' || n2 == '-'
}

func isIdentStart(r rune) bool {
	if r == 0 {
		return false
	}
	return unicode.IsLetter(r) || r == '_' || r == '%' || r == '‰' ||
		(!unicode.IsSpace(r) && !unicode.IsDigit(r) && !isOperatorRune(r) && r != '[' && r != ']' && r != '(' && r != ')')
}

func isIdentPart(r rune) bool {
	if r == 0 {
		return false
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' ||
		(!unicode.IsSpace(r) && !isOperatorRune(r) && r != '[' && r != ']' && r != '(' && r != ')')
}

func isOperatorRune(r rune) bool {
	switch r {
	case '+', '-', '*', '/', '^', '=', ';', ':', ',':
		return true
	}
	return false
}
