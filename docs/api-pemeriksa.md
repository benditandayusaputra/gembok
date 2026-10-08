# API pemeriksa GEMBOK

Dokumen ini adalah kontrak HTTP layanan `gembok-verifier` (kode di `sw/verifier`). Dasbor di `sw/dashboard` hanya memakai titik akhir di bawah ini.

Alamat bawaan: `http://127.0.0.1:8080`. Semua contoh memakai `curl`.

## Aturan umum

- Semua jawaban berupa JSON UTF-8 dengan `Cache-Control: no-store`.
- Permintaan `POST` wajib memakai `Content-Type: application/json`. Isinya satu objek JSON, paling besar 4 KiB. Ruas yang tidak dikenal ditolak. Isi kosong dianggap `{}`.
- Permintaan dari situs lain ditolak: bila ada header `Origin`, nilainya harus sama dengan `Host`; bila ada `Sec-Fetch-Site`, nilainya harus `same-origin` atau `none`.
- Bila layanan mendengar di alamat loopback (bawaan), header `Host` harus `localhost`, `127.0.0.1`, atau `::1`. Ini menutup serangan DNS rebinding dari halaman web lain.
- Tidak ada autentikasi. Layanan ini alat demo; jangan dibuka ke jaringan yang tidak dipercaya.
- Batas waktu tiap permintaan dihitung saat layanan mulai dari parameter PUF chip: `maks(20 detik, coba_nyala x chip.batas_waktu_puf_ms + 15 detik)`, 40 detik untuk parameter bawaan. Rinciannya di bagian Batas waktu chip. Perintah ke chip dijalankan satu per satu.
- Waktu ditulis dalam UTC format RFC 3339. Data biner (kunci, tanda tangan, isi memori) ditulis sebagai hex huruf kecil.

## Galat

Galat selalu berbentuk:

```json
{"galat": {"kode": "BELUM_TERDAFTAR", "pesan": "Chip belum didaftarkan. Jalankan POST /api/daftar lebih dulu."}}
```

Ruas `kode_chip` ikut muncul bila galat berasal dari kode galat chip (STATUS bit 7..4, lihat `docs/peta-register.md`).

| HTTP | `kode` | Arti |
|---|---|---|
| 400 | `PERMINTAAN_TIDAK_SAH` | JSON rusak, ruas asing, isi terlalu besar, atau nilai di luar batas |
| 403 | `ASAL_DITOLAK` | `Origin` tidak sama dengan `Host` |
| 403 | `LINTAS_SITUS_DITOLAK` | `Sec-Fetch-Site` menunjukkan permintaan dari situs lain |
| 404 | `TIDAK_DITEMUKAN` | titik akhir API tidak ada |
| 405 | `METODE_TIDAK_DIIZINKAN` | metode salah; header `Allow` menyebut metode yang benar |
| 409 | `BELUM_TERDAFTAR` | belum ada sertifikat dan data bantu |
| 409 | `SUDAH_TERDAFTAR` | sudah terdaftar dan `ulang` tidak bernilai `true` |
| 409 | `BUKAN_SIMULASI` | penggantian chip diminta pada backend `mmio` |
| 409 | `TOPENG_TIDAK_SESUAI` | panjang topeng data bantu tersimpan tidak sama dengan panjang yang diminta chip (jumlah osilator di CAPS berbeda); PUF_RECON tidak dikirim. Daftarkan ulang chip |
| 409 | `CHIP_MENOLAK` | chip menjawab dengan kode galat, lihat `kode_chip` |
| 415 | `JENIS_ISI_DITOLAK` | `POST` tanpa `Content-Type: application/json` |
| 421 | `HOST_DITOLAK` | header `Host` bukan nama loopback |
| 500 | `GALAT_INTERNAL` | galat di layanan; rinciannya hanya ada di log |
| 502 | `CHIP_GAGAL` | akses ke chip gagal |
| 503 | `PEMBATAS_LAJU` | pembatas laju chip tetap aktif setelah tiga kali menunggu; ada header `Retry-After` |
| 503 | `PENERBIT_TIDAK_ADA` | kunci penerbit tidak dimuat |
| 503 | `TANDA_TANGAN_HABIS` | 1024 tanda tangan LMS penerbit sudah terpakai |
| 503 | `DIBATALKAN` | permintaan dibatalkan atau melewati batas waktu |
| 504 | `CHIP_TIDAK_MENJAWAB` | STATUS tidak menunjukkan selesai dalam batas waktu perintah (lihat Batas waktu chip) |

