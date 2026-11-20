package plan

import (
	"testing"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/lectura"
)

func carta(r botsdk.Rango, p botsdk.Palo) botsdk.Carta {
	return botsdk.Carta{Rango: r, Palo: p}
}

func turnoPreflop(mano botsdk.Mano, silla int, stack int64) botsdk.Turno {
	jugadores := make([]botsdk.JugadorPublico, 6)
	for i := 0; i < 6; i++ {
		ap := int64(0)
		if i == 1 {
			ap = 50
		}
		if i == 2 {
			ap = 100
		}
		if i == silla {
			ap = 0
			if silla == 1 {
				ap = 50
			}
			if silla == 2 {
				ap = 100
			}
		}
		id := "r" + string(rune('a'+i))
		if i == silla {
			id = "artesanal"
		}
		saldo := stack - ap
		if i != silla {
			saldo = stack
			if i == 1 {
				saldo = stack - 50
			}
			if i == 2 {
				saldo = stack - 100
			}
		}
		jugadores[i] = botsdk.JugadorPublico{
			ID: id, Nombre: id, Saldo: saldo, ApuestaRonda: ap,
			Activo: true, EnTorneo: true, EnMano: true, PosicionSilla: i,
		}
		if i == silla && silla != 1 && silla != 2 {
			jugadores[i].ApuestaRonda = 0
			jugadores[i].Saldo = stack
		}
	}
	validas := []botsdk.TipoAccion{botsdk.Fold, botsdk.Call, botsdk.Raise, botsdk.AllIn}
	if silla == 2 {
		validas = []botsdk.TipoAccion{botsdk.Check, botsdk.Raise, botsdk.AllIn}
	}
	return botsdk.Turno{
		IDJugador: "artesanal",
		Silla:     silla,
		Yo:        jugadores[silla],
		Mano:      mano,
		Validas:   validas,
		Timeout:   5_000_000_000,
		Estado: botsdk.EstadoPublico{
			IDMano: "mano-temprana", Etapa: botsdk.PreFlop, Pozo: 150,
			ApuestaActual: 100, SubidaMinima: 100,
			CiegaChica: 50, CiegaGrande: 100, PosicionBoton: 0,
			Jugadores: jugadores,
		},
	}
}

func TestManosFuertesYDebilesAlInicio(t *testing.T) {
	aa := botsdk.Mano{carta(botsdk.As, botsdk.Picas), carta(botsdk.As, botsdk.Corazones)}
	basura := botsdk.Mano{carta(botsdk.Siete, botsdk.Treboles), carta(botsdk.Dos, botsdk.Diamantes)}

	fuerte := Construir(Entrada{Turno: turnoPreflop(aa, 3, 10000)})
	if fuerte.Linea() != LineaValor || fuerte.Monto() != 250 {
		t.Fatalf("AA UTG al inicio: %+v monto %d", fuerte.Linea(), fuerte.Monto())
	}
	debil := Construir(Entrada{Turno: turnoPreflop(basura, 3, 10000)})
	if debil.Linea() != LineaRetirarse {
		t.Fatalf("72o UTG: %s (%s)", debil.Linea(), debil.Motivo())
	}
	ciega := Construir(Entrada{Turno: turnoPreflop(basura, 2, 10000)})
	if ciega.Linea() != LineaPasar {
		t.Fatalf("72o en la ciega grande debería pasar, got %s", ciega.Linea())
	}
}

func TestPushFoldSinAnte(t *testing.T) {
	aa := botsdk.Mano{carta(botsdk.As, botsdk.Picas), carta(botsdk.As, botsdk.Corazones)}
	basura := botsdk.Mano{carta(botsdk.Siete, botsdk.Treboles), carta(botsdk.Dos, botsdk.Diamantes)}
	corto := Construir(Entrada{Turno: turnoPreflop(aa, 3, 1000)})
	if corto.Linea() != LineaAllIn {
		t.Fatalf("AA con 10bb: %s %s", corto.Linea(), corto.Motivo())
	}
	suelta := Construir(Entrada{Turno: turnoPreflop(basura, 3, 1000)})
	if suelta.Linea() != LineaRetirarse {
		t.Fatalf("72o con 10bb desde UTG: %s", suelta.Linea())
	}
}

func TestNoPagaUnaApuestaGrandeConUnaPareja(t *testing.T) {
	// As y rey en un as seco: top pair. Equidad alta contra el aire, pero la
	// apuesta es del tamaño del pozo: en el kit eso es trío o mejor.
	mano := botsdk.Mano{carta(botsdk.As, botsdk.Corazones), carta(botsdk.Rey, botsdk.Diamantes)}
	turno := turnoPreflop(mano, 0, 10000)
	turno.Estado.Etapa = botsdk.Flop
	turno.Estado.Comunitarias = []botsdk.Carta{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.Siete, botsdk.Treboles), carta(botsdk.Dos, botsdk.Diamantes),
	}
	turno.Estado.Pozo = 1800
	turno.Estado.ApuestaActual = 800
	turno.Yo.ApuestaRonda = 0
	turno.Yo.Saldo = 9000
	turno.Validas = []botsdk.TipoAccion{botsdk.Fold, botsdk.Call, botsdk.Raise, botsdk.AllIn}
	grande := Construir(Entrada{Turno: turno, Equidad: 0.62, TieneEquidad: true})
	if grande.Linea() != LineaRetirarse {
		t.Fatalf("top pair contra apuesta grande: %s (%s)", grande.Linea(), grande.Motivo())
	}

	turno.Estado.Pozo = 1200
	turno.Estado.ApuestaActual = 300
	chica := Construir(Entrada{Turno: turno, Equidad: 0.62, TieneEquidad: true})
	if chica.Linea() != LineaPagar {
		t.Fatalf("top pair contra apuesta chica: %s (%s)", chica.Linea(), chica.Motivo())
	}
}

