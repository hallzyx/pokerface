package bot

import (
	"os/exec"
	"strings"
	"testing"
)

func TestFronteraDePaquetes(t *testing.T) {
	casos := []struct {
		pkg       string
		prohibido []string
	}{
		{"inteligenciaartesanal/internal/teatro", []string{"internal/rival", "internal/bot", "internal/equidad", "internal/paranoia"}},
		{"inteligenciaartesanal/internal/plan", []string{"internal/teatro", "internal/rival", "internal/bot", "internal/paranoia"}},
		{"inteligenciaartesanal/internal/paranoia", []string{"internal/rival", "internal/bot", "internal/teatro"}},
		{"inteligenciaartesanal/internal/rival", []string{"internal/teatro", "internal/bot", "internal/plan", "internal/paranoia"}},
	}
	for _, c := range casos {
		imps := imports(t, c.pkg)
		for _, malo := range c.prohibido {
			if strings.Contains(imps, malo) {
				t.Errorf("%s importa %s\n%s", c.pkg, malo, imps)
			}
		}
	}
}

func imports(t *testing.T, pkg string) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", "{{join .Imports \"\\n\"}}", pkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list %s: %v\n%s", pkg, err, out)
	}
	return string(out)
}
