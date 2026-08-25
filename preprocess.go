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

func splitNumberLetter(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		b.WriteRune(r)
		if unicode.IsDigit(r) || r == '.' {
			if i+1 < len(runes) {
				n := runes[i+1]
				if unicode.IsLetter(n) {
					if (n == 'e' || n == 'E') && i+2 < len(runes) {
						n2 := runes[i+2]
						if unicode.IsDigit(n2) || n2 == '+' || n2 == '-' {
							continue
						}
					}
					if n != 'e' && n != 'E' || i+2 >= len(runes) || !unicode.IsDigit(runes[i+2]) && runes[i+2] != '+' && runes[i+2] != '-' {
						b.WriteByte(' ')
					}
				}
			}
		}
	}
	return b.String()
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
