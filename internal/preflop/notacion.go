package preflop

import (
	"fmt"
	"strings"
)

// Expandir convierte una expresión de rango en claves canónicas (AKs, TT, 72o).
//
// Acepta:
//   - exacta: "AKs", "AKo", "TT"
//   - plus: "77+" (parejas hacia arriba), "ATs+" (mismo alto, kicker hacia arriba), "KTo+"
//   - tramo: "76s-T9s" (conectoras), "A2s-A5s" (mismo alto)
func Expandir(expr string) ([]string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("preflop: expresión vacía")
	}
	if i := strings.IndexByte(expr, '-'); i >= 0 {
		return expandirTramo(strings.TrimSpace(expr[:i]), strings.TrimSpace(expr[i+1:]))
	}
	plus := strings.HasSuffix(expr, "+")
	base := strings.TrimSuffix(expr, "+")
	hi, lo, palo, err := parsearMano(base)
	if err != nil {
		return nil, err
	}
	if !plus {
		return []string{formatear(hi, lo, palo)}, nil
	}
	return expandirPlus(hi, lo, palo)
}

func expandirPlus(hi, lo int, palo byte) ([]string, error) {
	if hi == lo {
		out := make([]string, 0, 15-hi)
		for r := hi; r <= 14; r++ {
			out = append(out, formatear(r, r, 0))
		}
		return out, nil
	}
	out := make([]string, 0, hi-lo)
	for k := lo; k < hi; k++ {
		out = append(out, formatear(hi, k, palo))
	}
	return out, nil
}

func expandirTramo(a, b string) ([]string, error) {
	ahi, alo, ap, err := parsearMano(a)
	if err != nil {
		return nil, err
	}
	bhi, blo, bp, err := parsearMano(b)
	if err != nil {
		return nil, err
	}
	if ap != bp {
		return nil, fmt.Errorf("preflop: el tramo %s-%s mezcla palos", a, b)
	}
	if ahi == bhi && ap != 0 {
		desde, hasta := alo, blo
		if desde > hasta {
			desde, hasta = hasta, desde
		}
		var out []string
		for k := desde; k <= hasta; k++ {
			if k >= ahi {
				continue
			}
			out = append(out, formatear(ahi, k, ap))
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("preflop: tramo vacío %s-%s", a, b)
		}
		return out, nil
	}
	if ap != 's' || ahi-alo != bhi-blo || ahi == alo {
		return nil, fmt.Errorf("preflop: tramo no soportado %s-%s", a, b)
	}
	gap := ahi - alo
	desde, hasta := ahi, bhi
	if desde > hasta {
		desde, hasta = hasta, desde
	}
	var out []string
	for h := desde; h <= hasta; h++ {
		l := h - gap
		if l < 2 {
			return nil, fmt.Errorf("preflop: tramo %s-%s se sale de la baraja", a, b)
		}
		out = append(out, formatear(h, l, ap))
	}
	return out, nil
}

func parsearMano(s string) (hi, lo int, palo byte, err error) {
	switch len(s) {
	case 2:
		hi, lo = valor(s[0]), valor(s[1])
		if hi < 2 || hi != lo {
			return 0, 0, 0, fmt.Errorf("preflop: pareja inválida %q", s)
		}
		return hi, lo, 0, nil
	case 3:
		if s[2] != 's' && s[2] != 'o' {
			return 0, 0, 0, fmt.Errorf("preflop: mano inválida %q", s)
		}
		hi, lo = valor(s[0]), valor(s[1])
		if hi < 2 || lo < 2 || hi <= lo {
			return 0, 0, 0, fmt.Errorf("preflop: mano inválida %q", s)
		}
		return hi, lo, s[2], nil
	default:
		return 0, 0, 0, fmt.Errorf("preflop: no entiendo %q", s)
	}
}

func formatear(hi, lo int, palo byte) string {
	if hi == lo {
		return string([]byte{letra(hi), letra(lo)})
	}
	return string([]byte{letra(hi), letra(lo), palo})
}

func valor(c byte) int {
	switch c {
	case 'A':
		return 14
	case 'K':
		return 13
	case 'Q':
		return 12
	case 'J':
		return 11
	case 'T':
		return 10
	default:
		if c >= '2' && c <= '9' {
			return int(c - '0')
		}
		return -1
	}
}

func letra(v int) byte {
	const s = "23456789TJQKA"
	if v < 2 || v > 14 {
		return '?'
	}
	return s[v-2]
}