## GET /api/status

Keadaan layanan, chip, pendaftaran, dan penerbit. Dasbor memanggilnya tiap 3 detik.

```
curl http://127.0.0.1:8080/api/status
```

| Ruas | Isi |
|---|---|
| `backend` | `sim` atau `mmio` |
| `siklus_simulasi` | `true` bila jumlah siklus berasal dari simulasi (angka tetap) |
| `clock_hz` | 50000000 |
| `parameter_bawaan` | `{k, nama}` dari opsi `--k`, dipakai saat mendaftar |
| `ambang_bawaan` | ambang PUF yang dipakai `POST /api/daftar` bila permintaan tidak menyebutkan `ambang`, dari opsi `--ambang` (bawaan 64) |
| `coba_nyala` | berapa kali PUF_RECON dicoba sebelum pemeriksa menyimpulkan chip gagal memulihkan kunci, dari opsi `--coba-nyala` (bawaan 5) |
| `peringatan` | larik `{kode, pesan}`, kosong bila tidak ada masalah. Lihat bagian Peringatan mode PUF di bawah |
| `chip.id`, `chip.versi` | register ID dan VERSION, misalnya `0x47454D42` dan `1.0` |
| `chip.mode_puf`, `chip.nama_mode_puf` | CAPS bit 1..0: 0 osilator cincin, 1 model simulasi, 2 kunci pengembangan tetap, 3 tidak dikenal |
| `chip.bangunan_debug`, `chip.jendela_log2`, `chip.jumlah_suara` | CAPS bit 2 (PUF_DEBUG), bit 7..3 (WIN_LOG2: satu jendela ukur osilator = 2^WIN_LOG2 siklus), dan bit 15..8 (VOTES) |
| `chip.jumlah_osilator`, `chip.panjang_topeng` | N_RO dari CAPS bit 27..16, dan panjang topeng data bantu yang diminta chip, `(N_RO + 6) / 8` byte (96 untuk 768 osilator, paling banyak 128) |
| `chip.batas_waktu_puf_ms` | batas waktu driver untuk PUF_ENROLL dan PUF_RECON, dihitung dari CAPS (lihat Batas waktu chip) |
| `chip.sudah_boot`, `chip.sibuk`, `chip.puf_siap`, `chip.pendinginan` | bit STATUS BOOTED, BUSY, PUF_READY, COOLDOWN |
| `chip.sisa_pendinginan_siklus` | register COOLDOWN |
| `chip.jumlah_bukti` | register PROOFS (jumlah PROVE sejak reset) |
| `chip.galat_terakhir`, `chip.siklus_terakhir`, `chip.ambang_puf` | STATUS.ERR, CYCLES, PUF_THRESH |
| `terdaftar` | `true` bila berkas sertifikat ada |
| `sertifikat` | `null`, atau `{id_chip, parameter, diterbitkan, sidik_ek, indeks_tanda_tangan, sah, masalah}` |
| `penerbit` | `{tersedia, algoritma, id, tanda_tangan_terpakai, tanda_tangan_maksimum}` |
| `simulasi` | `null` pada `mmio`, atau `{chip, daftar_sendiri, pilihan}` pada `sim` |
| `waktu` | waktu layanan |

