from __future__ import annotations

import hashlib
import os
from dataclasses import dataclass

N = 256
Q = 3329
ZETA = 17
INV_128 = 3303

@dataclass(frozen=True)
class Params:

    name: str
    k: int
    eta1: int
    eta2: int
    du: int
    dv: int

    @property
    def ek_len(self) -> int:
        return 384 * self.k + 32

    @property
    def dk_len(self) -> int:
        return 768 * self.k + 96

    @property
    def ct_len(self) -> int:
        return 32 * (self.du * self.k + self.dv)

ML_KEM_512 = Params("ML-KEM-512", k=2, eta1=3, eta2=2, du=10, dv=4)
ML_KEM_768 = Params("ML-KEM-768", k=3, eta1=2, eta2=2, du=10, dv=4)
ML_KEM_1024 = Params("ML-KEM-1024", k=4, eta1=2, eta2=2, du=11, dv=5)

PARAMS = {p.name: p for p in (ML_KEM_512, ML_KEM_768, ML_KEM_1024)}

def bitrev7(i: int) -> int:
    r = 0
    for _ in range(7):
        r = (r << 1) | (i & 1)
        i >>= 1
    return r

ZETAS = [pow(ZETA, bitrev7(i), Q) for i in range(128)]
GAMMAS = [pow(ZETA, 2 * bitrev7(i) + 1, Q) for i in range(128)]

def H(s: bytes) -> bytes:
    return hashlib.sha3_256(s).digest()

def J(s: bytes) -> bytes:
    return hashlib.shake_256(s).digest(32)

def G(c: bytes) -> tuple[bytes, bytes]:
    g = hashlib.sha3_512(c).digest()
    return g[:32], g[32:]

def PRF(eta: int, s: bytes, b: int) -> bytes:
    assert len(s) == 32 and 0 <= b < 256
    return hashlib.shake_256(s + bytes([b])).digest(64 * eta)

def XOF(rho: bytes, j: int, i: int, nbytes: int) -> bytes:
    return hashlib.shake_128(rho + bytes([j, i])).digest(nbytes)

