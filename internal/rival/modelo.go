// Package rival lleva las estadísticas vivas de cada jugador. El plan nunca
// lo recibe: solo una lectura.Lectura, que es una copia.
package rival

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/aicspucp/pokerfight-arena-participantes/pkg/botsdk"
	"inteligenciaartesanal/internal/lectura"
	"inteligenciaartesanal/internal/preflop"
	"inteligenciaartesanal/internal/rango"
)

//go:embed datos/umbrales.json
var umbralJSON []byte

// Umbrales salen de un JSON para poder ajustarlos sin tocar el modelo.
type Umbrales struct {
	ManosEstacion       int     `json:"manos_estacion"`
	VpipEstacion        float64 `json:"vpip_estacion"`
	AfEstacion          float64 `json:"af_estacion"`
	ManosFold           int     `json:"manos_fold"`
	FoldGrande          float64 `json:"fold_grande"`
	FoldGrandePaga      float64 `json:"fold_grande_paga"`
	CiegasApuestaGrande int64   `json:"ciegas_apuesta_grande"`
	MuestrasTresbet     int     `json:"muestras_tresbet"`
	FoldTresbetBajo     float64 `json:"fold_tresbet_bajo"`
	ManosRangoTight     int     `json:"manos_rango_tight"`
	VpipTight           float64 `json:"vpip_tight"`
}

// Modelo acumula VPIP, PFR, agresión, fold al c-bet y manos de showdown.
// También observa manos en las que nosotros no jugamos: le llega cada estado.
type Modelo struct {
	mu          sync.Mutex
	u           Umbrales
	yo          string
	perfiles    map[string]*perfil
	mano        string
	vistas      int
	vpipHecho   map[string]bool
	agresor     string
	subidas     int
	cbet        bool
	cbetGrande  bool
	empujeVivo  bool
	calleEmpuje botsdk.Etapa
	quienEmpuje string
}

type perfil struct {
	manos            int
	vpip             int
	pfr              int
	agresivas        int
	pasivas          int
	cbetChicoEncara  int
	cbetChicoFold    int
	cbetGrandeEncara int
	cbetGrandeFold   int
	openEncara       int
	openFold         int
	tresbetEncara    int
	tresbetFold      int
	mostradas        []string
	empujeEncara     int
	empujeFold       int
}

// Nuevo carga los umbrales embebidos.
func Nuevo() *Modelo {
	var u Umbrales
	if err := json.Unmarshal(umbralJSON, &u); err != nil {
		panic(err)
	}
	if u.CiegasApuestaGrande <= 0 {
		u.CiegasApuestaGrande = 4
	}
	return &Modelo{u: u, perfiles: map[string]*perfil{}}
}

// Umbrales devuelve una copia de la configuración.
func (m *Modelo) Umbrales() Umbrales {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.u
}

// FijarYo recuerda nuestro id para no modelarnos a nosotros mismos.
func (m *Modelo) FijarYo(id string) {
	if id == "" {
		return
	}
	m.mu.Lock()
	m.yo = id
	m.mu.Unlock()
}

// ObservarEstado aplica las acciones nuevas del historial. Es idempotente:
// un mismo estado repetido no cuenta dos veces.
func (m *Modelo) ObservarEstado(estado botsdk.EstadoPublico, yo string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if yo != "" {
		m.yo = yo
	}
	if estado.IDMano == "" {
		return
	}
	if estado.IDMano != m.mano {
		m.mano = estado.IDMano
		m.vistas = 0
		m.vpipHecho = map[string]bool{}
		m.agresor = ""
		m.subidas = 0
		m.cbet = false
		m.cbetGrande = false
		m.empujeVivo = false
		m.quienEmpuje = ""
		for _, j := range estado.Jugadores {
			if !j.EnMano || !j.EnTorneo || j.ID == m.yo || j.ID == "" {
				continue
			}
			m.perfil(j.ID).manos++
		}
	}
	hist := estado.HistorialAcciones
	if m.vistas > len(hist) {
		m.vistas = 0
	}
	bb := estado.CiegaGrande
	if bb <= 0 {
		bb = 100
	}
	for _, a := range hist[m.vistas:] {
		m.aplicar(a, bb)
	}
	m.vistas = len(hist)
}