`sertifikat.sah` adalah hasil pemeriksaan tanda tangan penerbit saat itu juga. `sidik_ek` adalah 8 byte pertama SHA3-256(ek).

Susunan CAPS (register `0x1C`):

| Bit | Isi |
|---|---|
| 1..0 | mode PUF (0 osilator cincin, 1 model simulasi, 2 kunci pengembangan tetap) |
| 2 | bangunan debug (PUF_DEBUG) |
| 7..3 | WIN_LOG2, jendela ukur 2^WIN_LOG2 siklus clock |
| 15..8 | VOTES, jumlah suara per pasangan |
| 27..16 | N_RO, jumlah osilator |
| 31..28 | nol |

Contoh: bangunan PUF_MODE 0 dengan nilai bawaan (768 osilator, 5 suara, WIN_LOG2 12) terbaca `0x03000560`. Backend `sim` melaporkan `0x03000561` (model simulasi, nilai lain sama). Layanan menolak chip yang CAPS-nya berisi N_RO di luar 2 sampai 1025 atau WIN_LOG2 di luar 1 sampai 20.

### Batas waktu chip

Driver menunggu STATUS.DONE paling lama 5 detik untuk perintah biasa (ENROLL, PROVE, WIPE). PUF_ENROLL dan PUF_RECON mengukur osilator berkali-kali, dan lamanya bergantung pada CAPS. Untuk kedua perintah itu batasnya:

```
B = (N_RO - 1) x (VOTES x (2^WIN_LOG2 + 32) + 16) + 100000      siklus, bilangan bulat 64 bit
batas = maks(5 s, 2 x B / 50 MHz + 1 s)
```

| N_RO, VOTES, WIN_LOG2 | B (siklus) | Batas PUF | Batas permintaan (`coba_nyala` 5) |
|---|---|---|---|
| 768, 5, 12 (bawaan) | 15.943.152 | 5 s (rumus 1,64 s) | 40 s |
| 768, 5, 16 | 251.565.552 | 11,06 s | 70,3 s |
| 1025, 15, 16 (terlama yang diizinkan perangkat keras) | 1.007.240.864 | 41,29 s | 221,4 s |

Pada parameter terlama, satu PUF_RECON sebenarnya sekitar 5 detik dan PUF_ENROLL sekitar 20 detik, jadi batas di atas memberi ruang dua kali lipat. Sebelum tiap perintah, driver menunggu chip menganggur dengan batas yang lebih panjang dari keduanya, supaya perintah PUF lama yang masih berjalan (misalnya karena permintaan sebelumnya dibatalkan) tidak dianggap macet. `chip.batas_waktu_puf_ms` melaporkan batas PUF yang berlaku, dan layanan menulis kedua batas ke log saat mulai.

### Peringatan mode PUF

Pada backend `mmio`, layanan membaca CAPS dan mengisi `peringatan` bila kunci chip tidak rahasia. Peringatan yang sama ditulis ke log saat layanan mulai, dan dasbor menampilkannya di atas halaman. Backend `sim` selalu mengirim larik kosong; sifat simulasinya sudah terlihat dari `backend` dan `chip.nama_mode_puf`.

| `kode` | Syarat | Arti |
|---|---|---|
| `PUF_MODEL_SIMULASI` | CAPS bit 1..0 = 1 | kunci dihitung dari rumus model yang diketahui umum, bukan dari fisik chip |
| `PUF_KUNCI_PENGEMBANGAN` | CAPS bit 1..0 = 2 | kunci pengembangan tetap dan publik; papan lain dengan bitstream yang sama lolos pemeriksaan |
| `PUF_MODE_TIDAK_DIKENAL` | CAPS bit 1..0 = 3 | mode yang tidak didefinisikan peta register; anggap kunci tidak rahasia |
| `PUF_BANGUNAN_DEBUG` | CAPS bit 2 = 1 | PUF_MEASURE dan PUF_DBG membuka hitungan mentah osilator, sama dengan membocorkan kunci |

