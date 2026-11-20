package rango

import (
	"testing"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

func TestSemillaNoEsAleatoria(t *testing.T) {
	subio := Semilla(0.2, 0.14, 0, true, false)
	if !contiene(subio, "AA") || contiene(subio, "72o") {
		t.Fatalf("el prior de un raise no puede incluir 72o: %v", subio[:min(8, len(subio))])
	}
	ancho := Semilla(0.55, 0.10, 20, false, true)
	if len(ancho) <= len(subio) {
		t.Fatalf("un VPIP alto tiene que ser más ancho (%d) que un raise corto (%d)", len(ancho), len(subio))
	}
}

func TestApuestaGrandeDejaSoloValor(t *testing.T) {
	board := []botsdk.Carta{
		{Rango: botsdk.As, Palo: botsdk.Picas},
		{Rango: botsdk.Siete, Palo: botsdk.Treboles},
		{Rango: botsdk.Dos, Palo: botsdk.Diamantes},
	}
	semilla := Semilla(0.4, 0.18, 10, true, false)
	valor := Achicar(semilla, board, Valor)
	if contiene(valor, "72o") || contiene(valor, "KQo") {
		t.Fatalf("una apuesta de valor no puede ser KQ o 72: %v", valor)
	}
	if !contiene(valor, "AA") && !contiene(valor, "77") && !contiene(valor, "A7s") {
		t.Fatalf("el valor en A72 tiene que poder ser AA, 77 o A7: %v", valor)
	}
}

func contiene(lista []string, clave string) bool {
	for _, c := range lista {
		if c == clave {
			return true
		}
	}
	return false
}
