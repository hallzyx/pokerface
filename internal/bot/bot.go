// Package bot es el orquestador. Decidir arma el plan, deja que el teatro lo
// presente y, solo si el pozo o la equidad lo piden, escucha a la voz de
// paranoia. Esa voz ajusta una copia de la línea: no toca el modelo.
package bot

import (
	"context"
	"flag"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/equidad"
	"inteligenciaartesanal/internal/lectura"
	"inteligenciaartesanal/internal/paranoia"
	"inteligenciaartesanal/internal/plan"
	"inteligenciaartesanal/internal/posicion"
	"inteligenciaartesanal/internal/preflop"
	"inteligenciaartesanal/internal/rival"
	"inteligenciaartesanal/internal/teatro"
)

// Estimador calcula la equidad. Los tests pueden poner uno que no responde.
type Estimador func(context.Context, equidad.Consulta) equidad.Resultado

// Opinador es la voz de paranoia inyectable.
type Opinador func(context.Context, paranoia.Consulta) paranoia.Ajuste

// Opciones permite sustituir la equidad o la voz en las pruebas.
type Opciones struct {
	Estimar Estimador
	Opinar  Opinador
}

// Bot juega una mesa. Los parámetros de estrategia se fijan en Nuevo y no
// se reescriben con el resultado de las manos.
type Bot struct {
	modelo    *rival.Modelo
	params    plan.Parametros
	estimarFn Estimador
	opinarFn  Opinador
	semilla   uint64
	n         atomic.Uint64
	detalle   *log.Logger
	bitacora  *log.Logger
	fichasMu  sync.Mutex
	idVisto   string
	fichas    int64
	hayFichas bool
	curso     cursoMano
	stats     resumenSesion
}

// Nuevo arma el bot con las tablas embebidas.
func Nuevo() *Bot { return NuevoCon(Opciones{}) }

// NuevoCon es Nuevo con piezas sustituibles.
func NuevoCon(o Opciones) *Bot {
	_ = preflop.PorDefecto()
	return &Bot{
		modelo:    rival.Nuevo(),
		params:    plan.ParametrosPorDefecto(),
		estimarFn: o.Estimar,
		opinarFn:  o.Opinar,
		semilla:   20261110,
		bitacora:  log.Default(),
	}
}

// ConBitacora dirige las líneas de mano y el resumen. El token no se escribe.
func (b *Bot) ConBitacora(l *log.Logger) *Bot {
	if l != nil {
		b.bitacora = l
	}
	return b
}

// Nombre es el del equipo.
func (*Bot) Nombre() string { return "Inteligencia artesanal" }

// Parametros devuelve la política fija. Ganar o perder un pozo no la cambia.
func (b *Bot) Parametros() plan.Parametros { return b.params }

// Iniciar lee flags ya parseados por el SDK. No mira el token.
func (b *Bot) Iniciar() error {
	if f := flag.Lookup("detalle"); f != nil && f.Value.String() == "true" {
		b.detalle = log.New(os.Stderr, "artesanal: ", log.LstdFlags)
	}
	if f := flag.Lookup("semilla"); f != nil {
		if n, err := strconv.ParseUint(f.Value.String(), 10, 64); err == nil && n != 0 {
			b.semilla = n
		}
	}
	return nil
}

// ManoInicio observa la mesa, también si esta mano no es nuestra.
func (b *Bot) ManoInicio(estado botsdk.EstadoPublico, _ botsdk.Mano) {
	b.verMesa(estado, false)
	b.modelo.ObservarEstado(estado, "")
}

// ManoFin anota las cartas mostradas. Los montos del reparto no entran al modelo.
func (b *Bot) ManoFin(res botsdk.ResultadoMano) {
	b.modelo.ObservarFin(res, "")
	b.cerrarMano(res)
}

// EstadoActualizado sigue a los rivales en los turnos ajenos.
func (b *Bot) EstadoActualizado(estado botsdk.EstadoPublico) {
	b.verMesa(estado, false)
	b.modelo.ObservarEstado(estado, "")
}

// Decidir responde siempre dentro del vigilante, bastante antes de los 5 s.
func (b *Bot) Decidir(ctx context.Context, t botsdk.Turno) botsdk.Accion {
	plazo := plazoDecision(t.Timeout, b.params)
	ctxD, cancelar := context.WithTimeout(ctx, plazo)
	defer cancelar()

	ch := make(chan botsdk.Accion, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				ch <- seguro(t)
			}
		}()
		ch <- b.pensar(ctxD, t)
	}()

	select {
	case accion := <-ch:
		return sanear(t, accion)
	case <-ctxD.Done():
		return seguro(t)
	}
}

