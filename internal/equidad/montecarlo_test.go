package equidad

import (
	"context"
	"testing"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

func TestEquidadDeTripsDentroDelPresupuesto(t *testing.T) {
	consulta := Consulta{
		Mano: botsdk.Mano{
			{Rango: botsdk.As, Palo: botsdk.Picas},
			{Rango: botsdk.As, Palo: botsdk.Corazones},
		},
		Comunitarias: []botsdk.Carta{
			{Rango: botsdk.As, Palo: botsdk.Treboles},
			{Rango: botsdk.Siete, Palo: botsdk.Diamantes},
			{Rango: botsdk.Dos, Palo: botsdk.Treboles},
		},
		Rivales:         1,
		MaxSimulaciones: 4000,
		Presupuesto:     160 * time.Millisecond,
	}
	inicio := time.Now()
	res := Estimar(context.Background(), 7, consulta)
	paso := time.Since(inicio)
	if paso > 250*time.Millisecond {
		t.Fatalf("el monte carlo se pasó del presupuesto corto: %s", paso)
	}
	if res.Simulaciones < 50 {
		t.Fatalf("muy pocas simulaciones: %d", res.Simulaciones)
	}
	if res.Valor < 0.85 {
		t.Fatalf("trío de ases contra una mano al azar: equidad %.3f", res.Valor)
	}
}

func TestEquidadContraRangoDeValorEsMasBaja(t *testing.T) {
	consulta := Consulta{
		Mano: botsdk.Mano{
			{Rango: botsdk.As, Palo: botsdk.Corazones},
			{Rango: botsdk.Rey, Palo: botsdk.Diamantes},
		},
		Comunitarias: []botsdk.Carta{
			{Rango: botsdk.As, Palo: botsdk.Picas},
			{Rango: botsdk.Siete, Palo: botsdk.Treboles},
			{Rango: botsdk.Dos, Palo: botsdk.Diamantes},
		},
		Rivales:         1,
		MaxSimulaciones: 800,
		Presupuesto:     200 * time.Millisecond,
	}
	alAzar := Estimar(context.Background(), 3, consulta)
	consulta.Rangos = [][]string{{"AA", "KK", "77", "22", "A7s", "A7o", "A2s"}}
	contraValor := Estimar(context.Background(), 3, consulta)
	if alAzar.Valor < 0.60 {
		t.Fatalf("AK en A72 contra el azar debería ir holgado: %.2f", alAzar.Valor)
	}
	if contraValor.Valor > alAzar.Valor-0.15 {
		t.Fatalf("contra valor la equidad tiene que bajar: azar %.2f valor %.2f", alAzar.Valor, contraValor.Valor)
	}
}

func TestEquidadRespetaElContexto(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	res := Estimar(ctx, 1, Consulta{
		Mano: botsdk.Mano{
			{Rango: botsdk.Rey, Palo: botsdk.Picas},
			{Rango: botsdk.Reina, Palo: botsdk.Corazones},
		},
		Comunitarias: []botsdk.Carta{
			{Rango: botsdk.Dos, Palo: botsdk.Treboles},
			{Rango: botsdk.Siete, Palo: botsdk.Diamantes},
			{Rango: botsdk.Nueve, Palo: botsdk.Picas},
		},
		Rivales:         1,
		MaxSimulaciones: 1_000_000,
		Presupuesto:     time.Second,
	})
	if res.Simulaciones == 0 {
		t.Fatal("tiene que devolver al menos un bloque de simulaciones")
	}
}
