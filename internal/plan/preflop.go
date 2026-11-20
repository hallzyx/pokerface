package plan

import (
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/posicion"
	"inteligenciaartesanal/internal/preflop"
)

func preflopLinea(e Entrada) Vista {
	if stackNuestroBB(e.Turno) <= e.Params.StackPushBB {
		return corto(e, stackNuestroBB(e.Turno))
	}
	if esEmpuje(e.Turno) {
		return pagarEmpuje(e)
	}
	return profundo(e)
}

func profundo(e Entrada) Vista {
	tab := preflop.PorDefecto()
	pos := posicionOUTG(e.Turno)
	clave, ok := preflop.Clave(e.Turno.Mano)
	if !ok {
		return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "mano inválida")
	}
	bb := ciega(e.Turno)
	subidas := subidasPreflop(e.Turno)
	tam := tab.Tamanos.AbrirCiegas
	if e.Lectura.SobreFold && e.Lectura.MuestraSuficiente {
		tam = tab.Tamanos.AbrirExplotadoCiegas
	}
	soloValor := e.Lectura.NoSuelta || e.Lectura.PagaGrande

	switch {
	case subidas == 0:
		if pos == "BB" && e.Turno.PorIgualar() == 0 {
			if tab.Tiene("tresbet", "valor", clave) || tab.Tiene("apertura", "UTG", clave) {
				return subir(tam*float64(bb), "ciega grande: subo")
			}
			return vista(LineaPasar, 0, 0, 0, "preflop", "", false, "ciega grande: paso")
		}
		abre := tab.Tiene("apertura", pos, clave)
		roba := e.Lectura.SobreFold && e.Lectura.MuestraSuficiente && preflop.SirveParaRobar(clave, pos)
		// La ciega grande que todavía debe fichas (no pudo pasar) usa el rango de UTG.
		bbDebe := pos == "BB" && tab.Tiene("apertura", "UTG", clave)
		if abre || roba || bbDebe {
			return subir(tam*float64(bb), "abro")
		}
		if e.Turno.PorIgualar() == 0 && e.Turno.Puede(botsdk.Check) {
			return vista(LineaPasar, 0, 0, 0, "preflop", "", false, "paso")
		}
		return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "suelto")

	case subidas == 1:
		if quiereTresBet(pos, clave, soloValor) && e.Turno.PorIgualar() <= 20*bb {
			return subir(float64(e.Turno.Estado.ApuestaActual)*tab.Tamanos.TresBetPor, "3-bet")
		}
		if pagaSubida(pos, clave) && e.Turno.PorIgualar() <= 6*bb {
			return vista(LineaPagar, 0, 0, 0, "preflop", "", false, "pago la subida")
		}
		if tab.Tiene("tresbet", "valor", clave) {
			return subir(float64(e.Turno.Estado.ApuestaActual)*tab.Tamanos.TresBetPor, "3-bet de valor")
		}
		return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "no pago la subida")

	case subidas == 2:
		if tab.Tiene("tresbet", "cuatrobet", clave) {
			// Por encima de 18 ciegas no se va all-in: un 4-bet marginal
			// elimina en la clasificatoria, donde las fichas se arrastran.
			if stackNuestroBB(e.Turno) <= 18 && tab.Tiene("tresbet", "continuar_cuatrobet", clave) {
				return vista(LineaAllIn, 0, 0, 0, "preflop", "", false, "4-bet corto")
			}
			return subir(float64(e.Turno.Estado.ApuestaActual)*tab.Tamanos.CuatroBetPor, "4-bet")
		}
		if tab.Tiene("tresbet", "pagar_tresbet", clave) && e.Turno.PorIgualar() <= 14*bb && !comprometeVida(e.Turno) {
			return vista(LineaPagar, 0, 0, 0, "preflop", "", false, "pago el 3-bet")
		}
		return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "suelto el 3-bet")

	default:
		if tab.Tiene("tresbet", "continuar_cuatrobet", clave) && stackNuestroBB(e.Turno) <= 22 {
			return vista(LineaAllIn, 0, 0, 0, "preflop", "", true, "sigo el 4-bet")
		}
		return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "suelto el 4-bet")
	}
}

// comprometeVida es un call que se lleva más del 15 % del stack que queda.
// En la clasificatoria las fichas se arrastran: pasar de ahí sin una mano
// muy fuerte es el camino a quedar en cero.
func comprometeVida(t botsdk.Turno) bool {
	if t.Yo.Saldo <= 0 {
		return false
	}
	return t.PorIgualar()*100 >= t.Yo.Saldo*15
}

