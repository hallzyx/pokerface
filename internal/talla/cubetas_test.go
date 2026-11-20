package talla

import (
	"testing"

	"inteligenciaartesanal/internal/lectura"
)

func TestCubetasYDefensaMinima(t *testing.T) {
	vacia := lectura.Lectura{}
	if got := Elegir("seca", false, vacia); got < 0.3 || got > 0.4 {
		t.Fatalf("seca: %v", got)
	}
	if got := Elegir("mojada", false, vacia); got < 0.6 {
		t.Fatalf("mojada: %v", got)
	}
	if got := Elegir("monotono", false, vacia); got < 0.6 {
		t.Fatalf("monótono: %v", got)
	}
	if d := DefensaMinima(1.0 / 3.0); d < 0.74 || d > 0.76 {
		t.Fatalf("defensa mínima de 1/3: %v", d)
	}
	if f := FoldNecesario(2.0 / 3.0); f < 0.39 || f > 0.41 {
		t.Fatalf("fold necesario de 2/3: %v", f)
	}
}
