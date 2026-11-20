// Package teatro traduce la vista del plan a una acción legal. Recibe la
// vista por valor y no importa el modelo de rivales: no puede escribir
// creencias, solo elegir la acción que ven los demás.
package teatro

import (
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/plan"
)

// Presentar convierte la línea en una acción que la mesa puede aceptar.
func Presentar(v plan.Vista, t botsdk.Turno) botsdk.Accion {
	switch v.Linea() {
	case plan.LineaAllIn:
		if t.Puede(botsdk.AllIn) {
			return t.AllIn()
		}
		return t.Apostar(t.Techo())
	case plan.LineaPagar:
		if t.Puede(botsdk.Call) {
			return t.Pagar()
		}
		if t.Puede(botsdk.AllIn) {
			return t.AllIn()
		}
		if t.Puede(botsdk.Check) {
			return t.Check()
		}
		return t.Fold()
	case plan.LineaPasar:
		if t.Puede(botsdk.Check) {
			return t.Check()
		}
		return t.Fold()
	case plan.LineaRetirarse:
		if t.PorIgualar() == 0 && t.Puede(botsdk.Check) {
			return t.Check()
		}
		return t.Fold()
	case plan.LineaValor, plan.LineaValorFino, plan.LineaFarol, plan.LineaSemifarol:
		if v.Monto() > 0 {
			return t.Apostar(v.Monto())
		}
		if v.Fraccion() > 0 {
			return t.ApostarPozo(v.Fraccion())
		}
		if t.Puede(botsdk.Check) {
			return t.Check()
		}
		if t.Puede(botsdk.Call) {
			return t.Pagar()
		}
		return t.Fold()
	default:
		if t.Puede(botsdk.Check) {
			return t.Check()
		}
		return t.Fold()
	}
}