func TestNoEntregaElStackConUnaPareja(t *testing.T) {
	mano := botsdk.Mano{carta(botsdk.As, botsdk.Corazones), carta(botsdk.Rey, botsdk.Diamantes)}
	turno := turnoPreflop(mano, 0, 8000)
	turno.Estado.Etapa = botsdk.Flop
	turno.Estado.Comunitarias = []botsdk.Carta{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.Nueve, botsdk.Treboles), carta(botsdk.Dos, botsdk.Diamantes),
	}
	turno.Estado.Pozo = 7000
	turno.Estado.ApuestaActual = 5000
	turno.Yo.ApuestaRonda = 0
	turno.Yo.Saldo = 8000
	turno.Validas = []botsdk.TipoAccion{botsdk.Fold, botsdk.Call, botsdk.AllIn}
	v := Construir(Entrada{Turno: turno, Equidad: 0.70, TieneEquidad: true})
	if v.Linea() != LineaRetirarse {
		t.Fatalf("pareja que compromete el stack: %s (%s)", v.Linea(), v.Motivo())
	}
}

func TestNoEntregaQuincePorCientoConUnaPareja(t *testing.T) {
	mano := botsdk.Mano{carta(botsdk.Rey, botsdk.Diamantes), carta(botsdk.Dos, botsdk.Diamantes)}
	turno := turnoPreflop(mano, 0, 4000)
	turno.Estado.Etapa = botsdk.Flop
	turno.Estado.Comunitarias = []botsdk.Carta{
		carta(botsdk.Rey, botsdk.Picas), carta(botsdk.Siete, botsdk.Treboles), carta(botsdk.Cuatro, botsdk.Corazones),
	}
	turno.Estado.Pozo = 5000
	turno.Estado.ApuestaActual = 800
	turno.Yo.ApuestaRonda = 0
	turno.Yo.Saldo = 4000
	turno.Validas = []botsdk.TipoAccion{botsdk.Fold, botsdk.Call, botsdk.Raise}
	v := Construir(Entrada{Turno: turno, Equidad: 0.62, TieneEquidad: true})
	if v.Linea() != LineaRetirarse {
		t.Fatalf("rey débil no paga el 20 %% del stack: %s (%s)", v.Linea(), v.Motivo())
	}

	ases := botsdk.Mano{carta(botsdk.As, botsdk.Corazones), carta(botsdk.As, botsdk.Picas)}
	turno.Mano = ases
	turno.Estado.Comunitarias = []botsdk.Carta{
		carta(botsdk.Siete, botsdk.Treboles), carta(botsdk.Dos, botsdk.Diamantes), carta(botsdk.Nueve, botsdk.Picas),
	}
	aa := Construir(Entrada{Turno: turno, Equidad: 0.75, TieneEquidad: true})
	if aa.Linea() == LineaRetirarse {
		t.Fatalf("AA overpair con equidad sí puede comprometer: %s (%s)", aa.Linea(), aa.Motivo())
	}
}

func TestEstacionNoRecibeFarol(t *testing.T) {
	turno := turnoPreflop(botsdk.Mano{carta(botsdk.Cuatro, botsdk.Corazones), carta(botsdk.Cinco, botsdk.Treboles)}, 0, 10000)
	turno.Estado.Etapa = botsdk.Flop
	turno.Estado.Comunitarias = []botsdk.Carta{
		carta(botsdk.As, botsdk.Picas), carta(botsdk.Rey, botsdk.Diamantes), carta(botsdk.Nueve, botsdk.Treboles),
	}
	turno.Estado.Pozo = 700
	turno.Estado.ApuestaActual = 0
	turno.Estado.HistorialAcciones = []botsdk.AccionRegistrada{
		{IDJugador: "artesanal", Etapa: botsdk.PreFlop, Accion: botsdk.Accion{Tipo: botsdk.Raise, Monto: 250}},
	}
	turno.Yo.ApuestaRonda = 0
	turno.Validas = []botsdk.TipoAccion{botsdk.Check, botsdk.Bet, botsdk.AllIn}
	v := Construir(Entrada{
		Turno: turno, Equidad: 0.12, TieneEquidad: true,
		Lectura: lectura.Lectura{Estacion: true},
	})
	if v.Linea() == LineaFarol || v.Linea() == LineaSemifarol {
		t.Fatalf("contra una estación no se farolea: %s", v.Linea())
	}
}

func TestConLineaNoPisaLaOriginal(t *testing.T) {
	v := vista(LineaValor, 0.33, 0, 0.7, "top", "seca", false, "valor")
	copia := v.ConLinea(LineaPasar)
	if v.Linea() != LineaValor || v.Equidad() != 0.7 {
		t.Fatal("la vista original cambió")
	}
	if copia.Linea() != LineaPasar || copia.Equidad() != 0.7 {
		t.Fatal("la copia no conserva la equidad")
	}
}
