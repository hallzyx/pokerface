// Package equidad estima la parte del pozo que gana la mano repartiendo al
// azar lo que no vemos. El presupuesto es corto a propósito: en una ronda de
// 50 manos el margen de la mesa es de unos 7 segundos por mano para toda la
// mesa, así que lo normal son 100–200 ms.
package equidad

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"github.com/aicspucp/pokerfight-arena-participantes/pkg/poker"
	"inteligenciaartesanal/internal/preflop"
)

const cadaCuantasMirar = 32

// Consulta describe la mano a simular.
type Consulta struct {
	Mano            botsdk.Mano
	Comunitarias    []botsdk.Carta
	Rivales         int
	MaxSimulaciones int
	Presupuesto     time.Duration
	// Rango restringe al primer rival si Rangos está vacío.
	Rango []string
	// Rangos es un rango por rival, en el mismo orden. Vacío significa al azar.
	Rangos [][]string
}

// Resultado es la equidad media. Si Simulaciones es 0, no hubo dato.
type Resultado struct {
	Valor        float64
	Simulaciones int
	Completa     bool
}

// Estimar juega la mano muchas veces hasta agotar el presupuesto o el cupo.
// Siempre mira el contexto: si vence, devuelve lo que haya alcanzado a ver.
func Estimar(ctx context.Context, semilla uint64, c Consulta) Resultado {
	if c.Rivales <= 0 {
		return Resultado{Valor: 1, Simulaciones: 1, Completa: true}
	}
	if c.Presupuesto <= 0 {
		c.Presupuesto = 160 * time.Millisecond
	}
	if c.MaxSimulaciones <= 0 {
		c.MaxSimulaciones = 4000
	}
	restantes := cartasRestantes(c.Mano, c.Comunitarias)
	faltan := 5 - len(c.Comunitarias)
	if faltan < 0 {
		faltan = 0
	}
	if maximo := (len(restantes) - faltan) / 2; c.Rivales > maximo {
		c.Rivales = maximo
	}
	if c.Rivales <= 0 {
		return Resultado{Valor: 1, Simulaciones: 1, Completa: true}
	}

	azar := rand.New(rand.NewPCG(semilla, semilla^0x6A09E667F3BCC909))
	porRival := combosPorRival(restantes, c.Rivales, c.Rangos, c.Rango)

	tablero := make([]botsdk.Carta, 5)
	copy(tablero, c.Comunitarias)
	limite := time.Now().Add(c.Presupuesto)
	var acumulado float64
	n := 0
	fallos := 0

	for n < c.MaxSimulaciones {
		if (n+fallos) > 0 && (n+fallos)%cadaCuantasMirar == 0 {
			if ctx.Err() != nil || time.Now().After(limite) {
				break
			}
		}
		manos, ok := repartir(azar, restantes, faltan, c.Rivales, porRival, tablero, len(c.Comunitarias))
		if !ok {
			fallos++
			if fallos > 64 {
				break
			}
			continue
		}
		acumulado += parteGanada(c.Mano, tablero, manos)
		n++
	}
	if n == 0 {
		return Resultado{}
	}
	return Resultado{
		Valor:        acumulado / float64(n),
		Simulaciones: n,
		Completa:     n >= c.MaxSimulaciones,
	}
}

// repartir arma un showdown sin modificar el mazo original. Si hay rango, el
// primer rival sale de ahí; si no se puede, devuelve false y el bucle prueba
// otra vez.
func combosPorRival(mazo []botsdk.Carta, rivales int, rangos [][]string, primero []string) [][][2]botsdk.Carta {
	out := make([][][2]botsdk.Carta, rivales)
	for i := 0; i < rivales; i++ {
		var claves []string
		if i < len(rangos) {
			claves = rangos[i]
		}
		if len(claves) == 0 && i == 0 {
			claves = primero
		}
		if len(claves) == 0 {
			continue
		}
		out[i] = combosDeRango(mazo, claves)
	}
	return out
}

