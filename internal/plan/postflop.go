package plan

import (
	"math"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/poker"
	"inteligenciaartesanal/internal/lectura"
	"inteligenciaartesanal/internal/posicion"
	"inteligenciaartesanal/internal/postflop"
	"inteligenciaartesanal/internal/talla"
)

func postflopLinea(e Entrada) Vista {
	t := e.Turno
	clase := postflop.Clasificar(t.Mano, t.Estado.Comunitarias)
	rivales := len(posicion.Rivales(t))
	if rivales < 1 {
		rivales = 1
	}
	porVenir := 5 - len(t.Estado.Comunitarias)
	eq := clase.Prior(rivales, porVenir)
	if e.TieneEquidad {
		eq = e.Equidad
	}
	precio := precioDePagar(t)
	bb := ciega(t)
	agresor := agresorPreflop(t) == t.IDJugador
	freq := frecuenciaFarol(e, rivales, agresor)
	cerca := t.PorIgualar() > 0 && math.Abs(eq-precio) <= 0.05 && t.Estado.Pozo >= 8*bb

	valor := clase.Fuerza >= postflop.FuerzaMonstruo && eq >= 0.52 ||
		clase.Fuerza >= postflop.FuerzaFuerte && eq >= 0.56 ||
		clase.Detalle == "top_bueno" && eq >= 0.50
	// Contra una estación (novato paga cualquier top pair) se apuesta más
	// fino: segunda pareja y top pair débil también son valor.
	fino := !valor && (clase.Detalle == "top_debil" || clase.Detalle == "medio" || clase.Detalle == "overpair") &&
		eq >= 0.42 && (e.Lectura.Estacion || e.Lectura.PagaGrande || (!e.Lectura.Estacion && (clase.Textura == "seca" || clase.Textura == "emparejada") && eq >= 0.46))

	if t.PorIgualar() == 0 {
		return sinApuesta(e, clase, eq, valor, fino, freq, agresor, cerca)
	}
	return contraApuesta(e, clase, eq, precio, valor, fino, cerca)
}

func sinApuesta(e Entrada, clase postflop.Clase, eq float64, valor, fino bool, freq float64, agresor, cerca bool) Vista {
	t := e.Turno
	puede := t.Puede(botsdk.Bet) || t.Puede(botsdk.Raise)
	spr := float64(t.Yo.Saldo) / float64(max(t.Estado.Pozo, 1))
	// Solo un trío o mejor va all-in. Una pareja, aunque sea alta, no.
	if clase.Categoria >= poker.Trio && eq >= 0.80 && spr <= 1.2 && (t.Puede(botsdk.AllIn) || puede) {
		return vista(LineaAllIn, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "all-in de valor")
	}
	if valor && puede {
		frac := fraccionTope(t, talla.Elegir(clase.Textura, false, e.Lectura), clase, eq)
		if frac > 0 {
			return vista(LineaValor, frac, 0, eq, clase.Detalle, clase.Textura, cerca, "apuesto valor")
		}
	}
	if fino && puede {
		frac := fraccionTope(t, talla.Elegir(clase.Textura, false, e.Lectura), clase, eq)
		if frac > 0 {
			return vista(LineaValorFino, frac, 0, eq, clase.Detalle, clase.Textura, cerca, "valor fino")
		}
	}
	rivales := len(posicion.Rivales(t))
	if clase.Outs >= 8 && puede && rivales <= 2 && !e.Lectura.PagaGrande && !huboDefensaPostflop(t) && (agresor || e.Lectura.SobreFold) {
		frac := fraccionTope(t, fraccionDeFarol(t, clase.Textura, e.Lectura), clase, eq)
		if frac > 0 {
			return vista(LineaSemifarol, frac, 0, eq, clase.Detalle, clase.Textura, cerca, "semi-farol")
		}
	}
	if clase.Fuerza <= postflop.FuerzaDebil && puede && freq > 0 && mezclar(t, freq) {
		frac := fraccionTope(t, fraccionDeFarol(t, clase.Textura, e.Lectura), clase, eq)
		if frac > 0 {
			return vista(LineaFarol, frac, 0, eq, clase.Detalle, clase.Textura, cerca, "farol")
		}
	}
	return vista(LineaPasar, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "paso")
}

func contraApuesta(e Entrada, clase postflop.Clase, eq, precio float64, valor, fino, cerca bool) Vista {
	t := e.Turno
	relacion := relacionDeLaApuesta(t)
	// Más del 15 % del stack solo se paga con trío o mejor, o con AA/KK
	// que siguen por delante del rango. Una pareja floja no. Multiway, el
	// doble par tampoco.
	if comprometeVida(t) && !aguantaStack(e, clase, eq, precio) {
		return vista(LineaRetirarse, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "no arriesgo el torneo")
	}
	if relacion >= 0.60 && clase.Fuerza < postflop.FuerzaMonstruo {
		if clase.Outs >= 8 && relacion < 1.05 && eq+0.01 >= precio && !comprometeVida(t) {
			return vista(LineaPagar, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "pago el proyecto")
		}
		return vista(LineaRetirarse, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "suelto la apuesta grande")
	}
	if clase.Fuerza < postflop.FuerzaFuerte && relacion >= 0.40 && clase.Outs < 8 && !e.Lectura.Estacion {
		return vista(LineaRetirarse, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "apuesta media, mano floja")
	}
	if clase.Categoria >= poker.Trio && eq >= 0.62 && eq >= precio+0.18 && (t.Puede(botsdk.Raise) || t.Puede(botsdk.Bet)) {
		if float64(t.Yo.Saldo) <= float64(t.Estado.Pozo)+float64(ciega(t)) && eq >= 0.70 {
			return vista(LineaAllIn, 0, 0, eq, clase.Detalle, clase.Textura, true, "resubo all-in")
		}
		frac := fraccionTope(t, talla.Elegir(clase.Textura, false, e.Lectura), clase, eq)
		if frac > 0 {
			return vista(LineaValor, frac, 0, eq, clase.Detalle, clase.Textura, cerca, "resubo el trío")
		}
	}
	margen := 0.08
	if e.Lectura.Estacion {
		margen = 0.04
	}
	if eq >= precio+margen && (valor || fino || clase.Outs >= 8 || eq >= precio+0.14) {
		return vista(LineaPagar, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "pago")
	}
	return vista(LineaRetirarse, 0, 0, eq, clase.Detalle, clase.Textura, cerca, "no me da el precio")
}

