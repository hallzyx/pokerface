#!/usr/bin/env python3
"""Corre mesas locales con las condiciones del torneo y resume las fichas.

No toca el código del kit. Levanta ./bin/mesa y los bots ya compilados.
Uso: scripts/torneo-local.py [repeticiones]
"""

from __future__ import annotations

import json
import os
import re
import signal
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BIN = ROOT / "bin"
CONFIG = ROOT / "config" / "mesas.json"
SALIDA = ROOT / "resultados" / "torneo-local"

LINEA = re.compile(r"^\s+#(\d+)\s+(\S+)\s+(\d+)\s+fichas\s*$")


def comandos(nombre: str, addr: str) -> tuple[list[str], Path | None]:
    token = ["-addr", addr, "-token", nombre]
    tabla = {
        "artesanal": [str(BIN / "artesanal"), *token],
        "aleatorio": [str(BIN / "aleatorio"), "-semilla", "3", *token],
        "conservador": [str(BIN / "conservador"), *token],
        "base": [str(BIN / "base"), *token],
        "estadistico": [str(BIN / "estadistico"), "-simulaciones", "2000", "-semilla", "7", *token],
        "bandido": [str(BIN / "bandido"), "-silencioso", *token],
        "espia": [str(BIN / "espia"), "-silencioso", *token],
        "darwin": [str(BIN / "darwin"), "-silencioso", *token],
        "novato": [sys.executable, "bot.py", "-silencioso", *token],
    }
    if nombre not in tabla:
        raise SystemExit(f"bot desconocido: {nombre}")
    cwd = ROOT / "kit" / "bots" / "novato" if nombre == "novato" else None
    return tabla[nombre], cwd


