# Compila el kit (sin modificarlo) y el bot de Inteligencia artesanal.
export PATH := $(HOME)/.local/go/bin:$(PATH)
export GOTOOLCHAIN := local

BIN := bin
N ?= 3
STATICCHECK := $(shell go env GOPATH)/bin/staticcheck

.PHONY: build kit bot fases test vet fmt lint verificar torneo clasificatoria reporte limpiar

build: kit bot ## Compila la arena, los bots del kit y el nuestro

kit: ## Compila el kit anclado en ./kit hacia ./bin
	cd kit && go build -o ../$(BIN)/ ./cmd/... ./bots/...

bot: ## Compila nuestro bot
	go build -o $(BIN)/artesanal ./cmd/artesanal

fases: ## Mesa con fichas por jugador, compilada en un árbol temporal del kit
	rm -rf build/kit-fases
	mkdir -p build/kit-fases
	tar -C kit --exclude .git -cf - . | tar -C build/kit-fases -xf -
	mkdir -p build/kit-fases/cmd/fases
	cp harness/fases/main.go build/kit-fases/cmd/fases/main.go
	cd build/kit-fases && go build -o ../../$(BIN)/fases ./cmd/fases

test: ## Tests del bot
	go test ./...

vet:
	go vet ./...

fmt:
	@archivos=$$(find . -name '*.go' -not -path './kit/*' -not -path './build/*'); \
	if [ -n "$$(gofmt -l $$archivos)" ]; then \
		echo "gofmt pendiente:"; gofmt -l $$archivos; exit 1; \
	fi

lint: ## staticcheck sobre nuestro módulo
	@test -x "$(STATICCHECK)" || go install honnef.co/go/tools/cmd/staticcheck@latest
	"$(STATICCHECK)" ./...

verificar: fmt vet lint test ## Lo que corre en CI
	python3 scripts/reporte.py --probar

reporte: ## Resume un log del bot. Uso: make reporte LOG=archivo
	@test -n "$(LOG)" || { echo "uso: make reporte LOG=archivo"; exit 1; }
	python3 scripts/reporte.py "$(LOG)"

torneo: build ## N torneos de 50 manos contra los 8 bots del kit
	scripts/torneo-local.py $(N)

clasificatoria: build fases ## Clasificatoria de 10 rondas, varias semillas
	scripts/clasificatoria.py

limpiar:
	rm -rf $(BIN) resultados
