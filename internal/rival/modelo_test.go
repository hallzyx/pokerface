package rival

import (
	"testing"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

func TestEstadisticasYRepartoNoMuevenNada(t *testing.T) {
	m := Nuevo()
	for n := 0; n < 4; n++ {
		id := "h"
		estado := botsdk.EstadoPublico{
			IDMano: id + string(rune('0'+n)), Etapa: botsdk.Flop,
			CiegaGrande: 100,
			Jugadores: []botsdk.JugadorPublico{
				{ID: "agresivo", EnMano: true, EnTorneo: true, Activo: true},
				{ID: "pasivo", EnMano: true, EnTorneo: true, Activo: true},
			},
			HistorialAcciones: []botsdk.AccionRegistrada{
				{IDJugador: "agresivo", Etapa: botsdk.PreFlop, Accion: botsdk.Accion{Tipo: botsdk.Raise, Monto: 250}},
				{IDJugador: "pasivo", Etapa: botsdk.PreFlop, Accion: botsdk.Accion{Tipo: botsdk.Call}},
				{IDJugador: "agresivo", Etapa: botsdk.Flop, Accion: botsdk.Accion{Tipo: botsdk.Bet, Monto: 600}},
				{IDJugador: "pasivo", Etapa: botsdk.Flop, Accion: botsdk.Accion{Tipo: botsdk.Fold}},
			},
		}
		m.ObservarEstado(estado, "yo")
		m.ObservarEstado(estado, "yo") // repetido: no cuenta dos veces
	}
	lec := m.Lectura(botsdk.EstadoPublico{}, []string{"pasivo"})
	if lec.Manos != 4 {
		t.Fatalf("manos: %d", lec.Manos)
	}
	if lec.FoldGrande < 0.99 {
		t.Fatalf("tendría que haberse ido ante la apuesta grande: %.2f", lec.FoldGrande)
	}
	if !lec.SobreFold {
		t.Fatal("cuatro folds a apuesta grande tienen que marcar sobre-fold")
	}
	firma := m.Firma()
	m.ObservarFin(botsdk.ResultadoMano{
		IDMano:   "x",
		Repartos: []botsdk.Reparto{{IDJugador: "yo", Monto: 9000}},
	}, "yo")
	if m.Firma() != firma {
		t.Fatalf("un reparto de fichas cambió el modelo: %s -> %s", firma, m.Firma())
	}
}