// ObservarFin anota las cartas mostradas. Ignora los repartos de fichas:
// ganar o perder un pozo no cambia los parámetros.
func (m *Modelo) ObservarFin(res botsdk.ResultadoMano, yo string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if yo != "" {
		m.yo = yo
	}
	for id, mano := range res.Mostradas {
		if id == m.yo || id == "" {
			continue
		}
		clave, ok := preflop.Clave(mano)
		if !ok {
			continue
		}
		p := m.perfil(id)
		p.mostradas = append(p.mostradas, clave)
		if len(p.mostradas) > 16 {
			p.mostradas = append([]string(nil), p.mostradas[len(p.mostradas)-16:]...)
		}
	}
}

// Lectura resume a los rivales indicados y arma un rango por cada uno a
// partir del estado de esta mano. Es un valor.
func (m *Modelo) Lectura(estado botsdk.EstadoPublico, ids []string) lectura.Lectura {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(ids) == 0 {
		return lectura.Lectura{}
	}
	var l lectura.Lectura
	l.Manos = 1 << 30
	estaciones := 0
	sobre := 0
	paganGrande := 0
	noSuelta := 0
	var sumVPIP, sumPFR, sumAF, sumFG, sumFC, sumFO, sumFT float64
	carpetas := 0
	sueltaEmpuje := 0
	conEmpuje := 0
	l.Rangos = make([][]string, 0, len(ids))
	for _, id := range ids {
		p := m.perfiles[id]
		if p == nil {
			l.Manos = 0
			l.Rangos = append(l.Rangos, m.rangoDe(nil, id, estado))
			continue
		}
		if p.manos < l.Manos {
			l.Manos = p.manos
		}
		vpip := tasa(p.vpip, p.manos)
		sumVPIP += vpip
		sumPFR += tasa(p.pfr, p.manos)
		sumAF += p.af()
		fg := tasa(p.cbetGrandeFold, p.cbetGrandeEncara)
		sumFG += fg
		sumFC += tasa(p.cbetChicoFold, p.cbetChicoEncara)
		sumFO += tasa(p.openFold, p.openEncara)
		sumFT += tasa(p.tresbetFold, p.tresbetEncara)
		fe := tasa(p.empujeFold, p.empujeEncara)
		if p.manos >= m.u.ManosEstacion && vpip >= m.u.VpipEstacion && p.af() < m.u.AfEstacion {
			estaciones++
		}
		if p.cbetGrandeEncara >= m.u.ManosFold && fg >= 0.70 {
			carpetas++
		}
		if p.cbetGrandeEncara >= m.u.ManosFold && fg < 0.55 {
			estaciones++
		}
		if p.cbetGrandeEncara >= m.u.ManosFold && fg >= m.u.FoldGrande {
			sobre++
		}
		if p.empujeEncara >= 6 {
			conEmpuje++
			if fe >= 0.62 {
				sueltaEmpuje++
			}
		}
		l.Rangos = append(l.Rangos, m.rangoDe(p, id, estado))
		if p.cbetGrandeEncara >= m.u.ManosFold && fg < m.u.FoldGrandePaga && p.manos >= m.u.ManosEstacion && vpip >= m.u.VpipEstacion {
			paganGrande++
		}
		if p.tresbetEncara >= m.u.MuestrasTresbet && tasa(p.tresbetFold, p.tresbetEncara) < m.u.FoldTresbetBajo {
			noSuelta++
		}
	}
	n := float64(len(ids))
	l.VPIP = sumVPIP / n
	l.PFR = sumPFR / n
	l.FactorAgresion = sumAF / n
	l.FoldGrande = sumFG / n
	l.FoldChico = sumFC / n
	l.FoldAlOpen = sumFO / n
	l.FoldAlTresBet = sumFT / n
	l.Estacion = estaciones > 0
	l.Carpeta = carpetas == len(ids) && carpetas > 0 && !l.Estacion
	l.SobreFold = sobre == len(ids) && sobre > 0
	l.PagaGrande = paganGrande > 0 && !l.SobreFold
	l.SoloPagaChico = l.Estacion && l.SobreFold
	l.NoSuelta = noSuelta > 0
	l.SueltaAlEmpuje = conEmpuje == len(ids) && sueltaEmpuje == len(ids) && len(ids) > 0
	l.MuestraSuficiente = l.Manos >= m.u.ManosFold && l.Manos < 1<<30
	l.Rango = rangoDelAgresor(estado, ids, l.Rangos)
	if l.Manos >= 1<<30 {
		l.Manos = 0
	}
	return l
}

