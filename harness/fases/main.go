// Mesa de una ronda de la clasificatoria. Se compila dentro del módulo del kit
// (el script la copia a un árbol temporal) para poder sentar a cada bot con
// las fichas que trae de la ronda anterior. No modifica el submódulo.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/internal/crupier"
	"github.com/aicspucp/pokerfight-arena-participantes/internal/mesa"
)

func main() {
	addr := flag.String("addr", ":9000", "dirección de escucha (host:puerto)")
	maxJugadores := flag.Uint("jugadores", 6, "cantidad máxima de jugadores (2-8)")
	minJugadores := flag.Uint("min-jugadores", 2, "cantidad mínima para arrancar")
	ciegaChica := flag.Uint64("ciega-chica", 50, "ciega chica")
	ciegaGrande := flag.Uint64("ciega-grande", 100, "ciega grande")
	stack := flag.Uint64("stack", 10000, "fichas de quien no figure en -stacks")
	stacksRuta := flag.String("stacks", "", "JSON id→fichas de la ronda anterior")
	timeoutMs := flag.Uint64("timeout-ms", 5000, "plazo por jugada, en milisegundos")
	rondas := flag.Int64("rondas", 50, "manos de la ronda")
	semilla := flag.Uint64("semilla", 0, "semilla del crupier; 0 usa la fuente criptográfica")
	orden := flag.String("orden", "", "ids en orden de silla, separados por coma")
	id := flag.String("id", "mesa-1", "identificador de la mesa")
	flag.Parse()

	if *maxJugadores < 2 || *maxJugadores > 8 {
		log.Fatalf("fases: -jugadores debe estar entre 2 y 8, recibido %d", *maxJugadores)
	}
	if *minJugadores < 2 || *minJugadores > *maxJugadores {
		log.Fatalf("fases: -min-jugadores fuera de rango")
	}

	var stacks map[string]uint64
	if *stacksRuta != "" {
		datos, err := os.ReadFile(*stacksRuta)
		if err != nil {
			log.Fatalf("fases: no se pudo leer %s: %v", *stacksRuta, err)
		}
		if err := json.Unmarshal(datos, &stacks); err != nil {
			log.Fatalf("fases: %s no es un mapa de fichas: %v", *stacksRuta, err)
		}
	}

	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()

	var dealer crupier.Crupier
	if *semilla == 0 {
		dealer = crupier.Nuevo()
	} else {
		dealer = crupier.NuevoConSemilla(*semilla)
	}

	m := mesa.NuevaMesa(*id, mesa.ConfigMesa{
		MaxJugadores: uint8(*maxJugadores),
		MinJugadores: uint8(*minJugadores),
		Timeout:      *timeoutMs,
	}, mesa.ConfigPartida{
		CiegaMenor:      *ciegaChica,
		CiegaMayor:      *ciegaGrande,
		StackInicial:    *stack,
		CantidadRondas:  *rondas,
		StackPorJugador: stacks,
	}, dealer)

	sillas := map[string]int{}
	for i, idJugador := range strings.Split(*orden, ",") {
		idJugador = strings.TrimSpace(idJugador)
		if idJugador != "" {
			sillas[idJugador] = i
		}
	}
	servidor := mesa.NuevoServidor(mesa.ConfigServidor{
		Direccion:          *addr,
		TimeoutHandshakeMs: *timeoutMs,
		MaxConexiones:      int(*maxJugadores),
		SillaDe: func(idJugador string) (int, bool) {
			silla, ok := sillas[idJugador]
			return silla, ok
		},
	}, m, nil)

	if err := servidor.Abrir(); err != nil {
		log.Fatalf("fases: no se pudo abrir %s: %v", *addr, err)
	}
	log.Printf("mesa [%s]: escuchando en %s, esperando %d-%d jugadores", *id, servidor.Direccion(), *minJugadores, *maxJugadores)
	defer servidor.Cerrar()

	go func() {
		if err := servidor.Servir(ctx); err != nil {
			log.Printf("mesa [%s]: servidor detenido: %v", *id, err)
		}
	}()

	if !esperar(ctx, m, int(*minJugadores)) {
		log.Printf("mesa [%s]: cancelada antes de completar el mínimo", *id)
		return
	}
	log.Printf("mesa [%s]: mínimo de jugadores alcanzado, arrancando la partida", *id)
	resumen, err := m.Jugar(ctx)
	if err != nil {
		log.Printf("mesa [%s]: la partida terminó con error: %v", *id, err)
	}
	fmt.Printf("Mesa [%s]: partida terminada, %d rondas jugadas.\n", resumen.IDMesa, resumen.CantidadRondasJugadas)
	for _, p := range resumen.Posiciones {
		fmt.Printf("  #%d %-20s %d fichas\n", p.Posicion, p.IDJugador, p.SaldoFinal)
	}
}

func esperar(ctx context.Context, m mesa.MesaInterface, min int) bool {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		if m.Sentados() >= min {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
	}
}