def lanzar(cmd: list[str], cwd: Path | None, log: Path) -> subprocess.Popen:
    log.parent.mkdir(parents=True, exist_ok=True)
    fh = open(log, "w", encoding="utf-8")
    proc = subprocess.Popen(
        cmd,
        cwd=cwd,
        stdout=fh,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    proc._log_fh = fh  # type: ignore[attr-defined]
    return proc


def matar(proc: subprocess.Popen | None) -> None:
    if proc is None:
        return
    if proc.poll() is None:
        try:
            os.killpg(proc.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        else:
            try:
                proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(proc.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
    fh = getattr(proc, "_log_fh", None)
    if fh is not None and not fh.closed:
        fh.close()


def esperar_escucha(proc: subprocess.Popen, log: Path, segundos: float = 15) -> bool:
    limite = time.time() + segundos
    while time.time() < limite:
        if proc.poll() is not None:
            return False
        if log.exists() and "escuchando" in log.read_text(encoding="utf-8", errors="replace"):
            return True
        time.sleep(0.1)
    return False


def parsear(texto: str) -> list[dict]:
    puestos = []
    for linea in texto.splitlines():
        m = LINEA.match(linea)
        if not m:
            continue
        puestos.append({"puesto": int(m.group(1)), "id": m.group(2), "fichas": int(m.group(3))})
    return puestos


def jugar(nombre: str, bots: list[str], cond: dict, carpeta: Path, port: int) -> dict:
    addr = f"127.0.0.1:{port}"
    carpeta.mkdir(parents=True, exist_ok=True)
    mesa_cmd = [
        str(BIN / "mesa"),
        "-addr", f":{port}",
        "-id", nombre,
        "-jugadores", str(cond["jugadores"]),
        "-min-jugadores", str(cond["min_jugadores"]),
        "-rondas", str(cond["rondas"]),
        "-stack", str(cond["stack"]),
        "-ciega-chica", str(cond["ciega_chica"]),
        "-ciega-grande", str(cond["ciega_grande"]),
        "-timeout-ms", str(cond["timeout_ms"]),
    ]
    mesa_log = carpeta / "mesa.log"
    mesa = lanzar(mesa_cmd, None, mesa_log)
    procesos = [mesa]
    try:
        if not esperar_escucha(mesa, mesa_log):
            return {"nombre": nombre, "bots": bots, "error": "la mesa no llegó a escuchar", "puestos": [], "completo": False}
        for bot in bots:
            cmd, cwd = comandos(bot, addr)
            procesos.append(lanzar(cmd, cwd, carpeta / f"{bot}.log"))
        try:
            mesa.wait(timeout=cond["tope_segundos"])
            completo = True
        except subprocess.TimeoutExpired:
            completo = False
            matar(mesa)
    finally:
        for proc in procesos:
            matar(proc)
    texto = mesa_log.read_text(encoding="utf-8", errors="replace") if mesa_log.exists() else ""
    puestos = parsear(texto)
    return {
        "nombre": nombre,
        "bots": bots,
        "puestos": puestos,
        "completo": completo and len(puestos) == len(bots),
        "error": "" if puestos else "sin clasificación",
    }


def resumir(partidas: list[dict], stack: int) -> None:
    print()
    print("Resultados por mesa")
    print("-------------------")
    acumulado: dict[str, list[int]] = {}
    for partida in partidas:
        print(f"\n{partida['nombre']}  completo={partida['completo']}")
        if partida.get("error") and not partida["puestos"]:
            print(f"  error: {partida['error']}")
            continue
        for puesto in partida["puestos"]:
            neto = puesto["fichas"] - stack
            print(f"  #{puesto['puesto']} {puesto['id']:<14} {puesto['fichas']:>6}  ({neto:+d})")
            acumulado.setdefault(puesto["id"], []).append(puesto["fichas"])
    print("\nPromedio y desvío")
    print("-----------------")
    filas = []
    for bot, fichas in acumulado.items():
        media = sum(fichas) / len(fichas)
        filas.append((media, bot, len(fichas), desvio(fichas)))
    for media, bot, n, sigma in sorted(filas, reverse=True):
        print(f"  {bot:<14} {media:8.0f}  ±{sigma:6.0f}  en {n} mesas  (neto {media - stack:+.0f})")

    nuestras = acumulado.get("artesanal", [])
    if nuestras:
        bustos = sum(1 for f in nuestras if f <= 0)
        mitad = 0
        for partida in partidas:
            puestos = partida.get("puestos") or []
            if not puestos:
                continue
            corte = max(1, len(puestos) // 2)
            for p in puestos:
                if p["id"] == "artesanal" and p["puesto"] <= corte:
                    mitad += 1
        print(f"\nartesanal: {bustos}/{len(nuestras)} mesas en 0 fichas, {mitad}/{len(nuestras)} en la mitad alta")

    print("\nMisma mesa, más fichas")
    for rival in ("base", "novato", "conservador", "estadistico", "aleatorio", "bandido", "espia", "darwin"):
        ganan = pierden = empates = 0
        for partida in partidas:
            ids = {p["id"]: p["fichas"] for p in partida.get("puestos") or []}
            if "artesanal" not in ids or rival not in ids:
                continue
            if ids["artesanal"] > ids[rival]:
                ganan += 1
            elif ids["artesanal"] < ids[rival]:
                pierden += 1
            else:
                empates += 1
        if ganan + pierden + empates == 0:
            continue
        print(f"  vs {rival:<12} ganamos {ganan}  perdimos {pierden}  empates {empates}")


def desvio(xs: list[int]) -> float:
    if len(xs) < 2:
        return 0.0
    media = sum(xs) / len(xs)
    return (sum((x - media) ** 2 for x in xs) / (len(xs) - 1)) ** 0.5


def main() -> None:
    repeticiones = int(sys.argv[1]) if len(sys.argv) > 1 else 3
    cfg = json.loads(CONFIG.read_text(encoding="utf-8"))
    cond = cfg["condiciones"]
    if os.environ.get("RONDAS"):
        cond["rondas"] = int(os.environ["RONDAS"])
    if not (BIN / "mesa").exists() or not (BIN / "artesanal").exists():
        raise SystemExit("faltan binarios: corre make build")

    partidas = []
    for rep in range(1, repeticiones + 1):
        for i, bots in enumerate(cfg["mesas"], start=1):
            nombre = f"r{rep}-mesa{i}"
            print(f"jugando {nombre}: {', '.join(bots)}", flush=True)
            inicio = time.time()
            resultado = jugar(nombre, bots, cond, SALIDA / nombre, 19100 + rep * 10 + i)
            resultado["segundos"] = round(time.time() - inicio, 1)
            partidas.append(resultado)
            print(f"  listo en {resultado['segundos']}s  completo={resultado['completo']}", flush=True)

    SALIDA.mkdir(parents=True, exist_ok=True)
    (SALIDA / "resumen.json").write_text(json.dumps(partidas, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    resumir(partidas, cond["stack"])
    print(f"\nDetalle en {SALIDA}")


if __name__ == "__main__":
    main()
