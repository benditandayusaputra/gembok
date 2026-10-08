# Asal vektor uji

Vektor diunduh pada 8 Oktober 2026 dari repositori resmi NIST:

- https://github.com/usnistgov/ACVP-Server (cabang `master`)
- `gen-val/json-files/ML-KEM-keyGen-FIPS203/internalProjection.json`
- `gen-val/json-files/ML-KEM-encapDecap-FIPS203/internalProjection.json`

Berkas disimpan tanpa diubah, hanya diganti namanya.

| Berkas | SHA-256 |
|---|---|
| `ML-KEM-keyGen-FIPS203.json` | `d7a62a2c3476957f56dd8d24f9004ea6776ccfe995ffe71a65bb9506dc9c7b1b` |
| `ML-KEM-encapDecap-FIPS203.json` | `a556952ce869bb89c3a3196a701dad89647c193a34c86eafb61a9d710d5b810f` |

Isi: 75 kasus keyGen, 75 kasus encapsulation, 30 kasus decapsulation, 30 kasus encapsulationKeyCheck, dan 30 kasus decapsulationKeyCheck, terbagi rata untuk ML-KEM-512, 768, dan 1024.
