// Comando artesanal: el bot de Inteligencia artesanal para PokerFight 2026.
//
// El token no se guarda. Sale de -token o, si no viene, de POKERFIGHT_TOKEN.
// Si la mesa cierra el socket, el proceso vuelve a entrar con el mismo token
// durante -ventana-reconexion (20 min). Si no lo logra, da la sesión por
// terminada y sale con código 0. El silencio entre rondas, con el socket
// abierto, no dispara nada. Ctrl+C y un rechazo de credencial o de versión
// salen enseguida.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"inteligenciaartesanal/internal/bot"
	"inteligenciaartesanal/internal/sesion"
)

func main() {
	if flag.Lookup("addr") == nil {
		flag.String("addr", "localhost:9000", "dirección de la mesa (host:puerto)")
	}
	if flag.Lookup("token") == nil {
		flag.String("token", os.Getenv("POKERFIGHT_TOKEN"), "token del equipo; también vale la variable POKERFIGHT_TOKEN")
	}
	if flag.Lookup("semilla") == nil {
		flag.Uint64("semilla", 20261110, "semilla del monte carlo")
	}
	if flag.Lookup("detalle") == nil {
		flag.Bool("detalle", false, "escribe en stderr la línea y la equidad de cada decisión")
	}
	if flag.Lookup("ventana-reconexion") == nil {
		flag.Duration("ventana-reconexion", sesion.VentanaPorDefecto, "plazo para volver a entrar tras una caída; si se agota, la sesión se da por terminada")
	}
	flag.Parse()

	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()

	token := flag.Lookup("token").Value.String()
	if token == "" {
		token = os.Getenv("POKERFIGHT_TOKEN")
	}
	ventana := sesion.VentanaPorDefecto
	if f := flag.Lookup("ventana-reconexion"); f != nil {
		if d, err := time.ParseDuration(f.Value.String()); err == nil && d > 0 {
			ventana = d
		}
	}
	err := sesion.Mantener(ctx, bot.Nuevo(), sesion.Config{
		Direccion: flag.Lookup("addr").Value.String(),
		Token:     token,
		Politica:  sesion.PorDefecto(),
		Ventana:   ventana,
		Log:       log.Default(),
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("artesanal: %v", err)
	}
}
