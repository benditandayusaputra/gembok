import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
KEYGEN_FILE = "ML-KEM-keyGen-FIPS203.json"
ENCAP_DECAP_FILE = "ML-KEM-encapDecap-FIPS203.json"
LEVELS = {"ML-KEM-512": 2, "ML-KEM-768": 3, "ML-KEM-1024": 4}
FIELDS = {
    "keyGen": ("d", "z", "ek", "dk"),
    "encapsulation": ("ek", "m", "c", "k"),
    "decapsulation": ("dk", "c", "k"),
    "encapsulationKeyCheck": ("ek",),
    "decapsulationKeyCheck": ("dk",),
}
WITH_VERDICT = ("encapsulationKeyCheck", "decapsulationKeyCheck")

def load_cases(vectors):
    keygen = json.loads((vectors / KEYGEN_FILE).read_text())
    for group in keygen["testGroups"]:
        for test in group["tests"]:
            yield "keyGen", group["parameterSet"], test
    encap_decap = json.loads((vectors / ENCAP_DECAP_FILE).read_text())
    for group in encap_decap["testGroups"]:
        for test in group["tests"]:
            yield group["function"], group["parameterSet"], test

def render_case(function, parameter_set, test):
    if function not in FIELDS:
        raise SystemExit(f"fungsi tidak dikenal di vektor: {function}")
    if parameter_set not in LEVELS:
        raise SystemExit(f"parameter tidak dikenal di vektor: {parameter_set}")
    lines = [f"uji {function} {LEVELS[parameter_set]} {test['tcId']}"]
    for name in FIELDS[function]:
        lines.append(f"{name} {bytes.fromhex(test[name]).hex()}")
    if function in WITH_VERDICT:
        lines.append(f"sah {int(test['testPassed'])}")
    lines.append("selesai")
    return lines

def main():
    parser = argparse.ArgumentParser(
        description="Ubah vektor NIST ACVP ML-KEM (JSON) menjadi berkas teks untuk gembok-cli acvp."
    )
    parser.add_argument("-o", "--keluaran", help="berkas keluaran (bawaan: keluaran standar)")
    parser.add_argument("--vektor", default=str(ROOT / "vectors"), help="folder berisi dua berkas JSON")
    args = parser.parse_args()

    cases = [render_case(*case) for case in load_cases(Path(args.vektor))]
    lines = ["gembok-acvp 1", f"kasus {len(cases)}"]
    for case in cases:
        lines.extend(case)
    text = "\n".join(lines) + "\n"

    if args.keluaran:
        Path(args.keluaran).write_text(text)
        print(f"{len(cases)} kasus ditulis ke {args.keluaran}", file=sys.stderr)
    else:
        sys.stdout.write(text)
    return 0

if __name__ == "__main__":
    sys.exit(main())
