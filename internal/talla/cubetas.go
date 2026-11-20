// Package talla elige el tamaño de la apuesta por cubetas. El tamaño cambia
// la frecuencia mínima de defensa: un tercio del pozo pide que el rival
// defienda el 75 %; dos tercios, el 60 %.
package talla

import (
	_ "embed"
	"encoding/json"
	"sync"

	"inteligenciaartesanal/internal/lectura"
)

//go:embed datos/cubetas.json
var raw []byte

// Cubetas son las fracciones del pozo. Se cargan de JSON.
type Cubetas struct {
	Seca            float64 `json:"seca"`
	Emparejada      float64 `json:"emparejada"`
	Mojada          float64 `json:"mojada"`
	Monotono        float64 `json:"monotono"`
	ValorEstacion   float64 `json:"valor_estacion"`
	ValorChico      float64 `json:"valor_chico"`
	FarolGrande     float64 `json:"farol_grande"`
	FarolSinMuestra float64 `json:"farol_sin_muestra"`
	FarolConMuestra float64 `json:"farol_con_muestra"`
	FarolSobreFold  float64 `json:"farol_sobre_fold"`
	FarolSonda      float64 `json:"farol_sonda"`
}

var (
	unaVez   sync.Once
	defecto  Cubetas
	errCarga error
)

// PorDefecto devuelve las cubetas embebidas.
func PorDefecto() Cubetas {
	unaVez.Do(func() {
		errCarga = json.Unmarshal(raw, &defecto)
	})
	if errCarga != nil {
		panic(errCarga)
	}
	return defecto
}

// Elegir devuelve la fracción del pozo. farol indica que la apuesta no es de valor.
func Elegir(textura string, farol bool, lec lectura.Lectura) float64 {
	c := PorDefecto()
	if farol {
		// El farol va grande: contra base y conservador, lo chico lo pagan.
		return c.FarolGrande
	}
	if lec.Estacion && !lec.PagaGrande {
		// Valor más chico: una estación paga top pair, no un pozo entero.
		return c.ValorChico
	}
	if lec.PagaGrande {
		return c.ValorEstacion
	}
	if lec.SoloPagaChico {
		return c.ValorChico
	}
	switch textura {
	case "mojada":
		return c.Mojada
	case "monotono":
		return c.Monotono
	case "emparejada":
		return c.Emparejada
	default:
		return c.Seca
	}
}

// DefensaMinima es la parte de las manos que hay que seguir para no ser
// explotado por un farol de ese tamaño: pozo / (pozo + apuesta) = 1 / (1+f).
func DefensaMinima(fraccion float64) float64 {
	if fraccion <= 0 {
		return 1
	}
	return 1 / (1 + fraccion)
}

// FoldNecesario es la frecuencia de fold para que un farol de esa fracción
// no pierda fichas: f / (1+f).
func FoldNecesario(fraccion float64) float64 {
	if fraccion <= 0 {
		return 0
	}
	return fraccion / (1 + fraccion)
}
