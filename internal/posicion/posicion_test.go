package posicion

import (
	"testing"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

func TestPosiciones6Max(t *testing.T) {
	estado := botsdk.EstadoPublico{PosicionBoton: 0}
	for s := 0; s < 6; s++ {
		estado.Jugadores = append(estado.Jugadores, botsdk.JugadorPublico{
			ID: "j", PosicionSilla: s, EnMano: true, EnTorneo: true, Activo: true,
		})
	}
	quiere := []string{"BTN", "SB", "BB", "UTG", "MP", "CO"}
	for s, nombre := range quiere {
		if got := De(estado, s); got != nombre {
			t.Fatalf("silla %d: got %s want %s", s, got, nombre)
		}
	}

	estado.PosicionBoton = 4
	// 4 BTN, 5 SB, 0 BB, 1 UTG, 2 MP, 3 CO
	if De(estado, 0) != "BB" || De(estado, 1) != "UTG" || De(estado, 4) != "BTN" {
		t.Fatalf("rotación: BB=%s UTG=%s BTN=%s", De(estado, 0), De(estado, 1), De(estado, 4))
	}
}