// fraccionTope recorta una apuesta que se llevaría más del 18 % del stack
// si la mano no aguanta ese compromiso. Cero significa que conviene pasar.
func fraccionTope(t botsdk.Turno, frac float64, clase postflop.Clase, eq float64) float64 {
	if frac <= 0 || t.Yo.Saldo <= 0 {
		return 0
	}
	if clase.Categoria >= poker.Trio && eq >= 0.70 {
		return frac
	}
	if clase.Detalle == "overpair" && parejaAlta(t.Mano) && eq >= 0.68 {
		return frac
	}
	maxFrac := 0.18 * float64(t.Yo.Saldo) / float64(max(t.Estado.Pozo, 1))
	if maxFrac < 0.15 {
		return 0
	}
	if frac > maxFrac {
		return maxFrac
	}
	return frac
}

// aguantaStack es la mano con la que se puede pagar más del 15 % del stack.
func aguantaStack(e Entrada, clase postflop.Clase, eq, precio float64) bool {
	if clase.Categoria >= poker.Trio && eq >= precio+0.12 && eq >= 0.60 {
		return true
	}
	if clase.Detalle == "overpair" && parejaAlta(e.Turno.Mano) && eq >= 0.68 {
		return true
	}
	rivales := len(posicion.Rivales(e.Turno))
	if rivales <= 1 && clase.Categoria == poker.DoblePar && eq >= precio+0.15 {
		return true
	}
	return false
}

func parejaAlta(m botsdk.Mano) bool {
	return m[0].Rango == m[1].Rango && m[0].Rango >= botsdk.Rey
}

// relacionDeLaApuesta es la apuesta a pagar dividida por el pozo de antes de
// esa apuesta. 1 es una apuesta del tamaño del pozo.
func relacionDeLaApuesta(t botsdk.Turno) float64 {
	pagar := t.PorIgualar()
	if pagar <= 0 {
		return 0
	}
	antes := t.Estado.Pozo - pagar
	if antes < 1 {
		antes = 1
	}
	return float64(pagar) / float64(antes)
}

// fraccionDeFarol sube el farol hasta unas 4 ciegas grandes cuando el pozo
// es chico. Conservador y base pagan por debajo de eso y se van por encima.
func fraccionDeFarol(t botsdk.Turno, textura string, lec lectura.Lectura) float64 {
	frac := talla.Elegir(textura, true, lec)
	pozo := t.Estado.Pozo
	if pozo < 1 {
		return frac
	}
	minimo := float64(4*ciega(t)) / float64(pozo)
	if minimo <= frac {
		return frac
	}
	tope := 1.5
	if lec.SobreFold || lec.SoloPagaChico {
		tope = 3
	}
	if minimo > tope {
		return frac
	}
	return minimo
}

func frecuenciaFarol(e Entrada, rivales int, agresor bool) float64 {
	// Novato y cualquier estación pagan el top pair: no se les farolea.
	// Base y conservador se van: el c-bet grande se mantiene.
	if e.Lectura.Estacion || e.Lectura.PagaGrande || huboDefensaPostflop(e.Turno) || rivales >= 3 {
		return 0
	}
	if (e.Lectura.Carpeta || e.Lectura.SobreFold || e.Lectura.SoloPagaChico) && agresor && rivales <= 2 {
		return e.Params.FarolSobreFold
	}
	if agresor && !e.Lectura.MuestraSuficiente && rivales <= 2 {
		return e.Params.FarolSinMuestra
	}
	if agresor && rivales <= 1 {
		return e.Params.FarolConMuestra
	}
	return 0
}

func huboDefensaPostflop(t botsdk.Turno) bool {
	for _, a := range t.Estado.HistorialAcciones {
		if a.Etapa == botsdk.PreFlop || a.IDJugador == t.IDJugador {
			continue
		}
		switch a.Accion.Tipo {
		case botsdk.Call, botsdk.Raise, botsdk.Bet, botsdk.AllIn:
			return true
		}
	}
	return false
}

func agresorPreflop(t botsdk.Turno) string {
	id := ""
	for _, a := range t.Estado.HistorialAcciones {
		if a.Etapa != botsdk.PreFlop {
			continue
		}
		switch a.Accion.Tipo {
		case botsdk.Bet, botsdk.Raise, botsdk.AllIn:
			id = a.IDJugador
		}
	}
	return id
}

func precioDePagar(t botsdk.Turno) float64 {
	costo := t.PorIgualar()
	if t.Yo.Saldo < costo {
		costo = t.Yo.Saldo
	}
	if costo <= 0 {
		return 0
	}
	noIgualable := t.PorIgualar() - costo
	pozoFinal := t.Estado.Pozo - noIgualable + costo
	if pozoFinal <= 0 {
		return 1
	}
	return float64(costo) / float64(pozoFinal)
}
