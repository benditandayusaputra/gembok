from __future__ import annotations

import hashlib
import hmac

import mlkem

LABEL_SEED = b"GEMBOK-v1/benih"
LABEL_TAG = b"GEMBOK-v1/bukti"
LABEL_CHECK = b"GEMBOK-v1/cek"
MAX_CONTEXT = 255

def derive_seeds(puf_key: bytes) -> tuple[bytes, bytes]:
    if len(puf_key) != 32:
        raise ValueError("keluaran PUF harus 32 byte")
    out = hashlib.shake_256(LABEL_SEED + puf_key).digest(64)
    return out[:32], out[32:]

def helper_check(puf_key: bytes, mask: bytes) -> bytes:
    assert len(puf_key) == 32
    return hashlib.shake_256(LABEL_CHECK + puf_key + mask).digest(16)

def compute_tag(k_shared: bytes, context: bytes) -> bytes:
    if len(context) > MAX_CONTEXT:
        raise ValueError("konteks terlalu panjang")
    return hashlib.sha3_256(
        LABEL_TAG + k_shared + bytes([len(context)]) + context
    ).digest()

class Chip:

    def __init__(self, params: mlkem.Params, puf_key: bytes) -> None:
        self._params = params
        self._puf_key = puf_key

    def _power_up(self) -> tuple[bytes, bytes]:
        d, z = derive_seeds(self._puf_key)
        return mlkem.keygen_internal(self._params, d, z)

    def enroll(self) -> bytes:
        ek, _dk = self._power_up()
        return ek

    def prove(self, c: bytes, context: bytes = b"") -> bytes:
        if not mlkem.check_ciphertext(self._params, c):
            raise ValueError("panjang ciphertext salah")
        _ek, dk = self._power_up()
        k_shared = mlkem.decaps_internal(self._params, dk, c)
        return compute_tag(k_shared, context)

class Verifier:

    def __init__(self, params: mlkem.Params, ek: bytes) -> None:
        if not mlkem.check_encaps_key(params, ek):
            raise ValueError("kunci publik chip tidak lolos pemeriksaan FIPS 203")
        self._params = params
        self._ek = ek

    def challenge(self) -> tuple[bytes, bytes]:
        k_shared, c = mlkem.encaps(self._params, self._ek)
        return c, k_shared

    @staticmethod
    def check(k_shared: bytes, context: bytes, tag: bytes) -> bool:
        return hmac.compare_digest(compute_tag(k_shared, context), tag)

def authenticate(chip: Chip, verifier: Verifier, context: bytes = b"") -> bool:
    c, k_shared = verifier.challenge()
    tag = chip.prove(c, context)
    return verifier.check(k_shared, context, tag)
