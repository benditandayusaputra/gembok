from __future__ import annotations

import contextlib
import io
import random
import sys
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "model"))
sys.path.insert(0, str(ROOT / "tests"))

import gembok
import mlkem
import run_acvp

ALL_PARAMS = list(mlkem.PARAMS.values())

def quiet_acvp() -> int:
    with contextlib.redirect_stdout(io.StringIO()):
        return run_acvp.main()

def schoolbook_mul(f: list[int], g: list[int]) -> list[int]:
    h = [0] * mlkem.N
    for i, a in enumerate(f):
        for j, b in enumerate(g):
            k = i + j
            if k < mlkem.N:
                h[k] = (h[k] + a * b) % mlkem.Q
            else:
                h[k - mlkem.N] = (h[k - mlkem.N] - a * b) % mlkem.Q
    return h

class TestAcvp(unittest.TestCase):
    def test_semua_vektor_nist_lolos(self):
        self.assertEqual(quiet_acvp(), 0)

class TestKontrolNegatif(unittest.TestCase):

    def test_skala_ntt_invers_salah(self):
        with mock.patch.object(mlkem, "INV_128", 1):
            self.assertEqual(quiet_acvp(), 1)

    def test_tanpa_penolakan_implisit(self):
        with mock.patch.object(mlkem, "J", lambda s: bytes(32)):
            self.assertEqual(quiet_acvp(), 1)

    def test_pembulatan_kompresi_salah(self):
        with mock.patch.object(mlkem, "compress", lambda d, x: ((x << d) // mlkem.Q) % (1 << d)):
            self.assertEqual(quiet_acvp(), 1)

    def test_cek_hash_kunci_dimatikan(self):
        with mock.patch.object(mlkem, "check_decaps_key", lambda p, dk: True):
            self.assertEqual(quiet_acvp(), 1)

    def test_cek_modulus_kunci_dimatikan(self):
        with mock.patch.object(mlkem, "check_encaps_key", lambda p, ek: True):
            self.assertEqual(quiet_acvp(), 1)

class TestSifatMatematis(unittest.TestCase):
    def setUp(self):
        self.rng = random.Random(203)

    def rand_poly(self) -> list[int]:
        return [self.rng.randrange(mlkem.Q) for _ in range(mlkem.N)]

    def test_tabel_zeta(self):
        self.assertEqual(pow(mlkem.ZETA, 128, mlkem.Q), mlkem.Q - 1)
        self.assertEqual((128 * mlkem.INV_128) % mlkem.Q, 1)
        self.assertEqual(len(set(mlkem.ZETAS)), 128)

    def test_ntt_bolak_balik(self):
        for _ in range(20):
            f = self.rand_poly()
            self.assertEqual(mlkem.ntt_inv(mlkem.ntt(f)), f)

    def test_kasus_sudut_ntt(self):
        nol = [0] * mlkem.N
        maks = [mlkem.Q - 1] * mlkem.N
        impuls = [1] + [0] * (mlkem.N - 1)
        for f in (nol, maks, impuls):
            self.assertEqual(mlkem.ntt_inv(mlkem.ntt(f)), f)

    def test_perkalian_ntt_sama_dengan_perkalian_langsung(self):
        for _ in range(5):
            f, g = self.rand_poly(), self.rand_poly()
            lewat_ntt = mlkem.ntt_inv(mlkem.multiply_ntts(mlkem.ntt(f), mlkem.ntt(g)))
            self.assertEqual(lewat_ntt, schoolbook_mul(f, g))

    def test_kompresi_semua_masukan(self):
        for d in (1, 4, 5, 10, 11):
            batas = (mlkem.Q + (1 << d)) // (1 << (d + 1))
            for x in range(mlkem.Q):
                y = mlkem.compress(d, x)
                self.assertTrue(0 <= y < (1 << d))
                x2 = mlkem.decompress(d, y)
                selisih = min((x - x2) % mlkem.Q, (x2 - x) % mlkem.Q)
                self.assertLessEqual(selisih, batas)

    def test_pengemasan_byte_bolak_balik(self):
        for d in range(1, 13):
            batas = mlkem.Q if d == 12 else (1 << d)
            f = [self.rng.randrange(batas) for _ in range(mlkem.N)]
            data = mlkem.byte_encode(d, f)
            self.assertEqual(len(data), 32 * d)
            self.assertEqual(mlkem.byte_decode(d, data), f)

    def test_ukuran_kunci_dan_ciphertext(self):
        harapan = {
            "ML-KEM-512": (800, 1632, 768),
            "ML-KEM-768": (1184, 2400, 1088),
            "ML-KEM-1024": (1568, 3168, 1568),
        }
        for p in ALL_PARAMS:
            self.assertEqual((p.ek_len, p.dk_len, p.ct_len), harapan[p.name])

class TestMlKem(unittest.TestCase):
    def setUp(self):
        self.rng = random.Random(768)

    def rb(self, n: int = 32) -> bytes:
        return self.rng.randbytes(n)

    def test_enkapsulasi_dekapsulasi_cocok(self):
        for p in ALL_PARAMS:
            for _ in range(5):
                ek, dk = mlkem.keygen_internal(p, self.rb(), self.rb())
                k, c = mlkem.encaps_internal(p, ek, self.rb())
                self.assertEqual(mlkem.decaps_internal(p, dk, c), k)

    def test_ciphertext_diubah_menghasilkan_kunci_pengganti(self):
        for p in ALL_PARAMS:
            ek, dk = mlkem.keygen_internal(p, self.rb(), self.rb())
            z = dk[-32:]
            k, c = mlkem.encaps_internal(p, ek, self.rb())
            rusak = bytearray(c)
            rusak[self.rng.randrange(len(c))] ^= 0x01
            rusak = bytes(rusak)
            hasil = mlkem.decaps_internal(p, dk, rusak)
            self.assertNotEqual(hasil, k)
            self.assertEqual(hasil, mlkem.J(z + rusak))

    def test_api_publik_menolak_masukan_salah(self):
        p = mlkem.ML_KEM_768
        ek, dk = mlkem.keygen(p)
        k, c = mlkem.encaps(p, ek)
        self.assertEqual(mlkem.decaps(p, dk, c), k)
        with self.assertRaises(ValueError):
            mlkem.decaps(p, dk, c[:-1])
        with self.assertRaises(ValueError):
            mlkem.encaps(p, b"\xff" * p.ek_len)
        dk_rusak = bytearray(dk)
        dk_rusak[768 * p.k + 40] ^= 0x80
        with self.assertRaises(ValueError):
            mlkem.decaps(p, bytes(dk_rusak), c)

class TestUjiSilang(unittest.TestCase):

    def test_sama_dengan_kyber_py(self):
        try:
            from kyber_py.ml_kem import ML_KEM_512, ML_KEM_768, ML_KEM_1024
        except ImportError:
            self.skipTest("kyber-py tidak terpasang (pip install kyber-py)")
        pembanding = {
            "ML-KEM-512": ML_KEM_512,
            "ML-KEM-768": ML_KEM_768,
            "ML-KEM-1024": ML_KEM_1024,
        }
        rng = random.Random(2026)
        for p in ALL_PARAMS:
            ref = pembanding[p.name]
            for _ in range(20):
                d, z, m = rng.randbytes(32), rng.randbytes(32), rng.randbytes(32)
                ek, dk = mlkem.keygen_internal(p, d, z)
                self.assertEqual((ek, dk), ref._keygen_internal(d, z))
                k, c = mlkem.encaps_internal(p, ek, m)
                self.assertEqual((k, c), ref._encaps_internal(ek, m))
                self.assertEqual(mlkem.decaps_internal(p, dk, c), ref._decaps_internal(dk, c))
                rusak = bytearray(c)
                rusak[rng.randrange(len(c))] ^= 1 << rng.randrange(8)
                rusak = bytes(rusak)
                self.assertEqual(
                    mlkem.decaps_internal(p, dk, rusak), ref._decaps_internal(dk, rusak)
                )

class TestProtokolGembok(unittest.TestCase):
    def setUp(self):
        self.rng = random.Random(1)
        self.puf = self.rng.randbytes(32)

    def test_chip_asli_lolos(self):
        for p in ALL_PARAMS:
            chip = gembok.Chip(p, self.puf)
            pemeriksa = gembok.Verifier(p, chip.enroll())
            for _ in range(3):
                self.assertTrue(gembok.authenticate(chip, pemeriksa, b"scan-001"))

    def test_kunci_publik_sama_setiap_menyala(self):
        p = mlkem.ML_KEM_768
        self.assertEqual(gembok.Chip(p, self.puf).enroll(), gembok.Chip(p, self.puf).enroll())

    def test_chip_tiruan_gagal(self):
        p = mlkem.ML_KEM_768
        asli = gembok.Chip(p, self.puf)
        tiruan = gembok.Chip(p, self.rng.randbytes(32))
        pemeriksa = gembok.Verifier(p, asli.enroll())
        self.assertFalse(gembok.authenticate(tiruan, pemeriksa))

    def test_putar_ulang_bukti_lama_gagal(self):
        p = mlkem.ML_KEM_768
        chip = gembok.Chip(p, self.puf)
        pemeriksa = gembok.Verifier(p, chip.enroll())
        c_lama, _k_lama = pemeriksa.challenge()
        bukti_lama = chip.prove(c_lama)
        _c_baru, k_baru = pemeriksa.challenge()
        self.assertFalse(pemeriksa.check(k_baru, b"", bukti_lama))

    def test_konteks_berbeda_gagal(self):
        p = mlkem.ML_KEM_768
        chip = gembok.Chip(p, self.puf)
        pemeriksa = gembok.Verifier(p, chip.enroll())
        c, k = pemeriksa.challenge()
        bukti = chip.prove(c, b"toko-A")
        self.assertTrue(pemeriksa.check(k, b"toko-A", bukti))
        self.assertFalse(pemeriksa.check(k, b"toko-B", bukti))

    def test_ciphertext_diubah_gagal_tanpa_pesan_galat(self):
        p = mlkem.ML_KEM_768
        chip = gembok.Chip(p, self.puf)
        pemeriksa = gembok.Verifier(p, chip.enroll())
        c, k = pemeriksa.challenge()
        rusak = bytes([c[0] ^ 1]) + c[1:]
        bukti = chip.prove(rusak)
        self.assertEqual(len(bukti), 32)
        self.assertFalse(pemeriksa.check(k, b"", bukti))

    def test_masukan_salah_ditolak(self):
        p = mlkem.ML_KEM_768
        chip = gembok.Chip(p, self.puf)
        with self.assertRaises(ValueError):
            chip.prove(b"\x00" * (p.ct_len - 1))
        with self.assertRaises(ValueError):
            gembok.Verifier(p, b"\xff" * p.ek_len)
        with self.assertRaises(ValueError):
            gembok.derive_seeds(b"pendek")

    def test_chip_tidak_punya_jalur_keluar_kunci(self):
        publik = {n for n in dir(gembok.Chip) if not n.startswith("_")}
        self.assertEqual(publik, {"enroll", "prove"})

if __name__ == "__main__":
    unittest.main(verbosity=2)
