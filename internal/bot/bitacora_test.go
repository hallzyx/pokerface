package bot

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

func TestManoFinEscribePozoGanadorNetoYCartas(t *testing.T) {
	var buf bytes.Buffer
	b := Nuevo().ConBitacora(log.New(&buf, "", 0))
	turno := turnoInicial(botsdk.Mano{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Corazones),
	}, 3, 10000)
	b.ManoInicio(turno.Estado, turno.Mano)
	_ = b.Decidir(context.Background(), turno)

	final := turno.Estado
	final.Jugadores[3].Saldo = 9800
	final.Jugadores[3].ApuestaRonda = 200
	b.EstadoActualizado(final)

	b.ManoFin(botsdk.ResultadoMano{
		IDMano: turno.Estado.IDMano,
		Repartos: []botsdk.Reparto{
			{IDJugador: "artesanal", Monto: 400},
			{IDJugador: "rival", Monto: 0},
		},
		Mostradas: map[string]botsdk.Mano{
			"rival":     {carta(botsdk.Reina, botsdk.Picas), carta(botsdk.Reina, botsdk.Corazones)},
			"artesanal": {carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Corazones)},
		},
	})

	linea := buf.String()
	for _, quiero := range []string{
		"mano=mano-temprana",
		"n=1",
		"pozo=400",
		"ganadores=artesanal:400",
		"mio=400",
		"neto=200",
		"fichas=10200",
		"showdown=1",
		"en_showdown=1",
		"nuestra=1",
		"cartas=artesanal:AsKh,rival:QsQh",
	} {
		if !strings.Contains(linea, quiero) {
			t.Fatalf("falta %q en\n%s", quiero, linea)
		}
	}
	if strings.Contains(linea, " ") && strings.Contains(strings.TrimSpace(linea), " =") {
		t.Fatalf("la línea no es clave=valor: %s", linea)
	}
}

func TestManoSinShowdownYResumen(t *testing.T) {
	var buf bytes.Buffer
	b := Nuevo().ConBitacora(log.New(&buf, "", 0))

	ganarSinMostrar(b, "m1", 150)
	perderEnShowdown(b, "m2")
	ganarEnShowdown(b, "m3")

	texto := buf.String()
	if strings.Count(texto, " showdown=0") != 1 || strings.Count(texto, " showdown=1") != 2 {
		t.Fatalf("showdowns:\n%s", texto)
	}
	if !strings.Contains(texto, "cartas=-") {
		t.Fatalf("la mano sin showdown debería ir sin cartas:\n%s", texto)
	}
	resumen := b.LineaResumen()
	for _, quiero := range []string{
		"resumen manos=3",
		"neto=50",
		"showdown_ganados=1",
		"showdown_perdidos=1",
		"sin_showdown=1",
	} {
		if !strings.Contains(resumen, quiero) {
			t.Fatalf("falta %q en %s", quiero, resumen)
		}
	}
}

func TestAllInRegistraElMontoReal(t *testing.T) {
	var buf bytes.Buffer
	b := Nuevo()
	b.detalle = log.New(&buf, "", 0)
	turno := turnoInicial(botsdk.Mano{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.As, botsdk.Corazones),
	}, 3, 1000)
	accion := b.Decidir(context.Background(), turno)
	if accion.Tipo != botsdk.AllIn {
		t.Fatalf("AA con 10 ciegas debería ir all-in, got %+v", accion)
	}
	if accion.Monto != 0 {
		t.Fatalf("la acción que viaja sigue sin monto, got %d", accion.Monto)
	}
	linea := buf.String()
	if !strings.Contains(linea, "allin 1000") {
		t.Fatalf("el log no trae el monto real:\n%s", linea)
	}
	if strings.Contains(linea, "allin 0") {
		t.Fatalf("el all-in quedó en 0:\n%s", linea)
	}
}

func ganarSinMostrar(b *Bot, id string, pozo int64) {
	turno := turnoInicial(botsdk.Mano{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Corazones),
	}, 3, 10000)
	turno.Estado.IDMano = id
	b.ManoInicio(turno.Estado, turno.Mano)
	_ = b.Decidir(context.Background(), turno)
	b.ManoFin(botsdk.ResultadoMano{
		IDMano:   id,
		Repartos: []botsdk.Reparto{{IDJugador: "artesanal", Monto: pozo}},
	})
}

func perderEnShowdown(b *Bot, id string) {
	turno := turnoInicial(botsdk.Mano{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Corazones),
	}, 3, 10000)
	turno.Estado.IDMano = id
	b.ManoInicio(turno.Estado, turno.Mano)
	_ = b.Decidir(context.Background(), turno)
	final := turno.Estado
	final.Jugadores[3].Saldo = 9800
	b.EstadoActualizado(final)
	b.ManoFin(botsdk.ResultadoMano{
		IDMano: id,
		Repartos: []botsdk.Reparto{
			{IDJugador: "rival", Monto: 200},
		},
		Mostradas: map[string]botsdk.Mano{
			"artesanal": {carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Corazones)},
			"rival":     {carta(botsdk.Reina, botsdk.Picas), carta(botsdk.Reina, botsdk.Corazones)},
		},
	})
}

func ganarEnShowdown(b *Bot, id string) {
	turno := turnoInicial(botsdk.Mano{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Corazones),
	}, 3, 10000)
	turno.Estado.IDMano = id
	b.ManoInicio(turno.Estado, turno.Mano)
	_ = b.Decidir(context.Background(), turno)
	final := turno.Estado
	final.Jugadores[3].Saldo = 9800
	b.EstadoActualizado(final)
	b.ManoFin(botsdk.ResultadoMano{
		IDMano: id,
		Repartos: []botsdk.Reparto{
			{IDJugador: "artesanal", Monto: 300},
		},
		Mostradas: map[string]botsdk.Mano{
			"artesanal": {carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Corazones)},
		},
	})
}
