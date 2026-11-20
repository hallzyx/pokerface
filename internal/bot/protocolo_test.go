package bot

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

func TestIDManoCoincideConElEstado(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	hecho := make(chan error, 1)
	go func() {
		hecho <- botsdk.Correr(context.Background(), Nuevo(), botsdk.Opciones{
			Direccion: ln.Addr().String(),
			Token:     "artesanal",
		})
	}()

	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	lector := bufio.NewReader(conn)

	saludo, err := lector.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(saludo)) || !contains(saludo, "saludo") {
		t.Fatalf("saludo: %s", saludo)
	}
	enviar(t, conn, `{"tipo":"bienvenida","version":"1.0.0","id_jugador":"artesanal","silla":3,"mensaje":"ok"}`)
	enviar(t, conn, mensajeMano("mesa-prueba-mano-4"))
	enviar(t, conn, mensajeSolicitud("mesa-prueba-mano-4"))
	primera := leerAccion(t, lector)
	if primera.IDMano != "mesa-prueba-mano-4" {
		t.Fatalf("id_mano: got %q", primera.IDMano)
	}
	if primera.Accion == nil || primera.Accion.Tipo == "" {
		t.Fatalf("acción vacía: %+v", primera)
	}

	enviar(t, conn, mensajeSolicitud("mesa-prueba-mano-5"))
	segunda := leerAccion(t, lector)
	if segunda.IDMano != "mesa-prueba-mano-5" {
		t.Fatalf("la segunda respuesta usó %q", segunda.IDMano)
	}
	if segunda.IDMano == primera.IDMano {
		t.Fatal("repitió el id de la mano anterior")
	}

	_ = conn.Close()
	select {
	case <-hecho:
	case <-time.After(3 * time.Second):
		t.Fatal("el bot no soltó la conexión")
	}
}

type sobre struct {
	Tipo   string         `json:"tipo"`
	IDMano string         `json:"id_mano"`
	Accion *botsdk.Accion `json:"accion"`
}

func leerAccion(t *testing.T, lector *bufio.Reader) sobre {
	t.Helper()
	linea, err := lector.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var msg sobre
	if err := json.Unmarshal([]byte(linea), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Tipo != "accion" {
		t.Fatalf("tipo %s: %s", msg.Tipo, linea)
	}
	return msg
}

func enviar(t *testing.T, conn net.Conn, s string) {
	t.Helper()
	if _, err := conn.Write([]byte(s + "\n")); err != nil {
		t.Fatal(err)
	}
}

func mensajeMano(id string) string {
	return `{"tipo":"mano_inicio","version":"1.0.0","cartas":[{"rango":14,"palo":3},{"rango":14,"palo":2}],"estado":` + estadoJSON(id) + `}`
}

func mensajeSolicitud(id string) string {
	return `{"tipo":"solicitar_accion","version":"1.0.0","acciones_validas":["fold","call","raise","allin"],"timeout_ms":5000,"estado":` + estadoJSON(id) + `}`
}

func estadoJSON(id string) string {
	estado := botsdk.EstadoPublico{
		IDMano: id, Etapa: botsdk.PreFlop, Pozo: 150,
		ApuestaActual: 100, SubidaMinima: 100,
		CiegaChica: 50, CiegaGrande: 100, PosicionBoton: 0,
		Jugadores: []botsdk.JugadorPublico{
			{ID: "artesanal", Nombre: "artesanal", Saldo: 10000, Activo: true, EnTorneo: true, EnMano: true, PosicionSilla: 3},
			{ID: "rival", Nombre: "rival", Saldo: 9900, ApuestaRonda: 100, Activo: true, EnTorneo: true, EnMano: true, PosicionSilla: 2},
		},
	}
	raw, _ := json.Marshal(estado)
	return string(raw)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
