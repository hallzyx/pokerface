// Package sesion mantiene al bot conectado durante la clasificatoria.
//
// El SDK termina cuando el socket se cierra, y ese cierre es el mismo si la
// sesión acabó o si el servidor se cayó. Mientras no hay un rechazo definitivo
// (credencial o versión) ni el usuario para el proceso, se vuelve a entrar con
// el mismo token. La espera crece y tiene techo, para no martillar el puerto.
package sesion

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

// VentanaPorDefecto es cuánto se reintenta tras una caída antes de dar la
// sesión por terminada.
const VentanaPorDefecto = 20 * time.Minute

// Politica acota la espera entre reintentos.
type Politica struct {
	Inicial time.Duration
	Maximo  time.Duration
}

// Reloj es el tiempo que mira la ventana. Los tests ponen uno falso.
type Reloj interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type relojReal struct{}

func (relojReal) Now() time.Time { return time.Now() }

func (relojReal) After(d time.Duration) <-chan time.Time { return time.After(d) }

// PorDefecto espera medio segundo y no pasa de 8 s.
func PorDefecto() Politica {
	return Politica{Inicial: 500 * time.Millisecond, Maximo: 8 * time.Second}
}

// Config es la conexión que se reabre. Token no se escribe en el log.
type Config struct {
	Direccion string
	Token     string
	Politica  Politica
	// Ventana es el plazo desde la última caída para lograr un reingreso.
	// Cero usa VentanaPorDefecto. Un reingreso que llega a jugar la pone en cero.
	Ventana time.Duration
	Reloj   Reloj
	// Reintentos y EsperaDial se pasan al SDK. Cero deja los de siempre.
	Reintentos int
	EsperaDial time.Duration
	Log        *log.Logger
}

// InformaEmpuje deja contar, al caer una mesa, cuántas veces un rival se fue
// ante un all-in. Es opcional.
type InformaEmpuje interface {
	Empuje() (encara, seFue int)
}

// InformaFichas es el último stack visto. Es opcional.
type InformaFichas interface {
	Fichas() (n int64, conocido bool)
}

// InformaResumen es el cierre de la sesión. Es opcional.
type InformaResumen interface {
	LineaResumen() string
}

// Mantener llama a botsdk.Correr y, si la mesa se cae, vuelve a entrar.
// El silencio con el socket abierto no vuelve: Correr sigue bloqueado en la
// lectura. Sale si ctx se cancela, si la mesa rechaza la credencial o la
// versión, o si pasa Ventana sin un reingreso desde la última caída. Ese
// último caso devuelve nil: la sesión se presume terminada y el proceso
// debe salir con código 0.
func Mantener(ctx context.Context, bot botsdk.Bot, cfg Config) error {
	if cfg.Politica.Inicial <= 0 {
		cfg.Politica.Inicial = 500 * time.Millisecond
	}
	if cfg.Politica.Maximo < cfg.Politica.Inicial {
		cfg.Politica.Maximo = cfg.Politica.Inicial
	}
	if cfg.Ventana <= 0 {
		cfg.Ventana = VentanaPorDefecto
	}
	if cfg.Reloj == nil {
		cfg.Reloj = relojReal{}
	}
	token := cfg.Token
	if token == "" {
		token = bot.Nombre()
	}
	espera := cfg.Politica.Inicial
	intento := 0
	var caida time.Time
	defer anotarResumen(cfg, bot)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		intento++
		err := botsdk.Correr(ctx, bot, botsdk.Opciones{
			Direccion:       cfg.Direccion,
			Token:           token,
			Reintentos:      cfg.Reintentos,
			EsperaReintento: cfg.EsperaDial,
			Log:             cfg.Log,
		})
		anotarEmpuje(cfg.Log, bot)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}
		if RechazoDefinitivo(err) {
			registrar(cfg.Log, "rechazo definitivo, no reintento: %v", err)
			return err
		}
		ahora := cfg.Reloj.Now()
		if errors.Is(err, botsdk.ErrMesaCerro) {
			// Hubo socket. Un reingreso que llegó a jugar pone la ventana en cero
			// y esta caída abre otra.
			caida = ahora
		} else if !caida.IsZero() && ahora.Sub(caida) >= cfg.Ventana {
			return presumirFin(cfg, bot)
		}
		registrar(cfg.Log, "conexión caída (%v); reintento %d en %s", err, intento, espera.Round(time.Millisecond))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-cfg.Reloj.After(espera):
		}
		if !caida.IsZero() && cfg.Reloj.Now().Sub(caida) >= cfg.Ventana {
			return presumirFin(cfg, bot)
		}
		espera *= 2
		if espera > cfg.Politica.Maximo {
			espera = cfg.Politica.Maximo
		}
	}
}

func anotarResumen(cfg Config, bot botsdk.Bot) {
	info, ok := bot.(InformaResumen)
	if !ok {
		return
	}
	registrar(cfg.Log, "%s", info.LineaResumen())
}

func presumirFin(cfg Config, bot botsdk.Bot) error {
	linea := fmt.Sprintf("sesión o torneo presumido terminado: no hubo reconexión en %s", cfg.Ventana)
	if info, ok := bot.(InformaFichas); ok {
		if n, conocido := info.Fichas(); conocido {
			linea += fmt.Sprintf("; últimas fichas conocidas: %d", n)
		} else {
			linea += "; fichas desconocidas"
		}
	}
	registrar(cfg.Log, "%s", linea)
	return nil
}

// RechazoDefinitivo es una respuesta de la mesa que no se arregla reintentando:
// credencial desconocida o versión de protocolo distinta. El cierre del socket
// no lo es: también aparece cuando el servidor se cae.
func RechazoDefinitivo(err error) bool {
	if err == nil || errors.Is(err, botsdk.ErrMesaCerro) {
		return false
	}
	texto := strings.ToLower(err.Error())
	if strings.Contains(texto, "credencial") {
		return true
	}
	if strings.Contains(texto, "versión de protocolo") || strings.Contains(texto, "version de protocolo") {
		return true
	}
	return false
}

func anotarEmpuje(logger *log.Logger, bot botsdk.Bot) {
	informa, ok := bot.(InformaEmpuje)
	if !ok {
		return
	}
	encara, seFue := informa.Empuje()
	registrar(logger, "stats fold_al_empuje %d/%d", seFue, encara)
}

func registrar(logger *log.Logger, formato string, args ...any) {
	if logger == nil {
		return
	}
	logger.Output(2, fmt.Sprintf(formato, args...))
}
