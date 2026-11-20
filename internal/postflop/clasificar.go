// Package postflop clasifica la mano hecha, los proyectos y la textura del
// tablero. No decide la acción: solo describe, para que el plan elija la línea.
package postflop

import (
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/poker"
)

// Fuerza es un orden grueso, de nada a monstruo.
type Fuerza int

const (
	FuerzaNada Fuerza = iota
	FuerzaDebil
	FuerzaMedia
	FuerzaFuerte
	FuerzaMonstruo
)

// Clase es la lectura de nuestra mano contra el tablero.
type Clase struct {
	Fuerza    Fuerza
	Categoria poker.Categoria
	Detalle   string
	Textura   string
	Outs      int
}

// Clasificar describe la mano. Los outs salen de mirar cada carta que falta:
// cuántas completan color o escalera. Es el prior de la regla del 2 y el 4.
func Clasificar(mano botsdk.Mano, board []botsdk.Carta) Clase {
	ev := poker.Mejor(mano, board)
	c := Clase{
		Categoria: ev.Categoria,
		Textura:   Textura(board),
		Detalle:   detalle(mano, board, ev.Categoria),
	}
	if ev.Categoria < poker.Escalera {
		c.Outs = contarOuts(mano, board)
	}
	c.Fuerza = fuerzaDe(c)
	return c
}

// Textura clasifica el tablero en seca, emparejada, mojada o monotono.
func Textura(board []botsdk.Carta) string {
	if len(board) < 3 {
		return "seca"
	}
	var palos [4]int
	var rangos [15]int
	emparejada := false
	for _, carta := range board {
		palos[carta.Palo]++
		rangos[carta.Rango]++
		if rangos[carta.Rango] >= 2 {
			emparejada = true
		}
	}
	maxPalo := 0
	for _, n := range palos {
		if n > maxPalo {
			maxPalo = n
		}
	}
	if maxPalo >= 3 {
		return "monotono"
	}
	if mojado(rangos, maxPalo) {
		return "mojada"
	}
	if emparejada {
		return "emparejada"
	}
	return "seca"
}

// mojado: dos del mismo palo (hay proyecto de color) o arcoíris pero conectado.
func mojado(rangos [15]int, maxPalo int) bool {
	if maxPalo >= 2 {
		return true
	}
	var lista []int
	for r := 2; r <= 14; r++ {
		if rangos[r] > 0 {
			lista = append(lista, r)
		}
	}
	cercanos := 0
	for i := 1; i < len(lista); i++ {
		if lista[i]-lista[i-1] <= 2 {
			cercanos++
		}
	}
	return cercanos >= 1
}

func detalle(mano botsdk.Mano, board []botsdk.Carta, cat poker.Categoria) string {
	if cat != poker.Par || len(board) == 0 {
		if overcards(mano, board) {
			return "overcards"
		}
		return ""
	}
	var cuenta [15]int
	maxB := 0
	for _, c := range board {
		cuenta[c.Rango]++
		if int(c.Rango) > maxB {
			maxB = int(c.Rango)
		}
	}
	h0, h1 := int(mano[0].Rango), int(mano[1].Rango)
	if h0 == h1 {
		switch {
		case h0 > maxB:
			return "overpair"
		case h0 >= maxB-2:
			return "medio"
		default:
			return "bajo"
		}
	}
	par, kicker := 0, 0
	switch {
	case cuenta[h0] > 0:
		par, kicker = h0, h1
	case cuenta[h1] > 0:
		par, kicker = h1, h0
	default:
		return "mesa"
	}
	if par == maxB {
		if kicker >= int(botsdk.Reina) {
			return "top_bueno"
		}
		return "top_debil"
	}
	if par >= maxB-3 {
		return "medio"
	}
	return "bajo"
}

func overcards(mano botsdk.Mano, board []botsdk.Carta) bool {
	if len(board) == 0 || mano[0].Rango == mano[1].Rango {
		return false
	}
	maxB := 0
	for _, c := range board {
		if int(c.Rango) > maxB {
			maxB = int(c.Rango)
		}
	}
	return int(mano[0].Rango) > maxB && int(mano[1].Rango) > maxB
}