// Empuje suma, entre todos los rivales vistos, cuántas veces enfrentaron un
// all-in y cuántas se retiraron.
func (m *Modelo) Empuje() (encara, seFue int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.perfiles {
		encara += p.empujeEncara
		seFue += p.empujeFold
	}
	return encara, seFue
}

// Firma resume los contadores. Sirve para comprobar que un reparto de fichas
// no mueve el modelo.
func (m *Modelo) Firma() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.perfiles))
	for id := range m.perfiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b string
	for _, id := range ids {
		p := m.perfiles[id]
		b += fmt.Sprintf("%s:%d,%d,%d,%d,%d,%d;", id, p.manos, p.vpip, p.pfr, p.cbetGrandeEncara, p.cbetGrandeFold, len(p.mostradas))
	}
	return b
}

func (m *Modelo) perfil(id string) *perfil {
	p, ok := m.perfiles[id]
	if !ok {
		p = &perfil{}
		m.perfiles[id] = p
	}
	return p
}

func (m *Modelo) aplicar(a botsdk.AccionRegistrada, bb int64) {
	tipo := a.Accion.Tipo
	esSubida := tipo == botsdk.Bet || tipo == botsdk.Raise || tipo == botsdk.AllIn
	yaHabiaCbet := m.cbet && a.Etapa == botsdk.Flop
	if a.Etapa != m.calleEmpuje {
		m.empujeVivo = false
	}
	enfrentabaEmpuje := m.empujeVivo && a.IDJugador != m.quienEmpuje && a.IDJugador != m.yo && a.IDJugador != ""

	if a.Etapa == botsdk.PreFlop && esSubida {
		m.subidas++
		if m.subidas == 1 {
			m.agresor = a.IDJugador
		}
	}
	if a.Etapa != botsdk.Flop {
		m.cbet = false
	} else if !m.cbet && esSubida && m.agresor != "" && a.IDJugador == m.agresor {
		m.cbet = true
		m.cbetGrande = a.Accion.Monto >= m.u.CiegasApuestaGrande*bb
	}

	if enfrentabaEmpuje && (tipo == botsdk.Fold || tipo == botsdk.Call || tipo == botsdk.Raise || tipo == botsdk.AllIn) {
		p := m.perfil(a.IDJugador)
		p.empujeEncara++
		if tipo == botsdk.Fold {
			p.empujeFold++
		}
	}
	if esSubida && (tipo == botsdk.AllIn || a.Accion.Monto >= 8*bb) {
		m.empujeVivo = true
		m.calleEmpuje = a.Etapa
		m.quienEmpuje = a.IDJugador
	}

	if a.IDJugador == "" || a.IDJugador == m.yo {
		return
	}
	p := m.perfil(a.IDJugador)
	switch a.Etapa {
	case botsdk.PreFlop:
		switch {
		case esSubida || tipo == botsdk.Call:
			if !m.vpipHecho[a.IDJugador] {
				m.vpipHecho[a.IDJugador] = true
				p.vpip++
			}
			if esSubida {
				p.pfr++
				p.agresivas++
			} else {
				p.pasivas++
				if m.subidas >= 2 {
					p.tresbetEncara++
				} else if m.subidas == 1 {
					p.openEncara++
				}
			}
		case tipo == botsdk.Fold && m.subidas >= 2:
			p.tresbetEncara++
			p.tresbetFold++
		case tipo == botsdk.Fold && m.subidas == 1:
			p.openEncara++
			p.openFold++
		}
	default:
		switch {
		case esSubida:
			p.agresivas++
		case tipo == botsdk.Call:
			p.pasivas++
		}
		if yaHabiaCbet && a.IDJugador != m.agresor {
			if m.cbetGrande {
				p.cbetGrandeEncara++
				if tipo == botsdk.Fold {
					p.cbetGrandeFold++
				}
			} else {
				p.cbetChicoEncara++
				if tipo == botsdk.Fold {
					p.cbetChicoFold++
				}
			}
		}
	}
}