def bits_to_bytes(bits: list[int]) -> bytes:
    assert len(bits) % 8 == 0
    out = bytearray(len(bits) // 8)
    for i, bit in enumerate(bits):
        out[i // 8] |= bit << (i % 8)
    return bytes(out)

def bytes_to_bits(data: bytes) -> list[int]:
    return [(byte >> j) & 1 for byte in data for j in range(8)]

def byte_encode(d: int, f: list[int]) -> bytes:
    assert len(f) == N and 1 <= d <= 12
    limit = Q if d == 12 else (1 << d)
    bits = []
    for a in f:
        assert 0 <= a < limit
        bits.extend((a >> j) & 1 for j in range(d))
    return bits_to_bytes(bits)

def byte_decode(d: int, data: bytes) -> list[int]:
    assert len(data) == 32 * d and 1 <= d <= 12
    m = Q if d == 12 else (1 << d)
    bits = bytes_to_bits(data)
    return [
        sum(bits[i * d + j] << j for j in range(d)) % m
        for i in range(N)
    ]

def compress(d: int, x: int) -> int:
    return (((x << d) * 2 + Q) // (2 * Q)) % (1 << d)

def decompress(d: int, y: int) -> int:
    return (Q * y + (1 << (d - 1))) >> d

def compress_poly(d: int, f: list[int]) -> list[int]:
    return [compress(d, x) for x in f]

def decompress_poly(d: int, f: list[int]) -> list[int]:
    return [decompress(d, y) for y in f]

def sample_ntt(rho: bytes, j: int, i: int) -> list[int]:
    assert len(rho) == 32
    a: list[int] = []
    nblocks = 3
    while True:
        stream = XOF(rho, j, i, 168 * nblocks)
        a.clear()
        pos = 0
        while pos + 3 <= len(stream) and len(a) < N:
            c0, c1, c2 = stream[pos], stream[pos + 1], stream[pos + 2]
            pos += 3
            d1 = c0 + 256 * (c1 % 16)
            d2 = (c1 // 16) + 16 * c2
            if d1 < Q:
                a.append(d1)
            if d2 < Q and len(a) < N:
                a.append(d2)
        if len(a) == N:
            return list(a)
        nblocks += 1

def sample_poly_cbd(eta: int, data: bytes) -> list[int]:
    assert len(data) == 64 * eta
    bits = bytes_to_bits(data)
    f = []
    for i in range(N):
        x = sum(bits[2 * i * eta + j] for j in range(eta))
        y = sum(bits[2 * i * eta + eta + j] for j in range(eta))
        f.append((x - y) % Q)
    return f

def ntt(f: list[int]) -> list[int]:
    f = list(f)
    i = 1
    length = 128
    while length >= 2:
        for start in range(0, N, 2 * length):
            zeta = ZETAS[i]
            i += 1
            for j in range(start, start + length):
                t = (zeta * f[j + length]) % Q
                f[j + length] = (f[j] - t) % Q
                f[j] = (f[j] + t) % Q
        length //= 2
    return f

def ntt_inv(f: list[int]) -> list[int]:
    f = list(f)
    i = 127
    length = 2
    while length <= 128:
        for start in range(0, N, 2 * length):
            zeta = ZETAS[i]
            i -= 1
            for j in range(start, start + length):
                t = f[j]
                f[j] = (t + f[j + length]) % Q
                f[j + length] = (zeta * (f[j + length] - t)) % Q
        length *= 2
    return [(x * INV_128) % Q for x in f]

def base_case_multiply(a0: int, a1: int, b0: int, b1: int, gamma: int) -> tuple[int, int]:
    c0 = (a0 * b0 + a1 * b1 * gamma) % Q
    c1 = (a0 * b1 + a1 * b0) % Q
    return c0, c1

def multiply_ntts(f: list[int], g: list[int]) -> list[int]:
    h = [0] * N
    for i in range(128):
        h[2 * i], h[2 * i + 1] = base_case_multiply(
            f[2 * i], f[2 * i + 1], g[2 * i], g[2 * i + 1], GAMMAS[i]
        )
    return h

def poly_add(f: list[int], g: list[int]) -> list[int]:
    return [(a + b) % Q for a, b in zip(f, g)]

def poly_sub(f: list[int], g: list[int]) -> list[int]:
    return [(a - b) % Q for a, b in zip(f, g)]

def _expand_matrix(p: Params, rho: bytes) -> list[list[list[int]]]:
    return [[sample_ntt(rho, j, i) for j in range(p.k)] for i in range(p.k)]

def kpke_keygen(p: Params, d: bytes) -> tuple[bytes, bytes]:
    assert len(d) == 32
    rho, sigma = G(d + bytes([p.k]))
    n = 0
    a_hat = _expand_matrix(p, rho)
    s = []
    for _ in range(p.k):
        s.append(sample_poly_cbd(p.eta1, PRF(p.eta1, sigma, n)))
        n += 1
    e = []
    for _ in range(p.k):
        e.append(sample_poly_cbd(p.eta1, PRF(p.eta1, sigma, n)))
        n += 1
    s_hat = [ntt(x) for x in s]
    e_hat = [ntt(x) for x in e]
    t_hat = []
    for i in range(p.k):
        acc = [0] * N
        for j in range(p.k):
            acc = poly_add(acc, multiply_ntts(a_hat[i][j], s_hat[j]))
        t_hat.append(poly_add(acc, e_hat[i]))
    ek_pke = b"".join(byte_encode(12, t) for t in t_hat) + rho
    dk_pke = b"".join(byte_encode(12, x) for x in s_hat)
    return ek_pke, dk_pke

def kpke_encrypt(p: Params, ek_pke: bytes, m: bytes, r: bytes) -> bytes:
    assert len(ek_pke) == p.ek_len and len(m) == 32 and len(r) == 32
    n = 0
    t_hat = [byte_decode(12, ek_pke[384 * i:384 * (i + 1)]) for i in range(p.k)]
    rho = ek_pke[384 * p.k:384 * p.k + 32]
    a_hat = _expand_matrix(p, rho)
    y = []
    for _ in range(p.k):
        y.append(sample_poly_cbd(p.eta1, PRF(p.eta1, r, n)))
        n += 1
    e1 = []
    for _ in range(p.k):
        e1.append(sample_poly_cbd(p.eta2, PRF(p.eta2, r, n)))
        n += 1
    e2 = sample_poly_cbd(p.eta2, PRF(p.eta2, r, n))
    y_hat = [ntt(x) for x in y]
    u = []
    for i in range(p.k):
        acc = [0] * N
        for j in range(p.k):
            acc = poly_add(acc, multiply_ntts(a_hat[j][i], y_hat[j]))
        u.append(poly_add(ntt_inv(acc), e1[i]))
    mu = decompress_poly(1, byte_decode(1, m))
    acc = [0] * N
    for j in range(p.k):
        acc = poly_add(acc, multiply_ntts(t_hat[j], y_hat[j]))
    v = poly_add(poly_add(ntt_inv(acc), e2), mu)
    c1 = b"".join(byte_encode(p.du, compress_poly(p.du, x)) for x in u)
    c2 = byte_encode(p.dv, compress_poly(p.dv, v))
    return c1 + c2

def kpke_decrypt(p: Params, dk_pke: bytes, c: bytes) -> bytes:
    assert len(dk_pke) == 384 * p.k and len(c) == p.ct_len
    c1 = c[:32 * p.du * p.k]
    c2 = c[32 * p.du * p.k:]
    u = [
        decompress_poly(p.du, byte_decode(p.du, c1[32 * p.du * i:32 * p.du * (i + 1)]))
        for i in range(p.k)
    ]
    v = decompress_poly(p.dv, byte_decode(p.dv, c2))
    s_hat = [byte_decode(12, dk_pke[384 * i:384 * (i + 1)]) for i in range(p.k)]
    acc = [0] * N
    for j in range(p.k):
        acc = poly_add(acc, multiply_ntts(s_hat[j], ntt(u[j])))
    w = poly_sub(v, ntt_inv(acc))
    return byte_encode(1, compress_poly(1, w))

def keygen_internal(p: Params, d: bytes, z: bytes) -> tuple[bytes, bytes]:
    assert len(d) == 32 and len(z) == 32
    ek_pke, dk_pke = kpke_keygen(p, d)
    ek = ek_pke
    dk = dk_pke + ek + H(ek) + z
    return ek, dk

def encaps_internal(p: Params, ek: bytes, m: bytes) -> tuple[bytes, bytes]:
    assert len(m) == 32
    k_shared, r = G(m + H(ek))
    c = kpke_encrypt(p, ek, m, r)
    return k_shared, c

def decaps_internal(p: Params, dk: bytes, c: bytes) -> bytes:
    dk_pke = dk[:384 * p.k]
    ek_pke = dk[384 * p.k:768 * p.k + 32]
    h = dk[768 * p.k + 32:768 * p.k + 64]
    z = dk[768 * p.k + 64:768 * p.k + 96]
    m_prime = kpke_decrypt(p, dk_pke, c)
    k_prime, r_prime = G(m_prime + h)
    k_bar = J(z + c)
    c_prime = kpke_encrypt(p, ek_pke, m_prime, r_prime)
    return k_prime if c == c_prime else k_bar

def check_encaps_key(p: Params, ek: bytes) -> bool:
    if len(ek) != p.ek_len:
        return False
    for i in range(p.k):
        chunk = ek[384 * i:384 * (i + 1)]
        if byte_encode(12, byte_decode(12, chunk)) != chunk:
            return False
    return True

def check_decaps_key(p: Params, dk: bytes) -> bool:
    if len(dk) != p.dk_len:
        return False
    ek = dk[384 * p.k:768 * p.k + 32]
    h = dk[768 * p.k + 32:768 * p.k + 64]
    return H(ek) == h

def check_ciphertext(p: Params, c: bytes) -> bool:
    return len(c) == p.ct_len

def keygen(p: Params) -> tuple[bytes, bytes]:
    return keygen_internal(p, os.urandom(32), os.urandom(32))

def encaps(p: Params, ek: bytes) -> tuple[bytes, bytes]:
    if not check_encaps_key(p, ek):
        raise ValueError("kunci enkapsulasi tidak lolos pemeriksaan FIPS 203 Bagian 7.2")
    return encaps_internal(p, ek, os.urandom(32))

def decaps(p: Params, dk: bytes, c: bytes) -> bytes:
    if not check_ciphertext(p, c):
        raise ValueError("panjang ciphertext salah (FIPS 203 Bagian 7.3)")
    if not check_decaps_key(p, dk):
        raise ValueError("kunci dekapsulasi tidak lolos pemeriksaan FIPS 203 Bagian 7.3")
    return decaps_internal(p, dk, c)
