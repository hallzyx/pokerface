// Package plan es la fuente de verdad de la mano: fuerza, equidad y línea
// que queremos. Devuelve una Vista por valor. El teatro puede presentar otra
// acción, pero no hay un camino de vuelta para reescribir esta vista ni el
// modelo de rivales: este paquete no importa ni a teatro ni a rival.
package plan

import (
	"hash/fnv"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/lectura"
	"inteligenciaartesanal/internal/preflop"
	"inteligenciaartesanal/internal/talla"
)

// Linea es la jugada que el plan quiere, antes de disfrazarla.
type Linea string

const (
	LineaRetirarse Linea = "retirarse"
	LineaPasar     Linea = "pasar"
	LineaPagar     Linea = "pagar"
	LineaValor     Linea = "valor"
	LineaValorFino Linea = "valor_fino"
	LineaSemifarol Linea = "semifarol"
	LineaFarol     Linea = "farol"
	LineaAllIn     Linea = "allin"
)

// Parametros es la política fija. Se copia al crear el bot y no se toca
// cuando se gana o se pierde un pozo.
type Parametros struct {
	VersionTablas      string
	PresupuestoEquidad time.Duration
	TopeDecision       time.Duration
	MargenPlazo        time.Duration
	StackPushBB        float64
	FarolSinMuestra    float64
	FarolConMuestra    float64
	FarolSobreFold     float64
	FarolSonda         float64
}

// ParametrosPorDefecto lee las cubetas y la versión de las tablas una vez.
func ParametrosPorDefecto() Parametros {
	c := talla.PorDefecto()
	return Parametros{
		VersionTablas:      preflop.PorDefecto().Tamanos.Version,
		PresupuestoEquidad: 160 * time.Millisecond,
		TopeDecision:       3200 * time.Millisecond,
		MargenPlazo:        1200 * time.Millisecond,
		StackPushBB:        preflop.PorDefecto().Tamanos.StackPushBB,
		FarolSinMuestra:    c.FarolSinMuestra,
		FarolConMuestra:    c.FarolConMuestra,
		FarolSobreFold:     c.FarolSobreFold,
		FarolSonda:         c.FarolSonda,
	}
}

// Entrada es todo lo que el plan necesita. Lectura es una copia.
type Entrada struct {
	Turno        botsdk.Turno
	Lectura      lectura.Lectura
	Equidad      float64
	TieneEquidad bool
	Params       Parametros
}

// Vista es la decisión del plan. Los campos son privados: otro paquete solo
// la lee y, si hace falta, pide una copia con otra línea.
type Vista struct {
	linea    Linea
	fraccion float64
	monto    int64
	equidad  float64
	fuerza   string
	textura  string
	cerca    bool
	motivo   string
}

func (v Vista) Linea() Linea      { return v.linea }
func (v Vista) Fraccion() float64 { return v.fraccion }
func (v Vista) Monto() int64      { return v.monto }
func (v Vista) Equidad() float64  { return v.equidad }
func (v Vista) Fuerza() string    { return v.fuerza }
func (v Vista) Textura() string   { return v.textura }
func (v Vista) Cerca() bool       { return v.cerca }
func (v Vista) Motivo() string    { return v.motivo }

// ConLinea devuelve una copia con otra línea. La vista original no cambia y
// no existe un setter sobre el modelo.
func (v Vista) ConLinea(l Linea) Vista {
	v.linea = l
	if v.motivo != "" {
		v.motivo += " | ajustada"
	}
	return v
}

// Construir elige la línea. Preflop sale de las tablas; postflop, de la
// clasificación y de la equidad que ya calculó el orquestador.
func Construir(e Entrada) Vista {
	if e.Params.VersionTablas == "" {
		e.Params = ParametrosPorDefecto()
	}
	if !preflop.ManoValida(e.Turno.Mano) {
		return vista(LineaRetirarse, 0, 0, 0, "", "", false, "sin cartas")
	}
	if len(e.Turno.Estado.Comunitarias) < 3 {
		return preflopLinea(e)
	}
	return postflopLinea(e)
}

func vista(l Linea, frac float64, monto int64, eq float64, fuerza, textura string, cerca bool, motivo string) Vista {
	return Vista{
		linea: l, fraccion: frac, monto: monto, equidad: eq,
		fuerza: fuerza, textura: textura, cerca: cerca, motivo: motivo,
	}
}

func ciega(t botsdk.Turno) int64 {
	if t.Estado.CiegaGrande > 0 {
		return t.Estado.CiegaGrande
	}
	return 100
}

func stackNuestroBB(t botsdk.Turno) float64 {
	return float64(t.Yo.Saldo+t.Yo.ApuestaRonda) / float64(ciega(t))
}

func subir(total float64, motivo string) Vista {
	if total < 0 {
		total = 0
	}
	return vista(LineaValor, 0, int64(total+0.5), 0, "preflop", "", false, motivo)
}

func mezclar(t botsdk.Turno, p float64) bool {
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(t.Estado.IDMano))
	_, _ = h.Write([]byte(t.Estado.Etapa))
	_, _ = h.Write([]byte{byte(t.Mano[0].Rango), byte(t.Mano[0].Palo), byte(t.Mano[1].Rango), byte(t.Mano[1].Palo)})
	return float64(h.Sum32()%10000)/10000 < p
}
