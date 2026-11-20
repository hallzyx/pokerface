# Inteligencia artesanal

Bot de Texas Hold'em no-limit para PokerFight 2026 (AICS PUCP). El kit oficial está en `kit/` como submódulo fijado y no se modifica.

## Enfoque

`Decidir` es un orquestador que llama módulos chicos: tablas preflop, clasificación del board, equidad Monte Carlo, modelo del rival y tamaños. El plan (fuerza, equidad, línea) es un valor de solo lectura. El teatro elige la acción que se envía y no puede reescribir esas creencias. El modelo se actualiza con cada `estado`, también en manos ajenas: VPIP, PFR, fold al c-bet y fold al all-in. El rango de cada rival se siembra con VPIP/PFR y se achica con lo que hace en la mano; una apuesta grande se trata como valor. Más del 15 % del stack solo se paga con trío y equidad, o con AA/KK que sigan por delante; el all-in pide trío. La segunda opinión corre solo en pozos grandes o decisiones cerradas, únicamente puede endurecer la línea y no bloquea: un vigilante responde antes de 4 s. Si el socket se cierra, reintenta con el mismo token (0,5 s, el doble hasta 8 s) durante `-ventana-reconexion` (20 min). Si no vuelve a entrar, escribe el cierre con las últimas fichas y sale con código 0. El silencio entre rondas, con el socket abierto, no cuenta. Ctrl+C y un rechazo de credencial o de versión salen enseguida.

## Cómo correrlo

```bash
git submodule update --init --recursive
make build
./bin/artesanal -addr HOST:PUERTO -token "$POKERFIGHT_TOKEN"
```

El token se lee de `-token` o de `POKERFIGHT_TOKEN`. No está en el repositorio ni se escribe en el log. En una mesa local cualquier texto sirve. Cada mano deja en stderr una línea `mano=…` (pozo, ganadores, neto, fichas, showdown y cartas); al salir, un `resumen`. `make reporte LOG=archivo` lo pasa a español.

## Resultados

Clasificatoria simulada contra los ocho bots del kit: 6 semillas, 10 rondas, 50 manos o 6 minutos, ciegas 50/100, las fichas se arrastran y quien llega a 0 no vuelve a sentarse. Pasan los 6 con más fichas; si hay menos de 6 vivos, entran los últimos eliminados. Con 9 inscritos no hay reparto de solo mesas de 5 y 6: cada ronda es una de 5 y una de 4.

Artesanal pasa en 6 de 6 semillas, en 3 de 6 todavía con fichas. Quiebra en 3 de 6. Fichas finales: media 18 297, desvío 21 260, mínimo 0, máximo 48 175. Los rivales se retiraron ante un all-in 130 de 379 veces (34 %).

Neto de artesanal por mesa compartida: base +3 837, conservador +3 821, aleatorio +3 710, darwin +2 384, espia +1 124, novato +1 155, estadistico +769, bandido −663. Una variante más corta de push y de calls quebró en 4 de 6 y no se quedó.