func repartir(azar *rand.Rand, mazo []botsdk.Carta, faltan, rivales int, porRival [][][2]botsdk.Carta, tablero []botsdk.Carta, ya int) ([]botsdk.Carta, bool) {
	deck := append([]botsdk.Carta(nil), mazo...)
	barajar(azar, deck, len(deck))
	if faltan > len(deck) {
		return nil, false
	}
	copy(tablero[ya:], deck[:faltan])
	usadas := make(map[botsdk.Carta]bool, faltan+2*rivales)
	for i := 0; i < faltan; i++ {
		usadas[deck[i]] = true
	}
	manos := make([]botsdk.Carta, 0, 2*rivales)
	for r := 0; r < rivales; r++ {
		if r < len(porRival) && len(porRival[r]) > 0 {
			var par [2]botsdk.Carta
			if !ponerCombo(azar, porRival[r], usadas, par[:]) {
				return nil, false
			}
			manos = append(manos, par[0], par[1])
			usadas[par[0]], usadas[par[1]] = true, true
			continue
		}
		tomo := 0
		for _, carta := range deck {
			if tomo == 2 {
				break
			}
			if usadas[carta] {
				continue
			}
			manos = append(manos, carta)
			usadas[carta] = true
			tomo++
		}
		if tomo < 2 {
			return nil, false
		}
	}
	if len(manos) < 2*rivales {
		return nil, false
	}
	return manos, true
}

func ponerCombo(azar *rand.Rand, combos [][2]botsdk.Carta, usadas map[botsdk.Carta]bool, dest []botsdk.Carta) bool {
	if len(combos) == 0 {
		return false
	}
	inicio := azar.IntN(len(combos))
	for k := 0; k < len(combos); k++ {
		combo := combos[(inicio+k)%len(combos)]
		if usadas[combo[0]] || usadas[combo[1]] {
			continue
		}
		dest[0], dest[1] = combo[0], combo[1]
		return true
	}
	return false
}

func combosDeRango(restantes []botsdk.Carta, claves []string) [][2]botsdk.Carta {
	quiere := make(map[string]struct{}, len(claves))
	for _, c := range claves {
		quiere[c] = struct{}{}
	}
	var out [][2]botsdk.Carta
	for i := 0; i < len(restantes); i++ {
		for j := i + 1; j < len(restantes); j++ {
			clave, ok := preflop.Clave(botsdk.Mano{restantes[i], restantes[j]})
			if !ok {
				continue
			}
			if _, si := quiere[clave]; si {
				out = append(out, [2]botsdk.Carta{restantes[i], restantes[j]})
			}
		}
	}
	return out
}

func barajar(azar *rand.Rand, cartas []botsdk.Carta, n int) {
	for i := 0; i < n; i++ {
		j := i + azar.IntN(len(cartas)-i)
		cartas[i], cartas[j] = cartas[j], cartas[i]
	}
}

func parteGanada(mia botsdk.Mano, tablero []botsdk.Carta, manosRivales []botsdk.Carta) float64 {
	propio := poker.Mejor(mia, tablero).Puntaje
	empatados := 0
	for i := 0; i+1 < len(manosRivales); i += 2 {
		rival := poker.Mejor(botsdk.Mano{manosRivales[i], manosRivales[i+1]}, tablero).Puntaje
		if rival > propio {
			return 0
		}
		if rival == propio {
			empatados++
		}
	}
	return 1 / float64(empatados+1)
}

func cartasRestantes(mano botsdk.Mano, comunitarias []botsdk.Carta) []botsdk.Carta {
	var vistas [15][4]bool
	for _, c := range mano {
		if c.Rango >= botsdk.Dos && c.Rango <= botsdk.As && c.Palo <= botsdk.Picas {
			vistas[c.Rango][c.Palo] = true
		}
	}
	for _, c := range comunitarias {
		if c.Rango >= botsdk.Dos && c.Rango <= botsdk.As && c.Palo <= botsdk.Picas {
			vistas[c.Rango][c.Palo] = true
		}
	}
	restantes := make([]botsdk.Carta, 0, 52)
	for rango := botsdk.Dos; rango <= botsdk.As; rango++ {
		for palo := botsdk.Treboles; palo <= botsdk.Picas; palo++ {
			if !vistas[rango][palo] {
				restantes = append(restantes, botsdk.Carta{Rango: rango, Palo: palo})
			}
		}
	}
	return restantes
}
