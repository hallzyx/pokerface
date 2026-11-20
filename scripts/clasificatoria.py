#!/usr/bin/env python3
"""Clasificatoria al formato de docs/torneo.md, contra los 8 bots del kit.

10 rondas, 50 manos o 6 minutos, ciegas 50/100, las fichas se arrastran y
quien llega a 0 no vuelve. Pasan los 6 con más fichas; si hay menos de 6
vivos, entran los últimos eliminados. Varias semillas de reparto y de
baraja. Con 9 inscritos no existe un reparto solo de mesas de 5 y 6: esa
semilla juega 5 y 4.
"""

from __future__ import annotations

import json
import os
import random
import re
import signal
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BIN = ROOT / "bin"
SALIDA = ROOT / "resultados" / "clasificatoria"
LINEA = re.compile(r"^\s+#(\d+)\s+(\S+)\s+(\d+)\s+fichas\s*$")
EMPUJE = re.compile(r"fold_al_empuje\s+(\d+)/(\d+)")

BOTS = [
    "artesanal",
    "aleatorio",
    "conservador",
    "novato",
    "base",
    "estadistico",
    "bandido",
    "espia",
    "darwin",
]


def tamanos(n: int) -> list[int]:
    if n < 2:
        return []
    if n <= 6:
        return [n]
    k = 1
    while 5 * k <= n:
        if n <= 6 * k:
            base, extra = divmod(n, k)
            return [base + 1 if i < extra else base for i in range(k)]
        k += 1
    k = max(1, (n + 5) // 6)
    if n // k > 8:
        k = (n + 7) // 8
    base, extra = divmod(n, k)
    return [base + 1 if i < extra else base for i in range(k)]


def sentar(vivos: list[str], rng: random.Random) -> list[list[str]]:
    orden = vivos[:]
    rng.shuffle(orden)
    grupos = []
    i = 0
    for talla in tamanos(len(orden)):
        grupos.append(orden[i : i + talla])
        i += talla
    return [g for g in grupos if len(g) >= 2]


def comando(nombre: str, addr: str) -> tuple[list[str], Path | None]:
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
    cwd = ROOT / "kit" / "bots" / "novato" if nombre == "novato" else None
    return tabla[nombre], cwd


def lanzar(cmd: list[str], cwd: Path | None, log: Path) -> subprocess.Popen:
    log.parent.mkdir(parents=True, exist_ok=True)
    fh = open(log, "w", encoding="utf-8")
    proc = subprocess.Popen(cmd, cwd=cwd, stdout=fh, stderr=subprocess.STDOUT, start_new_session=True)
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


def esperar_escucha(proc: subprocess.Popen, log: Path) -> bool:
    limite = time.time() + 15
    while time.time() < limite:
        if proc.poll() is not None:
            return False
        if log.exists() and "escuchando" in log.read_text(encoding="utf-8", errors="replace"):
            return True
        time.sleep(0.05)
    return False


def parsear(texto: str) -> dict[str, int]:
    fichas = {}
    for linea in texto.splitlines():
        m = LINEA.match(linea)
        if m:
            fichas[m.group(2)] = int(m.group(3))
    return fichas


def jugar_mesa(nombre: str, bots: list[str], fichas: dict[str, int], semilla: int, port: int, carpeta: Path) -> dict[str, int]:
    carpeta.mkdir(parents=True, exist_ok=True)
    stacks = {b: fichas[b] for b in bots}
    ruta = carpeta / "stacks.json"
    ruta.write_text(json.dumps(stacks), encoding="utf-8")
    addr_flag = f":{port}"
    addr = f"127.0.0.1:{port}"
    mesa_log = carpeta / "mesa.log"
    mesa = lanzar(
        [
            str(BIN / "fases"),
            "-addr", addr_flag,
            "-id", nombre,
            "-jugadores", str(len(bots)),
            "-min-jugadores", str(len(bots)),
            "-rondas", "50",
            "-stack", "10000",
            "-stacks", str(ruta),
            "-ciega-chica", "50",
            "-ciega-grande", "100",
            "-timeout-ms", "5000",
            "-semilla", str(semilla),
            "-orden", ",".join(bots),
        ],
        None,
        mesa_log,
    )
    procesos = [mesa]
    try:
        if not esperar_escucha(mesa, mesa_log):
            raise RuntimeError(f"{nombre}: la mesa no escuchó")
        for bot in bots:
            cmd, cwd = comando(bot, addr)
            procesos.append(lanzar(cmd, cwd, carpeta / f"{bot}.log"))
        try:
            mesa.wait(timeout=360)
        except subprocess.TimeoutExpired as exc:
            raise RuntimeError(f"{nombre}: pasó de 6 minutos") from exc
    finally:
        for proc in procesos:
            matar(proc)
    texto = mesa_log.read_text(encoding="utf-8", errors="replace")
    salida = parsear(texto)
    if len(salida) != len(bots):
        raise RuntimeError(f"{nombre}: clasificación incompleta ({len(salida)}/{len(bots)})")
    return salida


def fold_al_empuje(carpeta: Path) -> tuple[int, int]:
    se_fue = encara = 0
    for log in carpeta.rglob("artesanal.log"):
        texto = log.read_text(encoding="utf-8", errors="replace")
        for m in EMPUJE.finditer(texto):
            se_fue += int(m.group(1))
            encara += int(m.group(2))
    return se_fue, encara


def clasificar(fichas: dict[str, int], eliminados: list[str]) -> list[str]:
    vivos = [b for b in BOTS if fichas.get(b, 0) > 0]
    orden_elim = {b: i for i, b in enumerate(eliminados)}
    def clave(b: str) -> tuple:
        # Más fichas primero; entre quebrados, el que aguantó más (más al final de la lista).
        return (fichas.get(b, 0), orden_elim.get(b, 10_000), b)
    return sorted(BOTS, key=clave, reverse=True)


def una_semilla(semilla: int) -> dict:
    rng = random.Random(semilla)
    fichas = {b: 10000 for b in BOTS}
    eliminados: list[str] = []
    contra: dict[str, dict[str, int]] = {b: {"mesas": 0, "neto": 0} for b in BOTS if b != "artesanal"}
    rondas = []
    for ronda in range(1, 11):
        vivos = [b for b in BOTS if fichas[b] > 0]
        if len(vivos) <= 1:
            break
        grupos = sentar(vivos, rng)
        for i, grupo in enumerate(grupos, start=1):
            antes = {b: fichas[b] for b in grupo}
            nombre = f"s{semilla}-r{ronda}-m{i}"
            salida = jugar_mesa(
                nombre,
                grupo,
                fichas,
                semilla * 1000 + ronda * 10 + i,
                22000 + (semilla % 50) * 300 + ronda * 10 + i,
                SALIDA / f"semilla-{semilla}" / nombre,
            )
            for b, saldo in salida.items():
                fichas[b] = saldo
                if saldo <= 0 and b not in eliminados:
                    eliminados.append(b)
            if "artesanal" in grupo:
                neto = fichas["artesanal"] - antes["artesanal"]
                for rival in grupo:
                    if rival == "artesanal":
                        continue
                    contra[rival]["mesas"] += 1
                    contra[rival]["neto"] += neto
        rondas.append({"ronda": ronda, "fichas": dict(fichas)})
    orden = clasificar(fichas, eliminados)
    return {
        "semilla": semilla,
        "fichas": fichas,
        "orden": orden,
        "eliminados": eliminados,
        "pasa": "artesanal" in orden[:6],
        "pasa_con_fichas": "artesanal" in orden[:6] and fichas["artesanal"] > 0,
        "quiebra": fichas["artesanal"] <= 0,
        "puesto": orden.index("artesanal") + 1,
        "contra": contra,
        "rondas": rondas,
    }


def desvio(xs: list[float]) -> float:
    if len(xs) < 2:
        return 0.0
    media = sum(xs) / len(xs)
    return (sum((x - media) ** 2 for x in xs) / (len(xs) - 1)) ** 0.5


def main() -> None:
    semillas = [int(s) for s in sys.argv[1:]] or [1, 2, 3, 4, 5, 6]
    if not (BIN / "fases").exists() or not (BIN / "artesanal").exists():
        raise SystemExit("faltan binarios: make clasificatoria")
    SALIDA.mkdir(parents=True, exist_ok=True)
    resultados = []
    for semilla in semillas:
        print(f"semilla {semilla}", flush=True)
        inicio = time.time()
        resultado = una_semilla(semilla)
        resultado["segundos"] = round(time.time() - inicio, 1)
        resultados.append(resultado)
        print(
            f"  puesto {resultado['puesto']} fichas {resultado['fichas']['artesanal']} "
            f"pasa={resultado['pasa']} quiebra={resultado['quiebra']} en {resultado['segundos']}s",
            flush=True,
        )
    se_fue, encara = fold_al_empuje(SALIDA)
    n = len(resultados)
    pasan = sum(1 for r in resultados if r["pasa"])
    pasan_fichas = sum(1 for r in resultados if r["pasa_con_fichas"])
    quiebran = sum(1 for r in resultados if r["quiebra"])
    fichas = [r["fichas"]["artesanal"] for r in resultados]
    media = sum(fichas) / n
    print("\nClasificatoria")
    print(f"  semillas: {n}")
    print(f"  artesanal pasa: {pasan}/{n}  (con fichas: {pasan_fichas}/{n})")
    print(f"  artesanal quiebra: {quiebran}/{n}")
    print(f"  fichas finales: media {media:.0f}  desvío {desvio([float(x) for x in fichas]):.0f}  min {min(fichas)}  max {max(fichas)}")
    print(f"  fold al empuje: {se_fue}/{encara}")
    print("\nNeto de artesanal en mesas compartidas")
    for rival in [b for b in BOTS if b != "artesanal"]:
        mesas = sum(r["contra"][rival]["mesas"] for r in resultados)
        neto = sum(r["contra"][rival]["neto"] for r in resultados)
        if mesas == 0:
            continue
        print(f"  vs {rival:<12} {mesas:3} mesas  neto {neto:+d}  ({neto / mesas:+.0f} por mesa)")
    (SALIDA / "resumen.json").write_text(json.dumps(resultados, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"\nDetalle en {SALIDA}")


if __name__ == "__main__":
    main()
