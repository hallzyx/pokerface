package sesion

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/bot"
)

func TestReconectaSiLaMesaCaeAMitadDeMano(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	var bitacora bytes.Buffer
	logger := log.New(&bitacora, "", 0)
	token := "pfb-super-secreto"
	hecho := make(chan error, 1)
	go func() {
		hecho <- Mantener(ctx, bot.Nuevo(), Config{
			Direccion: ln.Addr().String(),
			Token:     token,
			Politica:  Politica{Inicial: 20 * time.Millisecond, Maximo: 40 * time.Millisecond},
			Log:       logger,
		})
	}()

	primera := atenderHastaCortar(t, ln, token, "mano-caida")
	if primera.IDMano != "mano-caida" {
		t.Fatalf("antes del corte: id %q", primera.IDMano)
	}

	segunda := atenderHastaCortar(t, ln, token, "mano-vuelta")
	if segunda.IDMano != "mano-vuelta" {
		t.Fatalf("después de reconectar: id %q", segunda.IDMano)
	}

	cancelar()
	select {
	case err := <-hecho:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("salida: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no salió al cancelar")
	}
	if strings.Contains(bitacora.String(), token) {
		t.Fatalf("el log incluye el token:\n%s", bitacora.String())
	}
	if !strings.Contains(bitacora.String(), "reintento") {
		t.Fatalf("no quedó el reintento en el log:\n%s", bitacora.String())
	}
}

func TestReconectaDentroDeLaVentana(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	reloj := nuevoReloj()
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	hecho := make(chan error, 1)
	go func() {
		hecho <- Mantener(ctx, bot.Nuevo(), Config{
			Direccion:  ln.Addr().String(),
			Token:      "pfb-super-secreto",
			Politica:   Politica{Inicial: time.Second, Maximo: 2 * time.Second},
			Ventana:    20 * time.Minute,
			Reloj:      reloj,
			Reintentos: 1,
			EsperaDial: time.Millisecond,
			Log:        log.New(io.Discard, "", 0),
		})
	}()
	parar := hacerAvanzar(reloj, 100*time.Millisecond)
	defer parar()

	if accion := atenderHastaCortar(t, ln, "pfb-super-secreto", "mano-1"); accion.IDMano != "mano-1" {
		t.Fatalf("primera mano: %q", accion.IDMano)
	}
	if accion := atenderHastaCortar(t, ln, "pfb-super-secreto", "mano-2"); accion.IDMano != "mano-2" {
		t.Fatalf("reconexión dentro de la ventana: %q", accion.IDMano)
	}
	select {
	case err := <-hecho:
		t.Fatalf("salió antes de tiempo: %v", err)
	default:
	}
	cancelar()
	select {
	case <-hecho:
	case <-time.After(3 * time.Second):
		t.Fatal("no salió al cancelar")
	}
}

