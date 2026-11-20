package preflop

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
)

//go:embed datos/*.json
var datos embed.FS

// Tamanos son los números de la estrategia preflop. Viven en JSON para poder
// cambiarlos sin reescribir el plan.
type Tamanos struct {
	Version              string  `json:"version"`
	AbrirCiegas          float64 `json:"abrir_ciegas"`
	AbrirExplotadoCiegas float64 `json:"abrir_explotado_ciegas"`
	TresBetPor           float64 `json:"tresbet_por"`
	CuatroBetPor         float64 `json:"cuatrobet_por"`
	StackPushBB          float64 `json:"stack_push_bb"`
}

// Tablas agrupa aperturas, defensa, 3-bet y push/fold. Cada conjunto es un
// mapa de claves canónicas. No se muta después de cargar.
type Tablas struct {
	sets    map[string]map[string]struct{}
	Tamanos Tamanos
}

var (
	unaVez   sync.Once
	defecto  *Tablas
	errCarga error
)

// PorDefecto devuelve las tablas embebidas. Un JSON roto es un error de
// programación: el binario no debería arrancar con rangos a medias.
func PorDefecto() *Tablas {
	unaVez.Do(func() {
		defecto, errCarga = Cargar()
	})
	if errCarga != nil {
		panic(errCarga)
	}
	return defecto
}

// Cargar lee los JSON embebidos.
func Cargar() (*Tablas, error) {
	t := &Tablas{sets: map[string]map[string]struct{}{}}
	if err := t.cargarMapa("datos/apertura.json", "apertura"); err != nil {
		return nil, err
	}
	if err := t.cargarMapa("datos/defensa.json", "defensa"); err != nil {
		return nil, err
	}
	if err := t.cargarMapa("datos/tresbet.json", "tresbet"); err != nil {
		return nil, err
	}
	if err := t.cargarPush("datos/pushfold.json", "empujar", "pagar"); err != nil {
		return nil, err
	}
	if err := t.cargarPush("datos/pushfold_medido.json", "empujar_medido", "pagar"); err != nil {
		return nil, err
	}
	raw, err := datos.ReadFile("datos/tamanos.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &t.Tamanos); err != nil {
		return nil, fmt.Errorf("preflop: tamanos.json: %w", err)
	}
	if t.Tamanos.AbrirCiegas <= 0 || t.Tamanos.StackPushBB <= 0 {
		return nil, fmt.Errorf("preflop: tamanos.json incompleto")
	}
	return t, nil
}

func (t *Tablas) cargarMapa(ruta, familia string) error {
	raw, err := datos.ReadFile(ruta)
	if err != nil {
		return err
	}
	var archivo map[string][]string
	if err := json.Unmarshal(raw, &archivo); err != nil {
		return fmt.Errorf("preflop: %s: %w", ruta, err)
	}
	for puesto, exprs := range archivo {
		set, err := compilar(exprs)
		if err != nil {
			return fmt.Errorf("preflop: %s %s: %w", ruta, puesto, err)
		}
		t.sets[familia+"/"+puesto] = set
	}
	return nil
}

func (t *Tablas) cargarPush(ruta, familiaEmpuje, familiaPago string) error {
	raw, err := datos.ReadFile(ruta)
	if err != nil {
		return err
	}
	var archivo map[string]struct {
		Empujar map[string][]string `json:"empujar"`
		Pagar   map[string][]string `json:"pagar"`
	}
	if err := json.Unmarshal(raw, &archivo); err != nil {
		return fmt.Errorf("preflop: %s: %w", ruta, err)
	}
	for cubo, bloque := range archivo {
		for puesto, exprs := range bloque.Empujar {
			set, err := compilar(exprs)
			if err != nil {
				return fmt.Errorf("preflop: push %s %s: %w", cubo, puesto, err)
			}
			t.sets[familiaEmpuje+"/"+cubo+"/"+puesto] = set
		}
		for puesto, exprs := range bloque.Pagar {
			set, err := compilar(exprs)
			if err != nil {
				return fmt.Errorf("preflop: pagar %s %s: %w", cubo, puesto, err)
			}
			t.sets[familiaPago+"/"+cubo+"/"+puesto] = set
		}
	}
	return nil
}

func compilar(exprs []string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
	for _, expr := range exprs {
		manos, err := Expandir(expr)
		if err != nil {
			return nil, err
		}
		for _, m := range manos {
			set[m] = struct{}{}
		}
	}
	return set, nil
}

// Tiene informa si la clave está en familia/puesto (por ejemplo apertura/UTG
// o empujar/10/BTN).
func (t *Tablas) Tiene(familia, puesto, clave string) bool {
	if t == nil || clave == "" {
		return false
	}
	_, ok := t.sets[familia+"/"+puesto][clave]
	return ok
}

// Lista devuelve una copia de las claves de ese conjunto, para estimar rangos.
func (t *Tablas) Lista(familia, puesto string) []string {
	set := t.sets[familia+"/"+puesto]
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// Clave devuelve la forma canónica de dos cartas: "AKs", "TT", "72o".
func Clave(mano botsdk.Mano) (string, bool) {
	a, b := mano[0], mano[1]
	if a.Rango < botsdk.Dos || b.Rango < botsdk.Dos || a == b {
		return "", false
	}
	hi, lo := a, b
	if b.Rango > a.Rango || (b.Rango == a.Rango && b.Palo > a.Palo) {
		hi, lo = b, a
	}
	rh := string(letra(int(hi.Rango)))
	rl := string(letra(int(lo.Rango)))
	if hi.Rango == lo.Rango {
		return rh + rl, true
	}
	if hi.Palo == lo.Palo {
		return rh + rl + "s", true
	}
	return rh + rl + "o", true
}

// ManoValida indica si hay dos cartas distintas de verdad.
func ManoValida(mano botsdk.Mano) bool {
	_, ok := Clave(mano)
	return ok
}

// CuboPush elige la tabla de push/fold según el stack efectivo en ciegas.
func CuboPush(bbEfectivas float64) string {
	switch {
	case bbEfectivas <= 8:
		return "8"
	case bbEfectivas <= 10:
		return "10"
	default:
		return "15"
	}
}

// SirveParaRobar amplia el robo en posición tardía contra quien se va mucho.
// 72o no entra: hace falta un as, un rey, una pareja o dos del mismo palo.
func SirveParaRobar(clave, puesto string) bool {
	if puesto != "CO" && puesto != "BTN" && puesto != "SB" {
		return false
	}
	if len(clave) < 2 {
		return false
	}
	if clave[0] == clave[1] || clave[0] == 'A' || clave[0] == 'K' {
		return true
	}
	return len(clave) == 3 && clave[2] == 's'
}
