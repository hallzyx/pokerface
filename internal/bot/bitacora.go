package bot

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

// cursoMano guarda el stack al abrir la mano y el saldo vivo, para poder
// calcular el neto cuando llega el reparto.
type cursoMano struct {
	id          string
	inicio      int64
	tieneInicio bool
	saldo       int64
	tieneSaldo  bool
	enMano      bool
	actuo       bool
	stacks      map[string]int64
	en          map[string]bool
}

type resumenSesion struct {
	manos       int
	neto        int64
	sdGanados   int
	sdPerdidos  int
	sinShowdown int
	registradas int
}

// LineaResumen es el cierre de la sesión, en pares clave=valor.
func (b *Bot) LineaResumen() string {
	b.fichasMu.Lock()
	defer b.fichasMu.Unlock()
	return fmt.Sprintf("resumen manos=%d neto=%d showdown_ganados=%d showdown_perdidos=%d sin_showdown=%d",
		b.stats.manos, b.stats.neto, b.stats.sdGanados, b.stats.sdPerdidos, b.stats.sinShowdown)
}

func (b *Bot) fijarID(id string) {
	if id == "" {
		return
	}
	b.fichasMu.Lock()
	b.idVisto = id
	b.fichasMu.Unlock()
}

func (b *Bot) verMesa(estado botsdk.EstadoPublico, actuo bool) {
	b.fichasMu.Lock()
	defer b.fichasMu.Unlock()
	if estado.IDMano != b.curso.id {
		b.abrirCurso(estado)
	}
	if actuo {
		b.curso.actuo = true
		b.curso.enMano = true
	}
	id := b.idVisto
	if id == "" {
		return
	}
	if !b.curso.tieneInicio {
		if s, ok := b.curso.stacks[id]; ok {
			b.curso.inicio = s
			b.curso.tieneInicio = true
			b.curso.enMano = b.curso.enMano || b.curso.en[id]
		}
	}
	j, ok := jugador(estado, id)
	if !ok {
		return
	}
	b.curso.saldo = j.Saldo
	b.curso.tieneSaldo = true
	b.fichas = j.Saldo + j.ApuestaRonda
	b.hayFichas = true
}

func (b *Bot) abrirCurso(estado botsdk.EstadoPublico) {
	c := cursoMano{
		id:     estado.IDMano,
		stacks: make(map[string]int64, len(estado.Jugadores)),
		en:     make(map[string]bool, len(estado.Jugadores)),
	}
	for _, j := range estado.Jugadores {
		c.stacks[j.ID] = j.Saldo + j.ApuestaRonda
		c.en[j.ID] = j.EnMano
	}
	if id := b.idVisto; id != "" {
		if s, ok := c.stacks[id]; ok {
			c.inicio = s
			c.tieneInicio = true
			c.enMano = c.en[id]
		}
		if j, ok := jugador(estado, id); ok {
			c.saldo = j.Saldo
			c.tieneSaldo = true
			b.fichas = j.Saldo + j.ApuestaRonda
			b.hayFichas = true
		}
	}
	b.curso = c
}

func (b *Bot) cerrarMano(res botsdk.ResultadoMano) {
	linea := b.cerrarManoLinea(res)
	b.escribir(linea)
}

func (b *Bot) cerrarManoLinea(res botsdk.ResultadoMano) string {
	b.fichasMu.Lock()
	defer b.fichasMu.Unlock()

	porID, pozo := sumarRepartos(res.Repartos)
	id := b.idVisto
	mio := porID[id]
	enShow := id != "" && tieneCartas(res.Mostradas, id)
	showdown := len(res.Mostradas) > 0
	nuestra := id != "" && (b.curso.enMano || b.curso.actuo || mio > 0 || enShow)
	mismo := b.curso.id == "" || res.IDMano == "" || b.curso.id == res.IDMano

	fichas := b.fichas
	neto := int64(0)
	if nuestra && mismo && b.curso.tieneSaldo {
		fichas = b.curso.saldo + mio
		if b.curso.tieneInicio {
			neto = fichas - b.curso.inicio
		}
		b.fichas = fichas
		b.hayFichas = true
	}

	b.stats.registradas++
	n := b.stats.registradas
	if nuestra {
		b.stats.manos++
		b.stats.neto += neto
		switch {
		case showdown && mio > 0:
			b.stats.sdGanados++
		case showdown && enShow && mio == 0:
			b.stats.sdPerdidos++
		case !showdown && mio > 0:
			b.stats.sinShowdown++
		}
	}
	b.curso = cursoMano{}

	mano := res.IDMano
	if mano == "" {
		mano = "-"
	}
	return fmt.Sprintf("mano=%s n=%d pozo=%d ganadores=%s mio=%d neto=%d fichas=%d showdown=%d en_showdown=%d nuestra=%d cartas=%s",
		mano, n, pozo, ganadoresDe(porID), mio, neto, fichas, uno(showdown), uno(enShow), uno(nuestra), cartasDe(res.Mostradas))
}

func (b *Bot) escribir(linea string) {
	l := b.bitacora
	if l == nil {
		l = log.Default()
	}
	l.Printf("%s", linea)
}

func sumarRepartos(repartos []botsdk.Reparto) (map[string]int64, int64) {
	porID := make(map[string]int64, len(repartos))
	var pozo int64
	for _, r := range repartos {
		porID[r.IDJugador] += r.Monto
		pozo += r.Monto
	}
	return porID, pozo
}

func ganadoresDe(porID map[string]int64) string {
	ids := make([]string, 0, len(porID))
	for id, monto := range porID {
		if monto > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return "-"
	}
	sort.Strings(ids)
	partes := make([]string, len(ids))
	for i, id := range ids {
		partes[i] = fmt.Sprintf("%s:%d", id, porID[id])
	}
	return strings.Join(partes, ",")
}

func cartasDe(mostradas map[string]botsdk.Mano) string {
	if len(mostradas) == 0 {
		return "-"
	}
	ids := make([]string, 0, len(mostradas))
	for id := range mostradas {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	partes := make([]string, len(ids))
	for i, id := range ids {
		mano := mostradas[id]
		partes[i] = id + ":" + mano[0].String() + mano[1].String()
	}
	return strings.Join(partes, ",")
}

func tieneCartas(mostradas map[string]botsdk.Mano, id string) bool {
	_, ok := mostradas[id]
	return ok
}

func jugador(estado botsdk.EstadoPublico, id string) (botsdk.JugadorPublico, bool) {
	for _, j := range estado.Jugadores {
		if j.ID == id {
			return j, true
		}
	}
	return botsdk.JugadorPublico{}, false
}

func uno(v bool) int {
	if v {
		return 1
	}
	return 0
}