```json
"peringatan": [{"kode": "PUF_KUNCI_PENGEMBANGAN", "pesan": "Bitstream memakai kunci pengembangan tetap (PUF_MODE 2). Kunci ini publik, jadi hasil ASLI tidak membuktikan keaslian chip."}]
```

Peringatan tidak menghentikan pemeriksaan. Hasil ASLI pada mode ini hanya membuktikan bahwa jalur register dan brankas bekerja, bukan bahwa chip itu asli.

## POST /api/daftar

Alat pendaftaran. Urutannya mengikuti bagian "Mode brankas" di peta register:

1. Tulis PUF_THRESH, jalankan PUF_ENROLL, baca topeng HELPER_MASK sepanjang `chip.panjang_topeng` byte (dari CAPS) dan HELPER_CHK 16 byte.
2. Jalankan ENROLL dengan k yang diminta dan baca EK.
3. Terbitkan sertifikat atas EK (tanda tangan LMS, memakai satu dari 1024 daun).
4. Simpan data bantu dan sertifikat ke direktori data.

| Ruas permintaan | Wajib | Isi |
|---|---|---|
| `id_chip` | tidak | 1 sampai 64 karakter `A-Z a-z 0-9 . _ -`, diawali huruf atau angka. Bila kosong: `GEMBOK-` diikuti 12 hex dari SHA3-256("GEMBOK-v1/id" \|\| ek) |
| `k` | tidak | 3 atau 4. Bawaan dari `--k` |
| `ambang` | tidak | 1 sampai 65535, ditulis ke PUF_THRESH. Bila tidak ada atau `null`: nilai opsi `--ambang` (bawaan 64), yang juga dilaporkan `ambang_bawaan` di status |
| `ulang` | tidak | harus `true` bila sudah ada pendaftaran |

```
curl -X POST -H 'Content-Type: application/json' -d '{"id_chip":"GEMBOK-DEMO-01"}' http://127.0.0.1:8080/api/daftar
```

```json
{
 "sertifikat": {
  "versi": 1,
  "id_chip": "GEMBOK-DEMO-01",
  "parameter": "ML-KEM-768",
  "ek": "30dc2dac7caf8362...",
  "penerbit": "c2978a385b1c3bf997764c2ee7c4e394",
  "diterbitkan": "2026-10-08T03:23:57Z",
  "algoritma_tanda_tangan": "LMS_SHA256_M32_H10/LMOTS_SHA256_N32_W8",
  "tanda_tangan": "0000000000000004..."
 },
 "data_bantu": {"ambang": 64, "panjang_topeng": 96, "topeng": "55295555...", "cek": "109bd53790edfbbf7079966a14ab876d", "jumlah_pasangan": 256},
 "sidik_ek": "5aa0813a4f5b8cf9",
 "siklus_puf": 6000000,
 "siklus_daftar": 18100,
 "waktu_daftar_chip_us": 362,
 "siklus_simulasi": true,
 "tanda_tangan_terpakai": 1,
 "tanda_tangan_maksimum": 1024,
 "waktu_total_us": 127251
}
```

`data_bantu.panjang_topeng` adalah panjang topeng dalam byte. Panjang ini ikut disimpan di berkas data bantu dan dibandingkan dengan CAPS sebelum setiap PUF_RECON.

Bila PUF gagal (misalnya ambang terlalu tinggi), jawabannya HTTP 409 `CHIP_MENOLAK` dengan `kode_chip` `PUF_FAIL`, dan tidak ada yang disimpan.

## POST /api/nyalakan

Langkah tiap chip menyala: tulis HELPER_MASK dan HELPER_CHK dari data bantu tersimpan, lalu PUF_RECON. Layanan juga menulis ulang PUF_THRESH dengan ambang saat pendaftaran, walau chip tidak memakainya saat PUF_RECON. Isi permintaan `{}`.

