import hashlib
import json
import random
import sys
from pathlib import Path

sys.dont_write_bytecode = True

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "model"))
sys.path.insert(0, str(ROOT / "tb" / "common"))

import gembok
import mlkem
import puf_model

OUT = ROOT / "sw" / "verifier" / "internal" / "chip" / "testdata" / "fixture_pemeriksa.json"
PARAMS = {3: mlkem.ML_KEM_768, 4: mlkem.ML_KEM_1024}
DEV_KEY = (0x6B6F626D65672D6B6F626D65672D6B6F626D65672D6B6F626D65672D76656421).to_bytes(32, "little")
N_RO = 768
SEED = 20261008

def proof(p, chip, ek, c, context, k_verifier):
    tag = chip.prove(c, context)
    return {
        "c": c.hex(),
        "konteks": context.hex(),
        "k_pemeriksa": k_verifier.hex(),
        "tag": tag.hex(),
        "cocok": gembok.Verifier(p, ek).check(k_verifier, context, tag),
    }

def proofs(rng, p, chip, ek, contexts):
    out = []
    for context in contexts:
        k_shared, c = mlkem.encaps_internal(p, ek, rng.randbytes(32))
        out.append(proof(p, chip, ek, c, context, k_shared))
    k_shared, c = mlkem.encaps_internal(p, ek, rng.randbytes(32))
    pos = rng.randrange(len(c))
    damaged = c[:pos] + bytes([c[pos] ^ (1 << rng.randrange(8))]) + c[pos + 1:]
    out.append(proof(p, chip, ek, damaged, b"sandi-diubah", k_shared))
    return out

def levels(rng, puf_key, contexts):
    out = []
    for k, p in PARAMS.items():
        chip = gembok.Chip(p, puf_key)
        ek = chip.enroll()
        assert mlkem.check_encaps_key(p, ek)
        out.append({
            "k": k,
            "nama": p.name,
            "ek": ek.hex(),
            "h_ek": hashlib.sha3_256(ek).hexdigest(),
            "panjang_ct": p.ct_len,
            "bukti": proofs(rng, p, chip, ek, contexts),
        })
    return out

def mask_len(n_ro):
    return (n_ro + 6) // 8

def fixed_key_entry(rng, name, puf_key, mask):
    d, z = gembok.derive_seeds(puf_key)
    return {
        "nama": name,
        "kunci_puf": puf_key.hex(),
        "benih_d": d.hex(),
        "benih_z": z.hex(),
        "topeng": mask.hex(),
        "cek": gembok.helper_check(puf_key, mask).hex(),
        "tingkat": levels(rng, puf_key, [b"", b"scan-001", rng.randbytes(255)]),
    }

def model_entry(rng, chip_seed, thresh, with_levels, n_ro=N_RO):
    puf_key, mask, selected = puf_model.enroll(chip_seed, n_ro, thresh)
    assert len(mask) == mask_len(n_ro)
    entry = {
        "benih_chip": chip_seed,
        "ambang": thresh,
        "jumlah_osilator": n_ro,
        "frekuensi_awal": [puf_model.ro_freq(chip_seed, i) for i in range(16)],
        "terpilih": selected,
        "topeng": mask.hex(),
    }
    if selected == 256:
        entry["kunci_puf"] = puf_key.hex()
        entry["cek"] = gembok.helper_check(puf_key, mask).hex()
        if with_levels:
            entry["tingkat"] = levels(rng, puf_key, [b"gerbang-A"])
    return entry

def build():
    rng = random.Random(SEED)
    fixed = [
        fixed_key_entry(rng, "kunci pengembangan RTL (PUF_MODE 2)", DEV_KEY, bytes(mask_len(N_RO))),
        fixed_key_entry(rng, "kunci acak 1", rng.randbytes(32), rng.randbytes(mask_len(N_RO))),
        fixed_key_entry(rng, "kunci acak 2", rng.randbytes(32), rng.randbytes(mask_len(N_RO))),
    ]
    model = [
        model_entry(rng, 1, 64, True),
        model_entry(rng, 2, 64, True),
        model_entry(rng, 1, 200, False),
        model_entry(rng, 1, 900, False),
        model_entry(rng, 0xC0FFEE, 64, False),
        model_entry(rng, 1, 64, True, n_ro=1025),
        model_entry(rng, 3, 64, False, n_ro=801),
    ]
    return {
        "label": {
            "benih": gembok.LABEL_SEED.decode(),
            "bukti": gembok.LABEL_TAG.decode(),
            "cek": gembok.LABEL_CHECK.decode(),
        },
        "kunci_tetap": fixed,
        "model_puf": model,
    }

def main():
    out = Path(sys.argv[1]) if len(sys.argv) > 1 else OUT
    doc = build()
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(doc, indent=1) + "\n", encoding="utf-8")
    count = sum(len(level["bukti"]) for group in ("kunci_tetap", "model_puf") for entry in doc[group] for level in entry.get("tingkat", []))
    print(f"{len(doc['kunci_tetap'])} kunci tetap, {len(doc['model_puf'])} chip model, {count} bukti ditulis ke {out}")

if __name__ == "__main__":
    main()
