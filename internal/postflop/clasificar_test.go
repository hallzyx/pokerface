package postflop

import (
	"testing"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/poker"
)

func c(r botsdk.Rango, p botsdk.Palo) botsdk.Carta { return botsdk.Carta{Rango: r, Palo: p} }

func TestClasificaTrioYProyecto(t *testing.T) {
	mano := botsdk.Mano{c(botsdk.As, botsdk.Picas), c(botsdk.As, botsdk.Corazones)}
	board := []botsdk.Carta{c(botsdk.As, botsdk.Treboles), c(botsdk.Siete, botsdk.Diamantes), c(botsdk.Dos, botsdk.Treboles)}
	clase := Clasificar(mano, board)
	if clase.Categoria != poker.Trio || clase.Fuerza != FuerzaMonstruo {
		t.Fatalf("trío: %+v", clase)
	}
	seco := []botsdk.Carta{c(botsdk.As, botsdk.Picas), c(botsdk.Siete, botsdk.Corazones), c(botsdk.Dos, botsdk.Diamantes)}
	if Textura(seco) != "seca" {
		t.Fatalf("A72 arcoíris: %s", Textura(seco))
	}

	proyecto := botsdk.Mano{c(botsdk.Nueve, botsdk.Corazones), c(botsdk.Ocho, botsdk.Corazones)}
	boardP := []botsdk.Carta{c(botsdk.Siete, botsdk.Treboles), c(botsdk.Seis, botsdk.Diamantes), c(botsdk.Dos, botsdk.Picas)}
	clase = Clasificar(proyecto, boardP)
	if clase.Outs < 8 {
		t.Fatalf("escalera abierta debería tener al menos 8 outs, tiene %d", clase.Outs)
	}

	color := botsdk.Mano{c(botsdk.As, botsdk.Corazones), c(botsdk.Cinco, botsdk.Corazones)}
	boardC := []botsdk.Carta{c(botsdk.Rey, botsdk.Corazones), c(botsdk.Nueve, botsdk.Corazones), c(botsdk.Dos, botsdk.Treboles)}
	clase = Clasificar(color, boardC)
	if clase.Outs < 9 {
		t.Fatalf("proyecto de color: %d outs", clase.Outs)
	}
	if Textura(boardC) != "mojada" && Textura(boardC) != "monotono" {
		t.Fatalf("tablero a dos del color y conectado: %s", Textura(boardC))
	}
}

func TestTexturas(t *testing.T) {
	mono := []botsdk.Carta{c(botsdk.Jota, botsdk.Corazones), c(botsdk.Diez, botsdk.Corazones), c(botsdk.Nueve, botsdk.Corazones)}
	if Textura(mono) != "monotono" {
		t.Fatalf("monótono: %s", Textura(mono))
	}
	par := []botsdk.Carta{c(botsdk.Siete, botsdk.Picas), c(botsdk.Siete, botsdk.Corazones), c(botsdk.Dos, botsdk.Treboles)}
	if Textura(par) != "emparejada" {
		t.Fatalf("emparejada: %s", Textura(par))
	}
	seco := []botsdk.Carta{c(botsdk.As, botsdk.Picas), c(botsdk.Siete, botsdk.Corazones), c(botsdk.Dos, botsdk.Treboles)}
	if Textura(seco) != "seca" {
		t.Fatalf("seca: %s", Textura(seco))
	}
}

func TestPriorReglaDelDosYElCuatro(t *testing.T) {
	proyecto := Clasificar(
		botsdk.Mano{c(botsdk.As, botsdk.Corazones), c(botsdk.Cinco, botsdk.Corazones)},
		[]botsdk.Carta{c(botsdk.Rey, botsdk.Corazones), c(botsdk.Nueve, botsdk.Corazones), c(botsdk.Dos, botsdk.Treboles)},
	)
	flop := proyecto.Prior(1, 2)
	river := proyecto.Prior(1, 1)
	if flop < 0.30 || flop > 0.55 {
		t.Fatalf("prior en el flop ~36%%, got %.2f", flop)
	}
	if river >= flop {
		t.Fatalf("con una carta por venir el prior baja: flop %.2f river %.2f", flop, river)
	}
}
