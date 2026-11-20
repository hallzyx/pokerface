// Package paranoia es una segunda opinión. Corre solo en pozos grandes o
// decisiones cerradas, con un plazo propio, y únicamente puede endurecer la
// línea (retirarse o pasar). Si no llega a tiempo, no hay ajuste.
package paranoia

import (
	"context"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/equidad"
	"inteligenciaartesanal/internal/plan"
)

// Ajuste es lo único que la voz devuelve. No trae un modelo ni una vista
// mutable: el orquestador aplica la línea sobre una copia.
type Ajuste struct {
	Aplicar bool
	Linea   plan.Linea
	Motivo  string
}

// Consulta es una foto de la decisión. Todo viaja por valor.
type Consulta struct {
	Mano         botsdk.Mano
	Comunitarias []botsdk.Carta
	Rivales      int
	Rango        []string
	Equidad      float64
	Precio       float64
	Linea        plan.Linea
	PorIgualar   int64
	Semilla      uint64
}

// Debe dice si vale la pena la segunda opinión. Preflop no: ahí mandan las tablas.
func Debe(v plan.Vista, t botsdk.Turno) bool {
	if len(t.Estado.Comunitarias) < 3 {
		return false
	}
	if v.Linea() == plan.LineaPasar && t.PorIgualar() == 0 && !v.Cerca() {
		return false
	}
	if v.Linea() == plan.LineaRetirarse && !v.Cerca() {
		return false
	}
	bb := t.Estado.CiegaGrande
	if bb <= 0 {
		bb = 100
	}
	pozoBB := float64(t.Estado.Pozo+t.PorIgualar()) / float64(bb)
	if pozoBB >= 25 {
		return true
	}
	return v.Cerca()
}

// Presupuesto acota la voz. Los pozos enormes pueden usar casi todo el
// colchón de 3 s; una decisión cerrada y chica no.
func Presupuesto(t botsdk.Turno) time.Duration {
	bb := t.Estado.CiegaGrande
	if bb <= 0 {
		bb = 100
	}
	pozoBB := float64(t.Estado.Pozo+t.PorIgualar()) / float64(bb)
	switch {
	case pozoBB >= 60:
		return 2800 * time.Millisecond
	case pozoBB >= 25:
		return 700 * time.Millisecond
	default:
		return 400 * time.Millisecond
	}
}

// Opinar reestima la equidad, con rango si lo hay. Solo avisa cuando la
// equidad nueva es claramente peor: baja un farol a pasar, o un pago a
// retirarse. Nunca agranda la apuesta.
func Opinar(ctx context.Context, q Consulta) Ajuste {
	if ctx.Err() != nil {
		return Ajuste{}
	}
	resto := 2500 * time.Millisecond
	if plazo, ok := ctx.Deadline(); ok {
		if d := time.Until(plazo); d < resto {
			resto = d
		}
	}
	if resto <= 0 {
		return Ajuste{}
	}
	res := equidad.Estimar(ctx, q.Semilla, equidad.Consulta{
		Mano:            q.Mano,
		Comunitarias:    q.Comunitarias,
		Rivales:         q.Rivales,
		MaxSimulaciones: 20000,
		Presupuesto:     resto,
		Rango:           q.Rango,
	})
	if res.Simulaciones < 80 {
		return Ajuste{}
	}
	if q.Equidad-res.Valor < 0.08 {
		return Ajuste{}
	}
	switch q.Linea {
	case plan.LineaFarol, plan.LineaSemifarol:
		return Ajuste{Aplicar: true, Linea: plan.LineaPasar, Motivo: "la segunda lectura baja el farol"}
	case plan.LineaPagar, plan.LineaValor, plan.LineaValorFino:
		if q.PorIgualar > 0 && res.Valor+0.02 < q.Precio {
			return Ajuste{Aplicar: true, Linea: plan.LineaRetirarse, Motivo: "la equidad no paga el precio"}
		}
		if q.PorIgualar == 0 && (q.Linea == plan.LineaValor || q.Linea == plan.LineaValorFino) {
			return Ajuste{Aplicar: true, Linea: plan.LineaPasar, Motivo: "conviene no apostar"}
		}
	}
	return Ajuste{}
}
