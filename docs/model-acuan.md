# Model acuan Python

Model acuan adalah pembanding bit demi bit untuk RTL. Ditulis dari teks FIPS 203 (Agustus 2024) dan FIPS 202, tanpa ketergantungan luar, dan tidak menyalin kode implementasi lain.

Model ini model fungsional. Python tidak berjalan dalam waktu konstan, jadi model ini tidak untuk dipakai sebagai implementasi kriptografi.

## Berkas

| Berkas | Isi |
|---|---|
| `model/mlkem.py` | ML-KEM-512, 768, 1024 dan pemeriksaan masukan |
| `model/gembok.py` | Lapisan identitas: benih dari kunci PUF, nilai cek data bantu, tag bukti, chip dan pemeriksa |
| `tests/run_acvp.py` | Menjalankan vektor NIST ACVP terhadap model |
| `tests/test_model.py` | Vektor NIST, kontrol negatif, sifat matematis, uji silang dengan `kyber-py`, protokol |

## Fungsi dan algoritma FIPS 203

| Fungsi | Bagian FIPS 203 |
|---|---|
| `bitrev7`, `ZETAS`, `GAMMAS` | Bagian 4.3, tabel zeta dan gamma |
| `H`, `J`, `G`, `PRF`, `XOF` | Bagian 4.1 |
| `bits_to_bytes`, `bytes_to_bits` | Algoritma 3 dan 4 |
| `byte_encode`, `byte_decode` | Algoritma 5 dan 6 |
| `compress`, `decompress` | Bagian 4.2.1 |
| `sample_ntt` | Algoritma 7 |
| `sample_poly_cbd` | Algoritma 8 |
| `ntt`, `ntt_inv` | Algoritma 9 dan 10 |
| `multiply_ntts`, `base_case_multiply` | Algoritma 11 dan 12 |
| `kpke_keygen`, `kpke_encrypt`, `kpke_decrypt` | Algoritma 13, 14, 15 |
| `keygen_internal`, `encaps_internal`, `decaps_internal` | Algoritma 16, 17, 18 |
| `keygen`, `encaps`, `decaps` | Algoritma 19, 20, 21 |
| `check_encaps_key` | Bagian 7.2 (cek panjang dan cek modulus) |
| `check_decaps_key`, `check_ciphertext` | Bagian 7.3 (cek panjang dan cek hash) |

## Fungsi lapisan identitas

| Fungsi | Isi |
|---|---|
| `derive_seeds(kunci)` | (d, z) = SHAKE256("GEMBOK-v1/benih" \|\| kunci), 64 byte |
| `helper_check(kunci, topeng)` | SHAKE256("GEMBOK-v1/cek" \|\| kunci \|\| topeng), 16 byte |
| `compute_tag(K, konteks)` | SHA3-256("GEMBOK-v1/bukti" \|\| K \|\| panjang konteks \|\| konteks) |
| `Chip` | hanya punya `enroll()` dan `prove(c, konteks)`; kunci dibangkitkan ulang tiap panggilan |
| `Verifier` | `challenge()` memberi (c, K), `check(K, konteks, tag)` membandingkan dalam waktu konstan |

## Cara menjalankan

```
python3 tests/run_acvp.py
python3 -m unittest discover -s tests -v
```

Uji silang memakai `kyber-py` (`pip install kyber-py`). Bila belum terpasang, uji itu dilewati.

## Hasil

- Vektor resmi NIST ACVP: 240 dari 240 kasus lolos (25 keyGen, 25 encapsulation, 10 decapsulation, 10 encapsulationKeyCheck, 10 decapsulationKeyCheck, untuk tiap tingkat).
- 25 uji pendukung lolos, termasuk lima kontrol negatif (model yang sengaja dirusak harus gagal pada vektor NIST) dan uji silang 20 kasus acak per tingkat dengan `kyber-py`.

Asal vektor dan sidik SHA-256 berkasnya ada di `vectors/SUMBER.md`.
