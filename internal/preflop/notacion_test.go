package preflop

import "testing"

func TestExpandirNotacion(t *testing.T) {
	tiene := func(expr, clave string, querer bool) {
		t.Helper()
		manos, err := Expandir(expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		ok := false
		for _, m := range manos {
			if m == clave {
				ok = true
			}
		}
		if ok != querer {
			t.Fatalf("%s contiene %s = %v, quiero %v (%v)", expr, clave, ok, querer, manos)
		}
	}
	tiene("77+", "TT", true)
	tiene("77+", "66", false)
	tiene("77+", "AA", true)
	tiene("ATs+", "AKs", true)
	tiene("ATs+", "A9s", false)
	tiene("AJo+", "AKo", true)
	tiene("AJo+", "ATo", false)
	tiene("76s-T9s", "98s", true)
	tiene("76s-T9s", "65s", false)
	tiene("JJ", "JJ", true)
	tiene("JJ", "TT", false)
	tiene("KQo+", "KQo", true)
	tiene("KQo+", "AKo", false)
}

func TestTablasDeApertura(t *testing.T) {
	tab := PorDefecto()
	if !tab.Tiene("apertura", "UTG", "AA") {
		t.Fatal("AA tiene que abrir desde UTG")
	}
	if !tab.Tiene("apertura", "UTG", "AKs") {
		t.Fatal("AKs tiene que abrir desde UTG")
	}
	if tab.Tiene("apertura", "UTG", "72o") {
		t.Fatal("72o no abre desde UTG")
	}
	if !tab.Tiene("empujar/10", "UTG", "AA") {
		t.Fatal("falta AA en el push de 10bb")
	}
	if tab.Tiene("empujar/10", "UTG", "72o") {
		t.Fatal("72o no se empuja desde UTG")
	}
	if tab.Tamanos.Version == "" {
		t.Fatal("las tablas no traen versión")
	}
}