Pengukuran osilator berderau, jadi satu PUF_RECON yang gagal belum berarti chip palsu. Aturannya:

1. Bandingkan panjang topeng tersimpan dengan `chip.panjang_topeng` dari CAPS. Bila berbeda, jawab 409 `TOPENG_TIDAK_SESUAI` tanpa mengirim PUF_RECON.
2. Jalankan PUF_RECON. Bila chip menjawab `BAD_HELPER` atau `PUF_FAIL`, tulis ulang data bantu dan ulangi, sampai `coba_nyala` kali (opsi `--coba-nyala`, bawaan 5).
3. Kode chip lain tidak diulang. Galat bus atau batas waktu menjadi galat HTTP (`CHIP_GAGAL`, `CHIP_TIDAK_MENJAWAB`).

```json
{"berhasil": true, "puf_siap": true, "kode_chip": "OK", "keterangan": "Kunci PUF dipulihkan pada percobaan ke-1 dan cocok dengan nilai cek data bantu.", "percobaan": 1, "percobaan_maksimum": 5, "siklus": 5300000, "waktu_chip_us": 106000, "siklus_simulasi": true}
```

| Ruas jawaban | Isi |
|---|---|
| `berhasil`, `puf_siap` | `true` bila kunci PUF pulih dan STATUS.PUF_READY satu |
| `kode_chip` | `OK`, atau kode galat chip dari percobaan terakhir |
| `percobaan` | jumlah PUF_RECON yang dijalankan |
| `percobaan_maksimum` | batas percobaan (`coba_nyala`) |
| `siklus`, `waktu_chip_us` | CYCLES dan waktu chip dari percobaan terakhir |

Penolakan oleh chip bukan galat HTTP: jawabannya tetap 200 dengan `berhasil: false` dan `kode_chip` berisi `BAD_HELPER` atau `PUF_FAIL`. Chip tiruan gagal di semua percobaan:

```json
{"berhasil": false, "puf_siap": false, "kode_chip": "BAD_HELPER", "keterangan": "Chip menolak data bantu dalam 5 percobaan: data bantu PUF tidak cocok dengan kunci yang dipulihkan.", "percobaan": 5, "percobaan_maksimum": 5, "siklus": 5300000, "waktu_chip_us": 106000, "siklus_simulasi": true}
```

## POST /api/periksa

Satu pemeriksaan keaslian.

| Ruas permintaan | Wajib | Isi |
|---|---|---|
| `konteks` | tidak | teks UTF-8 paling banyak 255 byte, diikat ke bukti lewat CTX dan CTXLEN |

Urutan di layanan:

1. Muat sertifikat dan data bantu. Bila belum ada: 409 `BELUM_TERDAFTAR`.
2. Periksa tanda tangan LMS penerbit dan isi sertifikat (termasuk cek modulus ek, FIPS 203 Bagian 7.2). Bila gagal, hasilnya `PALSU` dan chip tidak ditanya sama sekali.
3. Encaps terhadap ek dari sertifikat: kunci bersama K dan ciphertext c.
4. Bila STATUS.PUF_READY nol, nyalakan chip dengan data bantu tersimpan (`nyala_otomatis: true`) memakai aturan percobaan ulang yang sama dengan `POST /api/nyalakan`. Bila semua percobaan ditolak dengan `BAD_HELPER` atau `PUF_FAIL`, hasilnya `PALSU` dengan alasan `DATA_BANTU_DITOLAK` dan PROVE tidak dikirim. Bila panjang topeng tidak cocok dengan CAPS, jawabannya 409 `TOPENG_TIDAK_SESUAI`.
5. Tulis c, konteks, dan panjangnya, lalu PROVE. Bila chip menjawab RATE, baca COOLDOWN, tunggu, lalu ulangi (paling banyak tiga kali).
6. Bandingkan TAG dengan SHA3-256("GEMBOK-v1/bukti" || K || byte(len(ctx)) || ctx) dalam waktu konstan.

