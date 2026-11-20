// Package posicion nombra la silla en una mesa corta: UTG, MP, CO, BTN, SB, BB.
package posicion

import (
	"sort"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

// De devuelve el nombre de la posición de esa silla en la mano en curso.
// Cuenta solo a quien recibió cartas. Si la silla no está, devuelve "".
func De(estado botsdk.EstadoPublico, silla int) string {
	orden := ordenar(estado)
	if len(orden) == 0 {
		return ""
	}
	nombres := nombres(len(orden))
	for i, s := range orden {
		if s == silla {
			if i < len(nombres) {
				return nombres[i]
			}
			return ""
		}
	}
	return ""
}

// Heroe es la posición de quien decide.
func Heroe(t botsdk.Turno) string {
	silla := t.Silla
	if silla < 0 {
		silla = t.Yo.PosicionSilla
	}
	return De(t.Estado, silla)
}

// Rivales son los que siguen en la mano, sin contarnos.
func Rivales(t botsdk.Turno) []string {
	ids := make([]string, 0, len(t.Estado.Jugadores))
	for _, j := range t.Estado.Jugadores {
		if j.ID == t.IDJugador || !j.Activo || !j.EnMano {
			continue
		}
		ids = append(ids, j.ID)
	}
	return ids
}

func ordenar(estado botsdk.EstadoPublico) []int {
	sillas := make([]int, 0, len(estado.Jugadores))
	for _, j := range estado.Jugadores {
		if j.EnMano && j.EnTorneo {
			sillas = append(sillas, j.PosicionSilla)
		}
	}
	sort.Ints(sillas)
	if len(sillas) == 0 {
		return nil
	}
	idx := 0
	for i, s := range sillas {
		if s >= estado.PosicionBoton {
			idx = i
			break
		}
	}
	orden := make([]int, 0, len(sillas))
	orden = append(orden, sillas[idx:]...)
	orden = append(orden, sillas[:idx]...)
	return orden
}

func nombres(n int) []string {
	switch n {
	case 2:
		return []string{"BTN", "BB"}
	case 3:
		return []string{"BTN", "SB", "BB"}
	case 4:
		return []string{"BTN", "SB", "BB", "CO"}
	case 5:
		return []string{"BTN", "SB", "BB", "UTG", "CO"}
	case 6:
		return []string{"BTN", "SB", "BB", "UTG", "MP", "CO"}
	case 7:
		return []string{"BTN", "SB", "BB", "UTG", "MP", "HJ", "CO"}
	default:
		if n <= 1 {
			return []string{"BTN"}
		}
		return []string{"BTN", "SB", "BB", "UTG", "UTG", "MP", "HJ", "CO"}[:min(n, 8)]
	}
}
