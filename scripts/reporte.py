#!/usr/bin/env python3
"""Resume el log del bot en español.

Lee las líneas mano=… (y, si no hay, la línea resumen) y las conexiones
que escribe el SDK. No interpreta decisiones ni tokens.

Uso: scripts/reporte.py ARCHIVO
     scripts/reporte.py --probar
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

CONEXION = re.compile(r"conectado a (\S+) como (\S+) \(silla (-?\d+)\)")
CLAVE = re.compile(r"^[a-z_]+$")


def campos(linea: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for tok in linea.split():
        if "=" not in tok:
            continue
        clave, valor = tok.split("=", 1)
        if CLAVE.fullmatch(clave):
            out[clave] = valor
    return out


def entero(valor: str | None) -> int:
    if not valor:
        return 0
    try:
        return int(valor)
    except ValueError:
        return 0


def con_signo(n: int) -> str:
    if n > 0:
        return f"+{n}"
    return str(n)


def resumir(texto: str) -> str:
    conexiones = CONEXION.findall(texto)
    manos = neto = ganados = perdidos = sin_showdown = 0
    fichas = None
    hubo_manos = False
    for linea in texto.splitlines():
        c = campos(linea)
        if "mano" not in c or "nuestra" not in c:
            continue
        hubo_manos = True
        if c.get("nuestra") != "1":
            continue
        manos += 1
        neto += entero(c.get("neto"))
        if "fichas" in c:
            fichas = entero(c["fichas"])
        mio = entero(c.get("mio"))
        showdown = c.get("showdown") == "1"
        en_showdown = c.get("en_showdown") == "1"
        if showdown and mio > 0:
            ganados += 1
        elif showdown and en_showdown and mio == 0:
            perdidos += 1
        elif not showdown and mio > 0:
            sin_showdown += 1

    if not hubo_manos:
        for linea in texto.splitlines():
            c = campos(linea)
            if "showdown_ganados" not in c:
                continue
            manos = entero(c.get("manos"))
            neto = entero(c.get("neto"))
            ganados = entero(c.get("showdown_ganados"))
            perdidos = entero(c.get("showdown_perdidos"))
            sin_showdown = entero(c.get("sin_showdown"))
            break

    if not conexiones and manos == 0 and not hubo_manos:
        return "No hay manos en el log."

    lineas: list[str] = []
    if not conexiones:
        lineas.append("Conexión: no aparece en el log.")
    for direccion, alias, silla in conexiones:
        lineas.append(f"Conexión: {alias}, silla {silla}, {direccion}")
    lineas.append(f"Manos jugadas: {manos}")
    lineas.append(f"Fichas netas: {con_signo(neto)}")
    lineas.append(f"Showdowns ganados: {ganados}")
    lineas.append(f"Showdowns perdidos: {perdidos}")
    lineas.append(f"Manos ganadas sin showdown: {sin_showdown}")
    if fichas is not None:
        lineas.append(f"Fichas al cierre: {fichas}")
    return "\n".join(lineas)


def probar() -> None:
    texto = "\n".join(
        [
            "2026/11/20 19:00:00 conectado a 127.0.0.1:19111 como artesanal (silla 2)",
            "preflop AsAc línea allin eq 0.00 -> allin 1000 (empujo)",
            "mano=m1 n=1 pozo=150 ganadores=artesanal:150 mio=150 neto=50 fichas=10050 showdown=0 en_showdown=0 nuestra=1 cartas=-",
            "mano=m2 n=2 pozo=400 ganadores=rival:400 mio=0 neto=-200 fichas=9800 showdown=1 en_showdown=1 nuestra=1 cartas=artesanal:AsKh,rival:QsQh",
            "mano=ajena n=3 pozo=80 ganadores=otro:80 mio=0 neto=0 fichas=9800 showdown=0 en_showdown=0 nuestra=0 cartas=-",
            "resumen manos=2 neto=-150 showdown_ganados=0 showdown_perdidos=1 sin_showdown=1",
        ]
    )
    salida = resumir(texto)
    esperados = [
        "Conexión: artesanal, silla 2, 127.0.0.1:19111",
        "Manos jugadas: 2",
        "Fichas netas: -150",
        "Showdowns ganados: 0",
        "Showdowns perdidos: 1",
        "Manos ganadas sin showdown: 1",
        "Fichas al cierre: 9800",
    ]
    for quiero in esperados:
        if quiero not in salida:
            raise SystemExit(f"falta {quiero!r} en:\n{salida}")
    if "allin" in salida or "token" in salida:
        raise SystemExit(salida)
    if resumir("nada útil") != "No hay manos en el log.":
        raise SystemExit("el log vacío no avisa")
    print("ok")


def main() -> None:
    if len(sys.argv) == 2 and sys.argv[1] == "--probar":
        probar()
        return
    if len(sys.argv) != 2:
        raise SystemExit("uso: scripts/reporte.py ARCHIVO")
    path = Path(sys.argv[1])
    if not path.is_file():
        raise SystemExit(f"no está el log: {path}")
    print(resumir(path.read_text(encoding="utf-8", errors="replace")))


if __name__ == "__main__":
    main()
