package bot

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/equidad"
)

func carta(r botsdk.Rango, p botsdk.Palo) botsdk.Carta {
	return botsdk.Carta{Rango: r, Palo: p}
}

func turnoInicial(mano botsdk.Mano, silla int, stack int64) botsdk.Turno {
	jugadores := make([]botsdk.JugadorPublico, 6)
	for i := 0; i < 6; i++ {
		ap := int64(0)
		if i == 1 {
			ap = 50
		}
		if i == 2 {
			ap = 100
		}
		id := "r" + string(rune('a'+i))
		if i == silla {
			id = "artesanal"
		}
		saldo := stack
		if i == 1 {
			saldo = stack - 50
		}
		if i == 2 {
			saldo = stack - 100
		}
		jugadores[i] = botsdk.JugadorPublico{
			ID: id, Nombre: id, Saldo: saldo, ApuestaRonda: ap,
			Activo: true, EnTorneo: true, EnMano: true, PosicionSilla: i,
		}
	}
	validas := []botsdk.TipoAccion{botsdk.Fold, botsdk.Call, botsdk.Raise, botsdk.AllIn}
	if silla == 2 {
		validas = []botsdk.TipoAccion{botsdk.Check, botsdk.Raise, botsdk.AllIn}
	}
	return botsdk.Turno{
		IDJugador: "artesanal", Silla: silla, Yo: jugadores[silla], Mano: mano,
		Validas: validas, Timeout: 5 * time.Second,
		Estado: botsdk.EstadoPublico{
			IDMano: "mano-temprana", Etapa: botsdk.PreFlop, Pozo: 150,
			ApuestaActual: 100, SubidaMinima: 100,
			CiegaChica: 50, CiegaGrande: 100, PosicionBoton: 0,
			Jugadores: jugadores,
		},
	}
}

func TestNombre(t *testing.T) {
	if Nuevo().Nombre() != "Inteligencia artesanal" {
		t.Fatal(Nuevo().Nombre())
	}
}

func TestManosInicialesFuertesYDebiles(t *testing.T) {
	b := Nuevo()
	aa := botsdk.Mano{carta(botsdk.As, botsdk.Picas), carta(botsdk.As, botsdk.Corazones)}
	basura := botsdk.Mano{carta(botsdk.Siete, botsdk.Treboles), carta(botsdk.Dos, botsdk.Diamantes)}

	fuerte := b.Decidir(context.Background(), turnoInicial(aa, 3, 10000))
	if fuerte.Tipo != botsdk.Raise || fuerte.Monto != 250 {
		t.Fatalf("AA temprano desde UTG: %+v", fuerte)
	}
	debil := b.Decidir(context.Background(), turnoInicial(basura, 3, 10000))
	if debil.Tipo != botsdk.Fold {
		t.Fatalf("72o temprano desde UTG: %+v", debil)
	}
	paso := b.Decidir(context.Background(), turnoInicial(basura, 2, 10000))
	if paso.Tipo != botsdk.Check {
		t.Fatalf("72o en la ciega grande: %+v", paso)
	}
}