func (p *perfil) af() float64 {
	if p.pasivas == 0 {
		if p.agresivas == 0 {
			return 0
		}
		return float64(p.agresivas)
	}
	return float64(p.agresivas) / float64(p.pasivas)
}

func (m *Modelo) rangoDe(p *perfil, id string, estado botsdk.EstadoPublico) []string {
	if p == nil {
		p = &perfil{}
	}
	subio, pago := false, false
	apuesta := 0.0
	paso := true
	for _, a := range estado.HistorialAcciones {
		if a.IDJugador != id {
			continue
		}
		tipo := a.Accion.Tipo
		if a.Etapa == botsdk.PreFlop && (tipo == botsdk.Bet || tipo == botsdk.Raise || tipo == botsdk.AllIn) {
			subio = true
			paso = false
		}
		if a.Etapa == botsdk.PreFlop && tipo == botsdk.Call {
			pago = true
			paso = false
		}
		if a.Etapa != botsdk.PreFlop && (tipo == botsdk.Bet || tipo == botsdk.Raise || tipo == botsdk.AllIn) {
			paso = false
			bb := float64(estado.CiegaGrande)
			if bb < 1 {
				bb = 100
			}
			if float64(a.Accion.Monto)/bb >= 8 || tipo == botsdk.AllIn {
				apuesta = 1
			} else if float64(a.Accion.Monto)/bb >= 4 && apuesta < 0.7 {
				apuesta = 0.7
			} else if apuesta < 0.4 {
				apuesta = 0.4
			}
		}
	}
	// La apuesta que estamos enfrentando ahora pisa el tamaño histórico.
	if estado.ApuestaActual > 0 {
		for i := len(estado.HistorialAcciones) - 1; i >= 0; i-- {
			a := estado.HistorialAcciones[i]
			if a.Etapa != estado.Etapa || a.IDJugador != id {
				continue
			}
			if a.Accion.Tipo == botsdk.Bet || a.Accion.Tipo == botsdk.Raise || a.Accion.Tipo == botsdk.AllIn {
				antes := estado.Pozo - estado.ApuestaActual
				if antes < 1 {
					antes = 1
				}
				apuesta = float64(estado.ApuestaActual) / float64(antes)
				paso = false
				break
			}
		}
	}
	sem := rango.Semilla(tasa(p.vpip, p.manos), tasa(p.pfr, p.manos), p.manos, subio, pago && !subio)
	sem = append(sem, p.mostradas...)
	nivel := rango.Cualquiera
	switch {
	case apuesta >= 0.65:
		nivel = rango.Valor
	case apuesta >= 0.30:
		nivel = rango.Medio
	case paso && p.cbetGrandeEncara >= m.u.ManosFold && tasa(p.cbetGrandeFold, p.cbetGrandeEncara) >= 0.70 && len(estado.Comunitarias) >= 3:
		nivel = rango.Debil
	}
	return rango.Achicar(sem, estado.Comunitarias, nivel)
}

func rangoDelAgresor(estado botsdk.EstadoPublico, ids []string, rangos [][]string) []string {
	agresor := ""
	for i := len(estado.HistorialAcciones) - 1; i >= 0; i-- {
		a := estado.HistorialAcciones[i]
		if a.Etapa != estado.Etapa {
			break
		}
		switch a.Accion.Tipo {
		case botsdk.Bet, botsdk.Raise, botsdk.AllIn:
			agresor = a.IDJugador
		}
		if agresor != "" {
			break
		}
	}
	if agresor != "" {
		for i, id := range ids {
			if id == agresor && i < len(rangos) {
				return append([]string(nil), rangos[i]...)
			}
		}
	}
	var mejor []string
	for _, r := range rangos {
		if len(r) == 0 {
			continue
		}
		if mejor == nil || len(r) < len(mejor) {
			mejor = r
		}
	}
	return append([]string(nil), mejor...)
}

func tasa(num, den int) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) / float64(den)
}