```
curl -X POST -H 'Content-Type: application/json' -d '{"konteks":"scan-001"}' http://127.0.0.1:8080/api/periksa
```

```json
{
 "id": 1,
 "waktu": "2026-10-08T03:23:57.861Z",
 "hasil": "ASLI",
 "alasan": "BUKTI_COCOK",
 "keterangan": "Bukti dari chip sama dengan SHA3-256 atas kunci bersama yang hanya diketahui pemeriksa dan pemegang kunci rahasia.",
 "id_chip": "GEMBOK-DEMO-01",
 "parameter": "ML-KEM-768",
 "konteks": "scan-001",
 "siklus_chip": 45400,
 "waktu_chip_us": 908,
 "waktu_total_us": 2067,
 "siklus_simulasi": true,
 "nyala_otomatis": false,
 "percobaan_nyala": 0,
 "tunggu_pembatas": 0,
 "tag_chip": "9080226374cacf60559a8e3301289e6d54efaa5c8f0787670273f13c7b95e350",
 "backend": "sim",
 "chip_simulasi": "asli"
}
```

| Ruas jawaban | Isi |
|---|---|
| `hasil` | `ASLI` atau `PALSU` |
| `alasan` | kode alasan, lihat tabel di bawah |
| `keterangan` | penjelasan singkat untuk manusia |
| `siklus_chip` | register CYCLES sesudah PROVE; 0 bila PROVE tidak dijalankan |
| `waktu_chip_us` | `siklus_chip / 50`, yaitu waktu chip pada clock 50 MHz dalam mikrodetik |
| `waktu_total_us` | waktu dinding di layanan, termasuk Encaps, lalu lintas bus, penyalaan otomatis, dan tunggu pembatas laju |
| `siklus_simulasi` | `true` bila siklus berasal dari backend `sim` |
| `nyala_otomatis` | `true` bila PUF_RECON dijalankan otomatis |
| `percobaan_nyala` | jumlah PUF_RECON pada penyalaan otomatis; 0 bila PUF sudah siap, lebih dari 1 bila percobaan pertama ditolak |
| `tunggu_pembatas` | berapa kali menunggu pembatas laju |
| `kode_chip` | kode galat chip bila pemeriksaan berhenti karena chip menolak |
| `tag_chip` | TAG 32 byte yang dikirim chip |
| `chip_simulasi` | `asli` atau `tiruan` pada backend `sim` |

| `alasan` | `hasil` | Arti |
|---|---|---|
| `BUKTI_COCOK` | `ASLI` | TAG sama dengan nilai yang dihitung pemeriksa |
| `BUKTI_TIDAK_COCOK` | `PALSU` | chip menjawab, tetapi TAG berbeda: chip tidak memegang kunci dekapsulasi pasangan ek di sertifikat, atau bukti lama diputar ulang |
| `SERTIFIKAT_TIDAK_SAH` | `PALSU` | tanda tangan penerbit salah, sertifikat diubah, atau dari kunci penerbit lain |
| `DATA_BANTU_DITOLAK` | `PALSU` | PUF_RECON gagal dengan `BAD_HELPER` atau `PUF_FAIL` di semua `percobaan_nyala` percobaan: PUF chip ini tidak menghasilkan kunci chip yang didaftarkan |
| `CHIP_TANPA_KUNCI` | `PALSU` | PROVE dijawab `NO_KEY` |
| `CHIP_MENOLAK` | `PALSU` | chip menjawab kode galat lain |

Galat operasional (chip tidak menjawab, pembatas laju tetap aktif, panjang topeng tidak cocok) bukan hasil `PALSU`, melainkan galat HTTP, dan tidak masuk riwayat.

## POST /api/serang/baca-kunci

Demo serangan: host mencoba membaca rahasia dari memori byte chip. Isi permintaan `{}`.

