from __future__ import annotations

M32 = 0xFFFFFFFF

def ro_freq(seed: int, idx: int) -> int:
    x = (seed ^ ((idx * 0x9E3779B1) & M32)) & M32
    x ^= x >> 16
    x = (x * 0x85EBCA6B) & M32
    x ^= x >> 13
    x = (x * 0xC2B2AE35) & M32
    x ^= x >> 16
    return 20000 + (x & 0x3FF)

def enroll(seed: int, n_ro: int = 768, thresh: int = 64, key_bits: int = 256):
    n_cand = n_ro - 1
    mask = [0] * n_cand
    key = []
    skip = False
    for c in range(n_cand):
        if skip or len(key) == key_bits:
            skip = False
            continue
        d = ro_freq(seed, c) - ro_freq(seed, c + 1)
        if abs(d) >= thresh:
            mask[c] = 1
            key.append(1 if d > 0 else 0)
            skip = True
    mask_bytes = bytearray((n_cand + 7) // 8)
    for c, b in enumerate(mask):
        mask_bytes[c >> 3] |= b << (c & 7)
    key_int = sum(b << i for i, b in enumerate(key))
    return key_int.to_bytes(32, "little"), bytes(mask_bytes), len(key)

def ro_half_ps(seed: int, idx: int, base: int) -> int:
    return base + (ro_freq(seed, idx) - 20000) // 4
