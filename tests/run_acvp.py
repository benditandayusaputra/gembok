from __future__ import annotations

import json
import sys
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "model"))

import mlkem

VECTORS = ROOT / "vectors"

def hx(s: str) -> bytes:
    return bytes.fromhex(s)

def run_keygen(results) -> None:
    data = json.loads((VECTORS / "ML-KEM-keyGen-FIPS203.json").read_text())
    for group in data["testGroups"]:
        p = mlkem.PARAMS[group["parameterSet"]]
        for t in group["tests"]:
            ek, dk = mlkem.keygen_internal(p, hx(t["d"]), hx(t["z"]))
            ok = ek == hx(t["ek"]) and dk == hx(t["dk"])
            results[(p.name, "keyGen")].append((t["tcId"], ok))

def run_encap_decap(results) -> None:
    data = json.loads((VECTORS / "ML-KEM-encapDecap-FIPS203.json").read_text())
    for group in data["testGroups"]:
        p = mlkem.PARAMS[group["parameterSet"]]
        fn = group["function"]
        for t in group["tests"]:
            if fn == "encapsulation":
                k, c = mlkem.encaps_internal(p, hx(t["ek"]), hx(t["m"]))
                ok = k == hx(t["k"]) and c == hx(t["c"])
            elif fn == "decapsulation":
                k = mlkem.decaps_internal(p, hx(t["dk"]), hx(t["c"]))
                ok = k == hx(t["k"])
            elif fn == "encapsulationKeyCheck":
                ok = mlkem.check_encaps_key(p, hx(t["ek"])) == t["testPassed"]
            elif fn == "decapsulationKeyCheck":
                ok = mlkem.check_decaps_key(p, hx(t["dk"])) == t["testPassed"]
            else:
                raise SystemExit(f"fungsi tidak dikenal di vektor: {fn}")
            results[(p.name, fn)].append((t["tcId"], ok))

def main() -> int:
    results: dict[tuple[str, str], list[tuple[int, bool]]] = defaultdict(list)
    run_keygen(results)
    run_encap_decap(results)

    order = ["keyGen", "encapsulation", "decapsulation",
             "encapsulationKeyCheck", "decapsulationKeyCheck"]
    total = passed = 0
    print(f"{'parameter':<13}{'fungsi':<24}{'lolos':>7}{'total':>7}")
    for name in mlkem.PARAMS:
        for fn in order:
            rows = results.get((name, fn), [])
            ok = sum(1 for _, r in rows if r)
            total += len(rows)
            passed += ok
            mark = "" if ok == len(rows) else "   GAGAL: " + ",".join(str(i) for i, r in rows if not r)
            print(f"{name:<13}{fn:<24}{ok:>7}{len(rows):>7}{mark}")
    print(f"\nTOTAL {passed} dari {total} kasus lolos")
    return 0 if passed == total and total > 0 else 1

if __name__ == "__main__":
    sys.exit(main())
