// Package rango arma el conjunto de manos que le creemos a un rival.
//
// La semilla sale de su VPIP y su PFR. Las acciones de esta mano la achican:
// una apuesta grande deja solo valor (doble par o mejor, overpair). Así la
// equidad no se calcula contra una mano al azar.
package rango

import (
	"math"
	"sort"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/poker"
)

// Nivel es lo mínimo que la acción deja en el rango.
type Nivel int

const (
	// Cualquiera no filtra por el tablero: es la semilla preflop.
	Cualquiera Nivel = iota
	// Medio es top pair, overpair, segunda pareja o mejor.
	Medio
	// Valor es doble par o mejor, u overpair. Es lo que apuesta fuerte.
	Valor
	// Debil es lo que queda cuando un rival ajustado pasa: sin top pair.
	Debil
)

// Semilla devuelve las claves canónicas de partida.
// subio es un raise preflop; pago es un call. Sin muestra se usa un prior.
func Semilla(vpip, pfr float64, manos int, subio, pago bool) []string {
	if manos < 4 {
		switch {
		case subio:
			return top(0.16)
		case pago:
			return top(0.34)
		default:
			return top(0.32)
		}
	}
	switch {
	case subio:
		f := pfr
		if f < 0.08 {
			f = 0.08
		}
		if f > 0.40 {
			f = 0.40
		}
		return top(f)
	case pago:
		f := vpip
		if f < pfr+0.06 {
			f = pfr + 0.06
		}
		if f < 0.18 {
			f = 0.18
		}
		if f > 0.65 {
			f = 0.65
		}
		return top(f)
	default:
		f := vpip
		if f < 0.22 {
			f = 0.22
		}
		if f > 0.70 {
			f = 0.70
		}
		return top(f)
	}
}

// Achicar deja solo las manos de la semilla que encajan con el tablero y la
// acción. Si el filtro vacía el rango de valor, deja un núcleo de manos fuertes
// para no volver a muestrear al azar.
func Achicar(semilla []string, board []botsdk.Carta, nivel Nivel) []string {
	if nivel == Cualquiera || len(board) < 3 {
		return copiar(semilla)
	}
	var out []string
	visto := map[string]struct{}{}
	for _, clave := range semilla {
		if _, ok := visto[clave]; ok {
			continue
		}
		visto[clave] = struct{}{}
		mano, ok := representar(clave, board)
		if !ok {
			continue
		}
		if encaja(mano, board, nivel) {
			out = append(out, clave)
		}
	}
	if len(out) == 0 && nivel == Valor {
		return []string{"AA", "KK", "QQ", "JJ", "AKs", "AKo"}
	}
	return out
}

func copiar(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func encaja(mano botsdk.Mano, board []botsdk.Carta, nivel Nivel) bool {
	ev := poker.Mejor(mano, board)
	over := esOverpair(mano, board)
	top := esTopPair(mano, board)
	switch nivel {
	case Valor:
		return ev.Categoria >= poker.DoblePar || over
	case Medio:
		return ev.Categoria >= poker.DoblePar || over || top || esSegunda(mano, board)
	case Debil:
		return !(ev.Categoria >= poker.DoblePar || over || top)
	default:
		return true
	}
}

func esOverpair(mano botsdk.Mano, board []botsdk.Carta) bool {
	if mano[0].Rango != mano[1].Rango || len(board) == 0 {
		return false
	}
	maxB := board[0].Rango
	for _, c := range board[1:] {
		if c.Rango > maxB {
			maxB = c.Rango
		}
	}
	return mano[0].Rango > maxB
}

func esTopPair(mano botsdk.Mano, board []botsdk.Carta) bool {
	if len(board) == 0 {
		return false
	}
	maxB := board[0].Rango
	for _, c := range board[1:] {
		if c.Rango > maxB {
			maxB = c.Rango
		}
	}
	return mano[0].Rango == maxB || mano[1].Rango == maxB
}

func esSegunda(mano botsdk.Mano, board []botsdk.Carta) bool {
	if len(board) < 2 || mano[0].Rango == mano[1].Rango {
		return false
	}
	unica := map[botsdk.Rango]struct{}{}
	for _, c := range board {
		unica[c.Rango] = struct{}{}
	}
	lista := make([]int, 0, len(unica))
	for r := range unica {
		lista = append(lista, int(r))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(lista)))
	if len(lista) < 2 {
		return false
	}
	segunda := botsdk.Rango(lista[1])
	return mano[0].Rango == segunda || mano[1].Rango == segunda
}

func representar(clave string, board []botsdk.Carta) (botsdk.Mano, bool) {
	if len(clave) < 2 {
		return botsdk.Mano{}, false
	}
	hi, lo := valor(clave[0]), valor(clave[1])
	if hi < 2 || lo < 2 {
		return botsdk.Mano{}, false
	}
	ocupada := func(r botsdk.Rango, p botsdk.Palo) bool {
		for _, c := range board {
			if c.Rango == r && c.Palo == p {
				return true
			}
		}
		return false
	}
	if hi == lo {
		var palos []botsdk.Palo
		for p := botsdk.Treboles; p <= botsdk.Picas; p++ {
			if !ocupada(botsdk.Rango(hi), p) {
				palos = append(palos, p)
			}
		}
		if len(palos) < 2 {
			return botsdk.Mano{}, false
		}
		return botsdk.Mano{
			{Rango: botsdk.Rango(hi), Palo: palos[0]},
			{Rango: botsdk.Rango(lo), Palo: palos[1]},
		}, true
	}
	suited := len(clave) == 3 && clave[2] == 's'
	for p1 := botsdk.Treboles; p1 <= botsdk.Picas; p1++ {
		if ocupada(botsdk.Rango(hi), p1) {
			continue
		}
		for p2 := botsdk.Treboles; p2 <= botsdk.Picas; p2++ {
			if p1 == p2 && !suited {
				continue
			}
			if p1 != p2 && suited {
				continue
			}
			if ocupada(botsdk.Rango(lo), p2) {
				continue
			}
			return botsdk.Mano{
				{Rango: botsdk.Rango(hi), Palo: p1},
				{Rango: botsdk.Rango(lo), Palo: p2},
			}, true
		}
	}
	return botsdk.Mano{}, false
}

func top(frac float64) []string {
	todas := ordenadas()
	n := int(math.Round(frac * float64(len(todas))))
	if n < 6 {
		n = 6
	}
	if n > len(todas) {
		n = len(todas)
	}
	return todas[:n]
}

var cacheOrden []string

func ordenadas() []string {
	if cacheOrden != nil {
		return cacheOrden
	}
	type par struct {
		clave string
		p     int
	}
	var lista []par
	for hi := 14; hi >= 2; hi-- {
		for lo := hi; lo >= 2; lo-- {
			if hi == lo {
				clave := string([]byte{letra(hi), letra(lo)})
				lista = append(lista, par{clave, 500 + hi*10})
				continue
			}
			lista = append(lista, par{string([]byte{letra(hi), letra(lo), 's'}), 200 + hi*12 + lo})
			lista = append(lista, par{string([]byte{letra(hi), letra(lo), 'o'}), hi*12 + lo})
		}
	}
	sort.Slice(lista, func(i, j int) bool {
		if lista[i].p == lista[j].p {
			return lista[i].clave < lista[j].clave
		}
		return lista[i].p > lista[j].p
	})
	cacheOrden = make([]string, len(lista))
	for i, p := range lista {
		cacheOrden[i] = p.clave
	}
	return cacheOrden
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
	return s[v-2]
}
