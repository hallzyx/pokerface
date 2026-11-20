// Package lectura es la foto inmutable de lo que creemos de los rivales.
//
// Es un valor, sin punteros al modelo vivo. El plan, el teatro y la voz de
// paranoia solo reciben esta foto: ninguno puede escribir de vuelta en las
// creencias. Quien actualiza el modelo es el paquete rival, y solo el
// orquestador lo tiene.
package lectura

// Lectura resume a los rivales que todavía están en la mano.
type Lectura struct {
	Manos             int
	VPIP              float64
	PFR               float64
	FactorAgresion    float64
	FoldChico         float64
	FoldGrande        float64
	FoldAlOpen        float64
	FoldAlTresBet     float64
	Estacion          bool // paga con manos hechas y casi no se va
	Carpeta           bool // se va ante una apuesta: base, conservador
	SobreFold         bool // se va ante apuestas grandes
	PagaGrande        bool // estación que también paga lo grande
	SoloPagaChico     bool // paga lo chico y suelta lo grande
	NoSuelta          bool // no se retira ante un 3-bet
	SueltaAlEmpuje    bool // medimos que se va ante un all-in
	Rango             []string
	Rangos            [][]string // uno por rival, en el orden pedido
	MuestraSuficiente bool
}

// ConRango indica si hay un rango estimado para muestrear.
func (l Lectura) ConRango() bool { return len(l.Rango) > 0 }