func TestSaleSiLaVentanaSeAgota(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	reloj := nuevoReloj()
	var bitacora bytes.Buffer
	token := "pfb-super-secreto"
	hecho := make(chan error, 1)
	go func() {
		hecho <- Mantener(context.Background(), bot.Nuevo(), Config{
			Direccion:  ln.Addr().String(),
			Token:      token,
			Politica:   Politica{Inicial: time.Minute, Maximo: 8 * time.Minute},
			Ventana:    20 * time.Minute,
			Reloj:      reloj,
			Reintentos: 1,
			EsperaDial: time.Millisecond,
			Log:        log.New(&bitacora, "", 0),
		})
	}()
	// Un minuto de reloj falso por cada tic real: la ventana de 20 min se
	// agota enseguida, sin dormir el proceso.
	parar := hacerAvanzar(reloj, time.Minute)
	defer parar()
	_ = atenderHastaCortar(t, ln, token, "mano-ultima")
	_ = ln.Close()

	select {
	case err := <-hecho:
		if err != nil {
			t.Fatalf("la sesión presumida terminada tiene que salir con nil, got %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("no salió al agotarse la ventana")
	}
	texto := bitacora.String()
	if !strings.Contains(texto, "sesión o torneo presumido terminado") {
		t.Fatalf("falta el aviso de cierre:\n%s", texto)
	}
	if !strings.Contains(texto, "últimas fichas conocidas: 10000") {
		t.Fatalf("faltan las fichas:\n%s", texto)
	}
	if strings.Contains(texto, token) {
		t.Fatalf("el log incluye el token:\n%s", texto)
	}
}

func TestSilencioConSocketAbiertoNoSale(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	reloj := nuevoReloj()
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	hecho := make(chan error, 1)
	go func() {
		hecho <- Mantener(ctx, bot.Nuevo(), Config{
			Direccion: ln.Addr().String(),
			Token:     "pfb-super-secreto",
			Ventana:   20 * time.Minute,
			Reloj:     reloj,
			Log:       log.New(io.Discard, "", 0),
		})
	}()

	conn := aceptar(t, ln)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	lector := bufio.NewReader(conn)
	if _, err := lector.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	escribir(t, conn, `{"tipo":"bienvenida","version":"1.0.0","id_jugador":"artesanal","silla":0,"mensaje":"ok"}`)
	escribir(t, conn, mensajeMano("mano-silencio"))
	reloj.Avanzar(25 * time.Minute)
	select {
	case err := <-hecho:
		t.Fatalf("el silencio con el socket abierto cerró el proceso: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	escribir(t, conn, mensajeSolicitud("mano-silencio"))
	if accion := leer(t, lector); accion.IDMano != "mano-silencio" {
		t.Fatalf("después del silencio: %q", accion.IDMano)
	}
	cancelar()
}

type relojFalso struct {
	mu      sync.Mutex
	ahora   time.Time
	esperas []esperaReloj
}

type esperaReloj struct {
	cuando time.Time
	ch     chan time.Time
}

func nuevoReloj() *relojFalso {
	return &relojFalso{ahora: time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)}
}

func (r *relojFalso) Now() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ahora
}

func (r *relojFalso) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	r.mu.Lock()
	defer r.mu.Unlock()
	cuando := r.ahora.Add(d)
	if d <= 0 {
		ch <- r.ahora
		return ch
	}
	r.esperas = append(r.esperas, esperaReloj{cuando: cuando, ch: ch})
	return ch
}

func (r *relojFalso) Avanzar(d time.Duration) {
	r.mu.Lock()
	r.ahora = r.ahora.Add(d)
	ahora := r.ahora
	var siguen []esperaReloj
	var listos []esperaReloj
	for _, e := range r.esperas {
		if !e.cuando.After(ahora) {
			listos = append(listos, e)
		} else {
			siguen = append(siguen, e)
		}
	}
	r.esperas = siguen
	r.mu.Unlock()
	for _, e := range listos {
		e.ch <- ahora
	}
}

func hacerAvanzar(r *relojFalso, paso time.Duration) func() {
	fin := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-fin:
				return
			case <-t.C:
				r.Avanzar(paso)
			}
		}
	}()
	return func() { close(fin) }
}

func TestLogueaConexionManoYResumen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var bitacora bufSeguro
	logger := log.New(&bitacora, "", 0)
	token := "pfb-super-secreto"
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	hecho := make(chan error, 1)
	go func() {
		hecho <- Mantener(ctx, bot.Nuevo().ConBitacora(logger), Config{
			Direccion:  ln.Addr().String(),
			Token:      token,
			Politica:   Politica{Inicial: 20 * time.Millisecond, Maximo: 40 * time.Millisecond},
			Reintentos: 1,
			EsperaDial: time.Millisecond,
			Log:        logger,
		})
	}()

	conn := aceptar(t, ln)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	lector := bufio.NewReader(conn)
	if _, err := lector.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	escribir(t, conn, `{"tipo":"bienvenida","version":"1.0.0","id_jugador":"artesanal","silla":3,"mensaje":"ok"}`)
	escribir(t, conn, mensajeMano("mano-log"))
	escribir(t, conn, mensajeSolicitud("mano-log"))
	if accion := leer(t, lector); accion.IDMano != "mano-log" {
		t.Fatalf("acción: %+v", accion)
	}
	escribir(t, conn, `{"tipo":"mano_fin","version":"1.0.0","resultado":{"id_mano":"mano-log","repartos":[{"id_jugador":"artesanal","monto":150}],"comunitarias":[]}}`)

	limite := time.Now().Add(3 * time.Second)
	for time.Now().Before(limite) && !strings.Contains(bitacora.String(), "mano=mano-log") {
		time.Sleep(10 * time.Millisecond)
	}
	cancelar()
	select {
	case <-hecho:
	case <-time.After(3 * time.Second):
		t.Fatal("no salió al cancelar")
	}
	texto := bitacora.String()
	for _, quiero := range []string{
		"conectado a " + ln.Addr().String() + " como artesanal (silla 3)",
		"mano=mano-log",
		"pozo=150",
		"ganadores=artesanal:150",
		"showdown=0",
		"resumen manos=1",
		"sin_showdown=1",
	} {
		if !strings.Contains(texto, quiero) {
			t.Fatalf("falta %q en\n%s", quiero, texto)
		}
	}
	if strings.Contains(texto, token) {
		t.Fatalf("el log incluye el token:\n%s", texto)
	}
}