func (b *Bot) pensar(ctx context.Context, t botsdk.Turno) botsdk.Accion {
	if t.IDJugador != "" {
		b.modelo.FijarYo(t.IDJugador)
		b.fijarID(t.IDJugador)
	}
	b.verMesa(t.Estado, true)
	b.modelo.ObservarEstado(t.Estado, t.IDJugador)

	entrada := plan.Entrada{
		Turno:   t,
		Lectura: b.modelo.Lectura(t.Estado, posicion.Rivales(t)),
		Params:  b.params,
	}
	if len(t.Estado.Comunitarias) >= 3 && preflop.ManoValida(t.Mano) {
		res := b.estimar(ctx, equidad.Consulta{
			Mano:            t.Mano,
			Comunitarias:    t.Estado.Comunitarias,
			Rivales:         len(posicion.Rivales(t)),
			MaxSimulaciones: 2500,
			Presupuesto:     b.params.PresupuestoEquidad,
			Rango:           entrada.Lectura.Rango,
			Rangos:          entrada.Lectura.Rangos,
		})
		if res.Simulaciones > 0 {
			entrada.Equidad = res.Valor
			entrada.TieneEquidad = true
		}
	}

	vista := plan.Construir(entrada)
	vista = b.ajustar(ctx, vista, t, entrada.Lectura)
	accion := teatro.Presentar(vista, t)
	if b.detalle != nil {
		monto := accion.Monto
		if accion.Tipo == botsdk.AllIn && monto == 0 {
			monto = t.Techo()
		}
		b.detalle.Printf("%s %s%s línea %s eq %.2f -> %s %d (%s)",
			t.Estado.Etapa, t.Mano[0], t.Mano[1], vista.Linea(), vista.Equidad(), accion.Tipo, monto, vista.Motivo())
	}
	return accion
}

func (b *Bot) estimar(ctx context.Context, c equidad.Consulta) equidad.Resultado {
	if b.estimarFn != nil {
		return b.estimarFn(ctx, c)
	}
	return equidad.Estimar(ctx, b.siguienteSemilla(), c)
}

func (b *Bot) ajustar(ctx context.Context, v plan.Vista, t botsdk.Turno, lec lectura.Lectura) plan.Vista {
	if !paranoia.Debe(v, t) {
		return v
	}
	ctxP, cancelar := context.WithTimeout(ctx, paranoia.Presupuesto(t))
	defer cancelar()
	ch := make(chan paranoia.Ajuste, 1)
	q := paranoia.Consulta{
		Mano:         t.Mano,
		Comunitarias: append([]botsdk.Carta(nil), t.Estado.Comunitarias...),
		Rivales:      len(posicion.Rivales(t)),
		Rango:        append([]string(nil), lec.Rango...),
		Equidad:      v.Equidad(),
		Precio:       0,
		Linea:        v.Linea(),
		PorIgualar:   t.PorIgualar(),
		Semilla:      b.siguienteSemilla(),
	}
	if t.PorIgualar() > 0 {
		q.Precio = float64(t.PorIgualar()) / float64(t.Estado.Pozo+t.PorIgualar())
	}
	go func() {
		defer func() { recover() }()
		if b.opinarFn != nil {
			ch <- b.opinarFn(ctxP, q)
			return
		}
		ch <- paranoia.Opinar(ctxP, q)
	}()
	select {
	case aj := <-ch:
		if aj.Aplicar {
			return v.ConLinea(aj.Linea)
		}
	case <-ctxP.Done():
	}
	return v
}

func (b *Bot) siguienteSemilla() uint64 {
	return b.semilla + b.n.Add(1)
}

func plazoDecision(timeout time.Duration, p plan.Parametros) time.Duration {
	tope := p.TopeDecision
	if tope <= 0 {
		tope = 3200 * time.Millisecond
	}
	margen := p.MargenPlazo
	if margen <= 0 {
		margen = 1200 * time.Millisecond
	}
	if timeout <= 0 || timeout > tope+margen {
		return tope
	}
	resto := timeout - margen
	if resto < 200*time.Millisecond {
		return 200 * time.Millisecond
	}
	if resto > tope {
		return tope
	}
	return resto
}

func seguro(t botsdk.Turno) botsdk.Accion {
	if t.Puede(botsdk.Check) {
		return t.Check()
	}
	return t.Fold()
}

func sanear(t botsdk.Turno, a botsdk.Accion) botsdk.Accion {
	if a.Tipo == "" || !t.Puede(a.Tipo) {
		return seguro(t)
	}
	if a.Tipo == botsdk.Bet || a.Tipo == botsdk.Raise {
		return t.Apostar(a.Monto)
	}
	return a
}

// FirmaModelo expone los contadores para las pruebas de estabilidad.
func (b *Bot) FirmaModelo() string { return b.modelo.Firma() }

// Empuje implementa sesion.InformaEmpuje.
func (b *Bot) Empuje() (int, int) { return b.modelo.Empuje() }

// Fichas devuelve el último stack visto. En una mano abierta es el saldo más
// lo apostado en la ronda; después del reparto, el stack ya cobrado.
func (b *Bot) Fichas() (int64, bool) {
	b.fichasMu.Lock()
	defer b.fichasMu.Unlock()
	return b.fichas, b.hayFichas
}