func TestNoSeInclinaDespuesDeUnPozoGrande(t *testing.T) {
	b := Nuevo()
	antes := b.Parametros()
	var firma string
	aa := botsdk.Mano{carta(botsdk.As, botsdk.Picas), carta(botsdk.As, botsdk.Corazones)}
	basura := botsdk.Mano{carta(botsdk.Siete, botsdk.Treboles), carta(botsdk.Dos, botsdk.Diamantes)}
	turno := turnoInicial(aa, 3, 10000)

	primera := b.Decidir(context.Background(), turno)
	firma = b.FirmaModelo()
	b.ManoFin(botsdk.ResultadoMano{
		IDMano:   "mano-ganada",
		Repartos: []botsdk.Reparto{{IDJugador: "artesanal", Monto: 25000}},
	})
	if b.FirmaModelo() != firma {
		t.Fatal("ganar un pozo grande reescribió el modelo")
	}
	segunda := b.Decidir(context.Background(), turno)
	b.ManoFin(botsdk.ResultadoMano{
		IDMano:   "mano-perdida",
		Repartos: []botsdk.Reparto{{IDJugador: "ra", Monto: 25000}},
	})
	if b.FirmaModelo() != firma {
		t.Fatal("perder un pozo grande reescribió el modelo")
	}
	tercera := b.Decidir(context.Background(), turno)

	if !reflect.DeepEqual(antes, b.Parametros()) {
		t.Fatalf("los parámetros se movieron: antes %+v ahora %+v", antes, b.Parametros())
	}
	if primera != segunda || segunda != tercera {
		t.Fatalf("la misma mano cambió de acción: %+v %+v %+v", primera, segunda, tercera)
	}
	if b.Decidir(context.Background(), turnoInicial(basura, 3, 10000)).Tipo != botsdk.Fold {
		t.Fatal("después de perder un pozo grande, 72o dejó de retirarse")
	}
}

func TestDecisionNormalEsRapida(t *testing.T) {
	b := Nuevo()
	turno := turnoInicial(botsdk.Mano{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.As, botsdk.Corazones),
	}, 0, 10000)
	turno.Estado.Etapa = botsdk.Flop
	turno.Estado.Comunitarias = []botsdk.Carta{
		carta(botsdk.As, botsdk.Treboles), carta(botsdk.Nueve, botsdk.Diamantes), carta(botsdk.Dos, botsdk.Corazones),
	}
	turno.Estado.Pozo = 600
	turno.Estado.ApuestaActual = 0
	turno.Yo.ApuestaRonda = 0
	turno.Validas = []botsdk.TipoAccion{botsdk.Check, botsdk.Bet, botsdk.AllIn}
	inicio := time.Now()
	accion := b.Decidir(context.Background(), turno)
	if time.Since(inicio) > 400*time.Millisecond {
		t.Fatalf("una decisión normal tardó %s", time.Since(inicio))
	}
	if accion.Tipo != botsdk.Bet && accion.Tipo != botsdk.Raise && accion.Tipo != botsdk.AllIn {
		t.Fatalf("el trío de ases tendría que apostar, got %+v", accion)
	}
}

func TestPlazoConComponenteLento(t *testing.T) {
	lento := func(ctx context.Context, c equidad.Consulta) equidad.Resultado {
		time.Sleep(4500 * time.Millisecond)
		return equidad.Resultado{Valor: 0.99, Simulaciones: 10}
	}
	b := NuevoCon(Opciones{Estimar: lento})
	turno := turnoInicial(botsdk.Mano{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.As, botsdk.Corazones),
	}, 0, 10000)
	turno.Estado.Etapa = botsdk.Flop
	turno.Estado.Comunitarias = []botsdk.Carta{
		carta(botsdk.As, botsdk.Treboles), carta(botsdk.Siete, botsdk.Diamantes), carta(botsdk.Dos, botsdk.Corazones),
	}
	turno.Estado.Pozo = 600
	turno.Estado.ApuestaActual = 0
	turno.Yo.ApuestaRonda = 0
	turno.Validas = []botsdk.TipoAccion{botsdk.Check, botsdk.Bet, botsdk.AllIn}
	// Sin apuesta pendiente el camino rápido apostaría el trío. El componente
	// lento no vuelve, así que tiene que salir el fallback (check).
	inicio := time.Now()
	accion := b.Decidir(context.Background(), turno)
	paso := time.Since(inicio)
	if paso >= 4*time.Second {
		t.Fatalf("respondió tarde: %s", paso)
	}
	if paso < 2*time.Second {
		t.Fatalf("volvió demasiado pronto (%s): el vigilante no esperó al componente lento", paso)
	}
	if accion.Tipo != botsdk.Check {
		t.Fatalf("el fallback con check disponible tiene que ser check, got %+v", accion)
	}
}