Bila chip sudah terdaftar, layanan lebih dulu menjalankan satu pemeriksaan biasa dengan konteks `serang/baca-kunci` (tercatat di riwayat), lalu membaca wilayah berikut lewat jendela memori byte `0x8000 + 4*i`:

| `nama` | Alamat | Panjang | Jenis |
|---|---|---|---|
| `D`, `Z`, `M`, `K`, `SIGMA`, `KBAR` | 0x1800 sampai 0x18BF | 6 x 32 | rahasia |
| `DK` | 0x1000 | 384k (1152 untuk k = 3) | rahasia |
| `H` | 0x18C0 | 32 | publik, pembanding |
| `TAG` | 0x18E0 | 32 | publik, pembanding |

Pembacaan baru dimulai setelah STATUS.BUSY nol, jadi nilai nol bukan akibat chip sedang sibuk.

```json
{
 "waktu": "2026-10-08T03:23:58.373Z",
 "bukti_dijalankan": true,
 "bukti": {"id": 5, "hasil": "ASLI", "alasan": "BUKTI_COCOK", "siklus_chip": 45400, "...": "..."},
 "wilayah": [
  {"nama": "D", "alamat": "0x1800", "panjang": 32, "rahasia": true, "semua_nol": true, "byte_bukan_nol": 0, "isi_hex": "0000000000000000000000000000000000000000000000000000000000000000"},
  {"nama": "DK", "alamat": "0x1000", "panjang": 1152, "rahasia": true, "semua_nol": true, "byte_bukan_nol": 0, "isi_hex": "0000..."},
  {"nama": "H", "alamat": "0x18C0", "panjang": 32, "rahasia": false, "semua_nol": false, "byte_bukan_nol": 32, "isi_hex": "5aa0813a4f5b8cf932ef62415e7b1f7527026330bfce7a6a78c1e4d431f533c6"}
 ],
 "jumlah_byte_rahasia": 1344,
 "byte_rahasia_bukan_nol": 0,
 "rahasia_semua_nol": true,
 "jumlah_byte_publik": 64,
 "byte_publik_bukan_nol": 64,
 "penjelasan": "Host membaca 1344 byte wilayah rahasia dan semuanya nol. ..."
}
```

Contoh di atas dipersingkat; jawaban asli memuat sembilan wilayah dan objek `bukti` lengkap seperti jawaban `/api/periksa`. Bila belum terdaftar, `bukti` bernilai `null` dan `catatan` menjelaskan bahwa memori dibaca tanpa bukti sebelumnya.

## POST /api/simulasi/chip

Hanya pada backend `sim`. Mengganti chip yang terpasang di "pembaca".

| Ruas permintaan | Wajib | Isi |
|---|---|---|
| `chip` | ya | `asli` atau `tiruan` |
| `daftar_sendiri` | tidak | bila `true`, chip yang baru dipasang langsung menjalankan PUF_ENROLL dengan PUF-nya sendiri |

Mengganti chip sama dengan mencabut dan memasang: chip baru mulai dari keadaan sesudah reset (memori nol, PUF_READY nol, PROOFS nol). Chip `asli` adalah model PUF dengan benih 1, chip `tiruan` model PUF dengan benih 2 (rumus `tb/common/puf_model.py`). Sertifikat dan data bantu yang tersimpan tidak berubah, jadi chip tiruan "membawa" sertifikat dan data bantu chip asli:

- tanpa `daftar_sendiri`: pemeriksaan berikutnya menjalankan PUF_RECON otomatis, chip menjawab `BAD_HELPER` di setiap percobaan, hasilnya `PALSU` dengan alasan `DATA_BANTU_DITOLAK` dan `percobaan_nyala` sama dengan `coba_nyala`;
- dengan `daftar_sendiri`: chip tiruan punya kunci sendiri dan menjawab PROVE, tetapi ek-nya bukan ek di sertifikat, hasilnya `PALSU` dengan alasan `BUKTI_TIDAK_COCOK`.