type bufSeguro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *bufSeguro) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *bufSeguro) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestNoReintentaSiRechazanLaCredencial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	hecho := make(chan error, 1)
	go func() {
		hecho <- Mantener(ctx, bot.Nuevo(), Config{
			Direccion: ln.Addr().String(),
			Token:     "token-malo",
			Politica:  Politica{Inicial: 30 * time.Millisecond, Maximo: 30 * time.Millisecond},
			Log:       log.New(io.Discard, "", 0),
		})
	}()

	conn := aceptar(t, ln)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	lector := bufio.NewReader(conn)
	if _, err := lector.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(conn, "{\"tipo\":\"error\",\"version\":\"1.0.0\",\"mensaje\":\"credencial desconocida\"}\n")

	select {
	case err := <-hecho:
		if err == nil || !RechazoDefinitivo(err) {
			t.Fatalf("debería ser un rechazo definitivo, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reintentó o se quedó colgado tras la credencial")
	}
}

func atenderHastaCortar(t *testing.T, ln net.Listener, token, idMano string) accionLeida {
	t.Helper()
	conn := aceptar(t, ln)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	lector := bufio.NewReader(conn)
	linea, err := lector.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var saludo struct {
		Tipo  string `json:"tipo"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(linea), &saludo); err != nil {
		t.Fatal(err)
	}
	if saludo.Tipo != "saludo" || saludo.Token != token {
		t.Fatalf("saludo: %+v", saludo)
	}
	escribir(t, conn, `{"tipo":"bienvenida","version":"1.0.0","id_jugador":"artesanal","silla":0,"mensaje":"ok"}`)
	escribir(t, conn, mensajeMano(idMano))
	escribir(t, conn, mensajeSolicitud(idMano))
	accion := leer(t, lector)
	return accion
}

func aceptar(t *testing.T, ln net.Listener) net.Conn {
	t.Helper()
	_ = ln.(*net.TCPListener).SetDeadline(time.Now().Add(4 * time.Second))
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func escribir(t *testing.T, conn net.Conn, s string) {
	t.Helper()
	if _, err := io.WriteString(conn, s+"\n"); err != nil {
		t.Fatal(err)
	}
}

func leer(t *testing.T, lector *bufio.Reader) accionLeida {
	t.Helper()
	linea, err := lector.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var msg accionLeida
	if err := json.Unmarshal([]byte(linea), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Tipo != "accion" || msg.Accion == nil {
		t.Fatalf("acción: %s", linea)
	}
	return msg
}

type accionLeida struct {
	Tipo   string         `json:"tipo"`
	IDMano string         `json:"id_mano"`
	Accion *botsdk.Accion `json:"accion"`
}

func mensajeMano(id string) string {
	return `{"tipo":"mano_inicio","version":"1.0.0","cartas":[{"rango":14,"palo":3},{"rango":13,"palo":2}],"estado":` + estado(id) + `}`
}

func mensajeSolicitud(id string) string {
	return `{"tipo":"solicitar_accion","version":"1.0.0","acciones_validas":["fold","raise","allin"],"timeout_ms":5000,"estado":` + estado(id) + `}`
}

func estado(id string) string {
	return `{"id_mano":"` + id + `","etapa":"preflop","pozo":150,"apuesta_actual":100,"subida_minima":100,"ciega_chica":50,"ciega_grande":100,"posicion_boton":1,"jugadores":[{"id":"artesanal","nombre":"artesanal","saldo":9900,"apuesta_ronda":100,"activo":true,"en_torneo":true,"en_mano":true,"posicion_silla":0},{"id":"rival","nombre":"rival","saldo":9950,"apuesta_ronda":50,"activo":true,"en_torneo":true,"en_mano":true,"posicion_silla":1}]}`
}
