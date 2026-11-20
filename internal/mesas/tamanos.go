// Package mesas reparte una clasificatoria en mesas de 5 y, si el resto no
// entra, de 6. Con 9 inscritos no hay forma de usar solo 5 y 6: queda una
// mesa de 5 y otra de 4, que es el reparto más cercano que cabe en la mesa
// del kit (máximo 8).
package mesas

// Tamanos devuelve cuántos jugadores se sientan en cada mesa.
func Tamanos(n int) []int {
	if n < 2 {
		return nil
	}
	if n <= 6 {
		return []int{n}
	}
	for k := 1; k <= n/5; k++ {
		if 5*k <= n && n <= 6*k {
			return repartir(n, k)
		}
	}
	k := (n + 5) / 6
	if k < 1 {
		k = 1
	}
	if n/k > 8 {
		k = (n + 7) / 8
	}
	return repartir(n, k)
}

func repartir(n, k int) []int {
	if k < 1 {
		k = 1
	}
	base := n / k
	extra := n % k
	out := make([]int, k)
	for i := 0; i < k; i++ {
		out[i] = base
		if i < extra {
			out[i]++
		}
	}
	return out
}