Jawaban berupa objek status yang sama dengan `GET /api/status`. Pada backend `mmio` jawabannya 409 `BUKAN_SIMULASI`.

## GET /api/riwayat

Hasil pemeriksaan terakhir, terbaru lebih dulu. Parameter `batas` (1 sampai 50, bawaan 50).

```
curl 'http://127.0.0.1:8080/api/riwayat?batas=10'
```

```json
{"jumlah": 5, "riwayat": [{"id": 5, "hasil": "ASLI", "...": "..."}]}
```

Riwayat hanya disimpan di memori layanan dan hilang saat layanan dimulai ulang.

## Sertifikat

Sertifikat mengikat id chip, set parameter, dan ek, ditandatangani penerbit dengan LMS (RFC 8554) memakai `LMS_SHA256_M32_H10` dan `LMOTS_SHA256_N32_W8`. Tanda tangan 1452 byte, kunci publik penerbit 56 byte.

Yang ditandatangani adalah badan kanonik berikut. Tiap ruas didahului panjangnya, `ruas(x) = u32be(len(x)) || x`:

```
badan = ruas("GEMBOK-v1/sertifikat")
     || ruas(versi)          1 byte, nilai 1
     || ruas(id_chip)        UTF-8
     || ruas(parameter)      "ML-KEM-768" atau "ML-KEM-1024"
     || ruas(ek)             384k + 32 byte
     || ruas(penerbit)       16 byte, pengenal I kunci LMS penerbit
     || ruas(diterbitkan)    u64be, detik Unix
```

Pemeriksa menerima sertifikat hanya bila semua syarat ini terpenuhi: versi 1; id chip memenuhi aturan di atas; parameter dikenal dan panjang ek cocok; ek lolos cek modulus FIPS 203; `penerbit` sama dengan I milik kunci publik penerbit yang dipercaya; tanda tangan LMS sah atas badan kanonik. Format JSON berkasnya sama dengan objek `sertifikat` pada jawaban `/api/daftar`.

## Berkas di direktori data

| Berkas | Isi |
|---|---|
| `penerbit/kunci-publik.json` | kunci publik LMS penerbit (titik kepercayaan pemeriksa) |
| `penerbit/kunci-rahasia.json` | benih, I, dan indeks daun berikutnya. Izin 0600 |
| `penerbit/pohon.bin` | cache 1024 nilai daun pohon Merkle (publik), dibangun ulang bila hilang atau tidak cocok |
| `chip/sertifikat.json` | sertifikat chip terdaftar |
| `chip/data-bantu.json` | data bantu PUF (publik): `versi` 2, `ambang`, `panjang_topeng`, `topeng`, `cek`, `disimpan`. Panjang tercatat harus sama dengan panjang topeng. Berkas versi 1 (tanpa `panjang_topeng`, topeng 96 byte) masih terbaca |
| `proses.lock` | kunci berkas agar dua proses tidak memakai direktori yang sama |

Indeks daun disimpan ke cakram (tulis ke berkas sementara, fsync, rename, fsync direktori) sebelum tanda tangan dibuat. Daun yang gagal dipakai tetap dianggap habis dan tidak pernah dipakai ulang.

## Batasan

- Hanya k = 3 (ML-KEM-768) dan k = 4 (ML-KEM-1024). Perangkat keras juga mendukung k = 2, tetapi pustaka standar Go tidak menyediakan ML-KEM-512. Backend `sim` menolak k = 2 dengan `BAD_PARAM`.
- Kunci penerbit LMS bersifat stateful dan hanya bisa membuat 1024 tanda tangan. Tiap pendaftaran memakai satu.
- Jumlah siklus pada backend `sim` adalah angka tetap (PROVE 45.400 untuk k = 3 dan 65.300 untuk k = 4, ENROLL 18.100 dan 26.900), bukan hasil pengukuran.