func fuerzaDe(c Clase) Fuerza {
	switch c.Categoria {
	case poker.EscaleraDeColor, poker.Poker, poker.FullHouse, poker.Color, poker.Escalera, poker.Trio, poker.DoblePar:
		return FuerzaMonstruo
	case poker.Par:
		switch c.Detalle {
		case "overpair", "top_bueno":
			return FuerzaFuerte
		case "top_debil", "medio":
			return FuerzaMedia
		default:
			return FuerzaDebil
		}
	default:
		switch {
		case c.Outs >= 12:
			return FuerzaFuerte
		case c.Outs >= 8:
			return FuerzaMedia
		case c.Outs >= 4 || c.Detalle == "overcards":
			return FuerzaDebil
		default:
			return FuerzaNada
		}
	}
}

// contarOuts mira cada carta que no está vista y cuenta las que nos dejan
// color o escalera. Una carta que hace las dos cuenta una sola vez.
func contarOuts(mano botsdk.Mano, board []botsdk.Carta) int {
	var vistas [15][4]bool
	for _, c := range mano {
		vistas[c.Rango][c.Palo] = true
	}
	for _, c := range board {
		vistas[c.Rango][c.Palo] = true
	}
	outs := 0
	extra := make([]botsdk.Carta, len(board)+1)
	copy(extra, board)
	for rango := botsdk.Dos; rango <= botsdk.As; rango++ {
		for palo := botsdk.Treboles; palo <= botsdk.Picas; palo++ {
			if vistas[rango][palo] {
				continue
			}
			extra[len(board)] = botsdk.Carta{Rango: rango, Palo: palo}
			ev := poker.Mejor(mano, extra)
			if ev.Categoria >= poker.Escalera {
				outs++
			}
		}
	}
	if outs > 15 {
		return 15
	}
	return outs
}

// Prior es la equidad aproximada contra un rival al azar, antes del Monte Carlo.
// Con dos cartas por venir cada out vale ~4 %; con una, ~2 %. El resto de la
// mano hecha aporta su línea de base.
func (c Clase) Prior(rivales int, cartasPorVenir int) float64 {
	base := priorHecho(c)
	if cartasPorVenir < 0 {
		cartasPorVenir = 0
	}
	if cartasPorVenir > 2 {
		cartasPorVenir = 2
	}
	porOut := 0.02 * float64(cartasPorVenir)
	draw := float64(c.Outs) * porOut
	eq := base
	if c.Categoria < poker.Par {
		eq = draw
		if c.Detalle == "overcards" && eq < 0.18 {
			eq = 0.18
		}
		if eq < 0.08 {
			eq = 0.08
		}
	} else if draw > 0 && eq < 0.85 {
		// La mano hecha ya incluye algo de equidad; el proyecto suma un poco.
		eq += draw * 0.35
	}
	if eq > 0.98 {
		eq = 0.98
	}
	if rivales > 1 && c.Fuerza < FuerzaMonstruo {
		factor := 1.0
		for i := 1; i < rivales; i++ {
			factor *= 0.75
		}
		eq *= factor
	}
	if eq < 0 {
		return 0
	}
	return eq
}

func priorHecho(c Clase) float64 {
	switch c.Categoria {
	case poker.EscaleraDeColor, poker.Poker:
		return 0.97
	case poker.FullHouse:
		return 0.92
	case poker.Color, poker.Escalera:
		return 0.84
	case poker.Trio:
		return 0.80
	case poker.DoblePar:
		return 0.72
	case poker.Par:
		switch c.Detalle {
		case "overpair":
			if c.Textura == "monotono" || c.Textura == "mojada" {
				return 0.62
			}
			return 0.76
		case "top_bueno":
			return 0.66
		case "top_debil":
			return 0.54
		case "medio":
			return 0.44
		case "bajo":
			return 0.34
		default:
			return 0.30
		}
	default:
		return 0.12
	}
}