func corto(e Entrada, bbEfectivas float64) Vista {
	tab := preflop.PorDefecto()
	pos := posicionOUTG(e.Turno)
	clave, ok := preflop.Clave(e.Turno.Mano)
	if !ok {
		return vista(LineaRetirarse, 0, 0, 0, "corto", "", false, "mano inválida")
	}
	cubo := preflop.CuboPush(bbEfectivas)
	familia := "empujar"
	if e.Lectura.SueltaAlEmpuje {
		familia = "empujar_medido"
	}
	if subidasPreflop(e.Turno) == 0 {
		if tab.Tiene(familia+"/"+cubo, pos, clave) {
			return vista(LineaAllIn, 0, 0, 0, "corto", "", false, "empujo")
		}
		if e.Turno.PorIgualar() == 0 && e.Turno.Puede(botsdk.Check) {
			return vista(LineaPasar, 0, 0, 0, "corto", "", false, "paso corto")
		}
		return vista(LineaRetirarse, 0, 0, 0, "corto", "", false, "suelto corto")
	}
	if tab.Tiene(familia+"/"+cubo, pos, clave) && (e.Turno.Puede(botsdk.Raise) || e.Turno.Puede(botsdk.Bet) || e.Turno.Puede(botsdk.AllIn)) && e.Turno.PorIgualar() < e.Turno.Yo.Saldo {
		return vista(LineaAllIn, 0, 0, 0, "corto", "", false, "reempujo")
	}
	if tab.Tiene("pagar/"+cubo, pos, clave) || tab.Tiene(familia+"/"+cubo, pos, clave) {
		return vista(LineaPagar, 0, 0, 0, "corto", "", false, "pago el empuje")
	}
	return vista(LineaRetirarse, 0, 0, 0, "corto", "", false, "no pago el empuje")
}

func pagarEmpuje(e Entrada) Vista {
	tab := preflop.PorDefecto()
	pos := posicionOUTG(e.Turno)
	clave, ok := preflop.Clave(e.Turno.Mano)
	if !ok {
		return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "mano inválida")
	}
	bb := float64(ciega(e.Turno))
	cubo := preflop.CuboPush(float64(e.Turno.Estado.ApuestaActual) / bb)
	// Profundo, un all-in rival solo se paga con KK+ o AK suited.
	if stackNuestroBB(e.Turno) > 20 && !tab.Tiene("tresbet", "continuar_cuatrobet", clave) {
		return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "no pago un all-in profundo")
	}
	if tab.Tiene("pagar/"+cubo, pos, clave) {
		return vista(LineaPagar, 0, 0, 0, "preflop", "", true, "pago el all-in")
	}
	return vista(LineaRetirarse, 0, 0, 0, "preflop", "", false, "no pago el all-in")
}

func quiereTresBet(pos, clave string, soloValor bool) bool {
	tab := preflop.PorDefecto()
	if soloValor {
		return tab.Tiene("tresbet", "valor", clave)
	}
	if tab.Tiene("tresbet", pos, clave) {
		return true
	}
	if pos == "BB" && tab.Tiene("defensa", "BB_tresbet", clave) {
		return true
	}
	if pos == "SB" && tab.Tiene("defensa", "SB_tresbet", clave) {
		return true
	}
	return false
}

func pagaSubida(pos, clave string) bool {
	tab := preflop.PorDefecto()
	switch pos {
	case "BB":
		return tab.Tiene("defensa", "BB_pagar", clave)
	case "SB":
		return tab.Tiene("defensa", "SB_pagar", clave)
	default:
		return tab.Tiene("tresbet", "pagar_subida", clave)
	}
}

func posicionOUTG(t botsdk.Turno) string {
	if p := posicion.Heroe(t); p != "" {
		return p
	}
	return "UTG"
}

func subidasPreflop(t botsdk.Turno) int {
	n := 0
	for _, a := range t.Estado.HistorialAcciones {
		if a.Etapa != botsdk.PreFlop {
			continue
		}
		switch a.Accion.Tipo {
		case botsdk.Bet, botsdk.Raise, botsdk.AllIn:
			n++
		}
	}
	return n
}

func esEmpuje(t botsdk.Turno) bool {
	bb := ciega(t)
	if t.Estado.ApuestaActual < 8*bb || t.PorIgualar() < 8*bb {
		return false
	}
	for _, j := range t.Estado.Jugadores {
		if j.ID != t.IDJugador && j.Activo && j.AllIn {
			return true
		}
	}
	return t.PorIgualar() >= t.Yo.Saldo/2 && t.Yo.Saldo > 0
}
