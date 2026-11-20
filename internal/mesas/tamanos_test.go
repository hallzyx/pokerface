package mesas

import "testing"

func TestTamanosDeCincoYSeis(t *testing.T) {
	casos := []struct {
		n    int
		want []int
	}{
		{5, []int{5}},
		{6, []int{6}},
		{10, []int{5, 5}},
		{11, []int{6, 5}},
		{12, []int{6, 6}},
		{16, []int{6, 5, 5}},
	}
	for _, c := range casos {
		got := Tamanos(c.n)
		if !igual(got, c.want) && !igual(got, invertir(c.want)) {
			t.Fatalf("n=%d: got %v want %v", c.n, got, c.want)
		}
		suma := 0
		for _, talla := range got {
			if talla < 5 || talla > 6 {
				t.Fatalf("n=%d mesa fuera de 5-6: %v", c.n, got)
			}
			suma += talla
		}
		if suma != c.n {
			t.Fatalf("n=%d suma %d", c.n, suma)
		}
	}
}

func TestNueveNoCabeEnCincoYSeis(t *testing.T) {
	got := Tamanos(9)
	if len(got) != 2 {
		t.Fatalf("9 jugadores: %v", got)
	}
	suma := got[0] + got[1]
	if suma != 9 || got[0] < 4 || got[1] < 4 || got[0] > 5 || got[1] > 5 {
		t.Fatalf("se esperaba 5 y 4, got %v", got)
	}
}

func igual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func invertir(a []int) []int {
	out := make([]int, len(a))
	for i := range a {
		out[len(a)-1-i] = a[i]
	}
	return out
}
