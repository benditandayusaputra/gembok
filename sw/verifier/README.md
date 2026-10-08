# gembok-verifier

Layanan pemeriksa untuk chip identitas GEMBOK. Layanan ini mendaftarkan chip (PUF_ENROLL, ENROLL, lalu menerbitkan sertifikat), menyalakannya (PUF_RECON), memeriksa keasliannya dengan tantangan ML-KEM, dan menyajikan dasbor di `/`.

Ditulis dengan Go 1.24 dan hanya memakai pustaka standar: `crypto/mlkem`, `crypto/sha3`, `crypto/sha256`, `net/http`, `embed`. Tidak ada modul luar, jadi `go build` tidak butuh akses ke proxy modul.

Kontrak HTTP ada di `docs/api-pemeriksa.md`. Kontrak register chip ada di `docs/peta-register.md`.

## Isi

| Paket | Isi |
|---|---|
| `cmd/gembok-verifier` | program utama dan opsi baris perintah |
| `internal/chip` | antarmuka chip, driver register (polling STATUS, kode galat, pembatas laju), backend `mmio` (`/dev/mem`, hanya Linux), dan backend `sim` |
| `internal/lms` | LMS dan LM-OTS menurut RFC 8554: pembangkitan kunci, tanda tangan stateful, pemeriksaan |
| `internal/issuer` | sertifikat: susunan byte kanonik, JSON, penerbitan, pemeriksaan |
| `internal/store` | direktori data: tulis atomik, kunci berkas, kunci penerbit, sertifikat, data bantu |
| `internal/verify` | alur pendaftaran, penyalaan, pemeriksaan, demo baca kunci, riwayat |
| `internal/httpapi` | API JSON dan dasbor tersemat (`internal/httpapi/web`) |

## Membangun

Dari folder `sw/verifier`:

| Perintah | Hasil |
|---|---|
| `make build` | `bin/gembok-verifier` untuk mesin ini |
| `make board` | `bin/gembok-verifier-arm` untuk ARM papan (GOOS=linux GOARCH=arm GOARM=7) |
| `make check` | go vet, go test, dan uji kompilasi silang ARM |

Tanpa `make`:

```
go build -o bin/gembok-verifier ./cmd/gembok-verifier
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o bin/gembok-verifier-arm ./cmd/gembok-verifier
```

Hasil bangun dasbor sudah disimpan di `internal/httpapi/web`, jadi membangun layanan tidak butuh Node. Untuk memperbarui dasbor setelah mengubah `sw/dashboard`:

```
make dashboard
```

Perintah itu menjalankan `npm ci`, `npm run check`, `npm run build`, lalu menyalin hasilnya ke `internal/httpapi/web`.

## Menjalankan simulasi di laptop

```
make run
```

atau

```
go run ./cmd/gembok-verifier --backend sim
```

Buka `http://127.0.0.1:8080`. Saat pertama kali jalan, layanan membuat kunci penerbit LMS (1024 daun). Di mesin x86 tempat layanan ini diuji, langkah itu sekitar setengah detik.

Opsi:

| Opsi | Bawaan | Isi |
|---|---|---|
| `--backend` | `sim` | `sim` atau `mmio` |
| `--base` | `0xFF240000` | alamat fisik IP GEMBOK untuk `mmio`: jembatan ringan HPS-ke-FPGA `0xFF200000` ditambah alamat dasar IP `0x40000` dari `docs/panduan-papan.md`. Harus kelipatan ukuran halaman |
| `--data` | `data` | direktori data |
| `--listen` | `127.0.0.1:8080` | alamat dengar HTTP |
| `--k` | `3` | parameter ML-KEM untuk pendaftaran: 3 (ML-KEM-768) atau 4 (ML-KEM-1024) |
| `--ambang` | `64` | ambang PUF untuk pendaftaran bila permintaan tidak menyebutkan `ambang`, 1 sampai 65535. Isi dengan hasil karakterisasi papan |
| `--coba-nyala` | `5` | berapa kali PUF_RECON dicoba sebelum pemeriksa menyimpulkan chip tidak bisa memulihkan kunci, 1 sampai 20 |

Urutan demo dari baris perintah:

```
B=http://127.0.0.1:8080
J='Content-Type: application/json'
curl -X POST -H "$J" -d '{"id_chip":"GEMBOK-DEMO-01"}' $B/api/daftar
curl -X POST -H "$J" -d '{}' $B/api/nyalakan
curl -X POST -H "$J" -d '{"konteks":"scan-001"}' $B/api/periksa
curl -X POST -H "$J" -d '{"chip":"tiruan"}' $B/api/simulasi/chip
curl -X POST -H "$J" -d '{}' $B/api/periksa
curl -X POST -H "$J" -d '{"chip":"asli"}' $B/api/simulasi/chip
curl -X POST -H "$J" -d '{}' $B/api/serang/baca-kunci
curl $B/api/riwayat
```

## Menjalankan di papan DE10-Nano

1. Bangun di laptop: `make board`. Hasilnya `bin/gembok-verifier-arm`, biner statis yang tidak butuh pustaka apa pun di papan.
2. Salin ke papan dengan nama `gembok-verifier`, nama yang dipakai `docs/panduan-papan.md`:

   ```
   scp bin/gembok-verifier-arm root@<alamat papan>:/home/root/gembok-verifier
   ssh root@<alamat papan> chmod 755 /home/root/gembok-verifier
   ```

   Bila menyalin lewat kartu microSD atau flashdisk, ganti nama berkasnya menjadi `gembok-verifier` dan jalankan `chmod 755` yang sama di papan, karena partisi FAT tidak menyimpan izin eksekusi.
3. Pastikan bitstream GEMBOK sudah dimuat. Alamat fisik IP = `0xFF200000` (jembatan ringan HPS-ke-FPGA) ditambah alamat dasar `s0` di Platform Designer. Panduan papan memakai alamat dasar `0x0004_0000`, jadi alamat fisiknya `0xFF240000`, sama dengan nilai bawaan `--base`.
4. Jalankan sebagai root di papan (butuh `/dev/mem`):

   ```
   cd /home/root
   ./gembok-verifier --backend mmio --base 0xFF240000 --data /home/root/gembok --listen 0.0.0.0:8080
   ```

Bila karakterisasi PUF (langkah 5 panduan papan) menghasilkan ambang selain 64, tambahkan `--ambang <nilai>` supaya pendaftaran dari dasbor memakai nilai itu tanpa harus mengisinya tiap kali.

Saat dibuka, driver memeriksa register ID (`0x47454D42`) dan versi mayor 1, lalu membaca CAPS dan menunggu STATUS.BOOTED. Bila ID tidak cocok, jumlah osilator di CAPS di luar 2 sampai 1025, atau WIN_LOG2 di luar 1 sampai 20, layanan berhenti dengan pesan galat; periksa `--base` dan bitstream. Log awal menampilkan isi CAPS (`osilator`, `suara`, `jendela_log2`) dan batas waktu yang dihitung darinya (`perintah_puf`, `permintaan`).

Bila CAPS melaporkan PUF_MODE 1 (model simulasi), PUF_MODE 2 (kunci pengembangan tetap), mode 3, atau bangunan debug, layanan menulis peringatan ke log saat mulai, `GET /api/status` mengisi `peringatan`, dan dasbor menampilkan peringatan merah di atas halaman. Pada mode itu kunci chip bukan rahasia: papan lain dengan bitstream yang sama menghasilkan kunci yang sama, jadi hasil ASLI tidak membuktikan apa pun. Mode itu hanya untuk bring-up. Demo memakai `PUF_MODE = 0` dan `PUF_DEBUG = 0`.

`--listen 0.0.0.0:8080` membuat dasbor bisa dibuka dari ponsel di jaringan yang sama. Layanan tidak punya autentikasi, jadi pakai hanya di jaringan demo tertutup. Layanan menulis peringatan ke log bila mendengar di luar loopback.

Pembangkitan kunci penerbit di CPU papan (Cortex-A9) jauh lebih lambat daripada di laptop dan belum diukur. Kunci baru hanya dibuat sekali; sesudahnya pohon Merkle dimuat dari cache `penerbit/pohon.bin`.

## Berkas data

Direktori `--data` dibuat dengan izin 0700:

| Berkas | Isi |
|---|---|
| `penerbit/kunci-publik.json` | kunci publik LMS penerbit, dipakai pemeriksa sebagai titik kepercayaan |
| `penerbit/kunci-rahasia.json` | benih, pengenal I, dan indeks daun berikutnya (izin 0600) |
| `penerbit/pohon.bin` | cache 1024 nilai daun (publik); dibangun ulang dari benih bila hilang atau tidak cocok dengan kunci publik |
| `chip/sertifikat.json` | sertifikat chip terdaftar |
| `chip/data-bantu.json` | data bantu PUF (publik): versi 2, ambang, panjang topeng (`panjang_topeng`), topeng, nilai cek. Berkas versi 1 (tanpa panjang, topeng 96 byte) masih terbaca |
| `proses.lock` | kunci berkas; proses kedua pada direktori yang sama ditolak |

Kunci LMS bersifat stateful. Indeks daun dicatat ke cakram (berkas sementara, fsync, rename, fsync direktori) sebelum tanda tangan dibuat, dan tanda tangan diperiksa ulang sebelum dilepas. Jangan menyalin `kunci-rahasia.json` ke mesin lain yang juga menandatangani, dan jangan memulihkannya dari cadangan lama: indeks daun yang sudah terpakai bisa terpakai lagi, dan itu membuka jalan pemalsuan sertifikat.

Untuk mengulang dari awal, hentikan layanan lalu hapus direktori data. Kunci penerbit baru akan dibuat, dan sertifikat lama tidak lagi sah.

## Backend sim

Backend `sim` meniru chip seperti yang terlihat dari host, pada tingkat register. Driver yang sama dipakai untuk `sim` dan `mmio`, jadi urutan register yang berjalan di papan juga teruji di laptop.

- Register ID, VERSION, CTRL, STATUS, PARAM, CTXLEN, CYCLES, CAPS, COOLDOWN, PROOFS, PUF_THRESH, dan jendela memori byte di `0x8000 + 4*i`. PUF_DBG (`0x30`, osilator c) dan PUF_DBG1 (`0x34`, osilator c + 1) terbaca 0 seperti bangunan tanpa debug; di bangunan debug RTL keduanya berisi hitungan 20 bit.
- CAPS = `N_RO << 16 | VOTES << 8 | WIN_LOG2 << 3 | mode`, dengan N_RO bawaan 768, VOTES 5, dan WIN_LOG2 12 (jendela 4096 siklus, jendela yang juga dipakai taksiran siklus PUF_ENROLL dan PUF_RECON di bawah). Chip model terbaca `0x03000561`. Jumlah osilator bisa diatur dari 2 sampai 1025 lewat `SimConfig`; uji memakai 768, 801, dan 1025.
- BUSY ditiru dengan waktu nyata dan naik pada tulis CTRL itu sendiri, seperti `cmd_pend` di `rtl/avmm_bridge.sv`. Sejak itu sampai perintah selesai, baca memori menghasilkan 0 dan semua tulis register maupun memori diabaikan, termasuk tulis pada siklus tepat sesudah CTRL. PARAM dan CTXLEN dikunci saat perintah mulai. Perintah yang langsung ditolak tetap melewati BUSY = 1 dan DONE = 0 selama 2 siklus, lalu selesai dengan CYCLES = 1.
- Kunci dari PUF: `(d, z) = SHAKE256("GEMBOK-v1/benih" || kunci)`, lalu kunci ML-KEM dari benih `d || z` (`mlkem.NewDecapsulationKey768` atau `1024`).
- Data bantu: `HELPER_CHK = SHAKE256("GEMBOK-v1/cek" || kunci || topeng)[0:16]`. Topeng panjangnya `(N_RO + 6) / 8` byte mulai `0x1A00` (96 byte untuk 768 osilator, paling banyak 128 byte untuk 1025), nilai cek 16 byte tetap di `0x1A80`. PUF_RECON menjawab `BAD_HELPER` bila data bantu bukan milik kunci chip ini, dan `PUF_FAIL` bila topeng tidak berisi tepat 256 pasangan.
- PUF model: hitungan osilator dari rumus `tb/common/puf_model.py` (sama dengan `rtl/puf_ro.sv` MODE 1 tanpa derau), pemilihan pasangan seperti `rtl/fuzzy_extractor.sv`. Chip `asli` memakai benih 1, chip `tiruan` benih 2. Ada juga PUF kunci tetap (seperti PUF_MODE 2) untuk uji.
- Sesudah ENROLL dan PROVE, slot D, Z, M, K, SIGMA, KBAR (0x1800 sampai 0x18BF) bernilai nol. Wilayah DK tidak pernah diisi pada mode brankas.
- Pembatas laju: setelah PROVE selesai, PROVE berikutnya ditolak dengan `RATE` selama 1.048.576 siklus (sekitar 21 ms), sama dengan nilai bawaan RTL.
- Perintah mode terbuka (KEYGEN, ENCAPS, DECAPS, CHECK_EK, CHECK_DK) dan PUF_MEASURE dijawab `BAD_CMD`, seperti bangunan yang tidak menyediakannya. Pustaka standar Go tidak membuka Encaps deterministik, jadi mode terbuka tidak ditiru.

Jumlah siklus adalah angka tetap dan ditandai `siklus_simulasi: true` di API: PROVE 45.400 (k = 3) dan 65.300 (k = 4), ENROLL 18.100 dan 26.900. PUF_ENROLL 6.000.000 dan PUF_RECON 5.300.000 adalah taksiran kasar dari struktur RTL (jendela 4096 siklus, 5 suara), bukan hasil simulasi RTL.

## Catatan driver

- Tiap perintah: tunggu BUSY = 0 dan BOOTED = 1, tulis PARAM, CTXLEN, dan masukan, tulis CTRL, lalu tunggu DONE = 1 dan BUSY = 0, baca STATUS.ERR dan CYCLES.
- Mulai siklus clock sesudah CTRL ditulis, STATUS melaporkan BUSY = 1 dan DONE = 0 (logika `cmd_pend` di `rtl/avmm_bridge.sv`), juga untuk perintah yang langsung ditolak. Perintah yang ditolak selesai dalam beberapa siklus. Jadi STATUS pertama yang dibaca driver sudah milik perintah baru.
- Driver tetap membaca register ID satu kali sesudah CTRL sebelum mulai membaca STATUS. Dengan RTL sekarang bacaan itu tidak diperlukan; ia pengaman yang tidak merugikan, misalnya untuk bitstream lama yang belum punya `cmd_pend`, dan biayanya hanya satu akses bus.
- Panjang topeng data bantu dibaca dari CAPS (`N_RO` di bit 27..16, panjang `(N_RO + 6) / 8` byte), bukan angka tetap. PUF_ENROLL membaca topeng sepanjang itu, dan PUF_RECON menolak data bantu yang panjangnya berbeda sebelum menulis apa pun ke chip.
- CAPS (`0x1C`): bit 1..0 mode PUF, bit 2 bangunan debug, bit 7..3 WIN_LOG2 (jendela ukur 2^WIN_LOG2 siklus), bit 15..8 VOTES, bit 27..16 N_RO, bit 31..28 nol. Bangunan PUF_MODE 0 dengan nilai bawaan terbaca `0x03000560`.
- Polling STATUS berputar tanpa tidur selama 3 ms pertama, lalu tidur 200 mikrodetik per putaran.
- Batas waktu ENROLL, PROVE, dan WIPE 5 detik. PUF_ENROLL dan PUF_RECON memakai `maks(5 s, 2 x B / 50 MHz + 1 s)` dengan `B = (N_RO - 1) x (VOTES x (2^WIN_LOG2 + 32) + 16) + 100000` siklus, dihitung dengan bilangan bulat 64 bit dari CAPS. Parameter bawaan (768, 5, 12) memberi 5 detik (rumusnya 1,64 detik). Parameter terlama yang diizinkan perangkat keras (1025, 15, 16) memberi 41,29 detik, sedangkan PUF_RECON sebenarnya sekitar 5 detik dan PUF_ENROLL sekitar 20 detik.
- Sebelum tiap perintah, driver menunggu BUSY turun dengan batas yang lebih panjang dari keduanya, jadi perintah PUF lama yang masih berjalan tidak dianggap macet.
- `RATE`: driver membaca COOLDOWN, menunggu `COOLDOWN x 20 ns` ditambah 100 mikrodetik, lalu mengulang, paling banyak tiga kali.
- Akses register memakai satu baca atau tulis 32 bit per alamat. Pada ARM keduanya terkompilasi menjadi satu instruksi `ldr` atau `str`.

## Penyalaan dan percobaan ulang PUF_RECON

Pengukuran osilator berderau, jadi chip asli kadang gagal memulihkan kunci pada satu pengukuran. Karena itu pemeriksa mengulang PUF_RECON, baik pada penyalaan otomatis sebelum pemeriksaan maupun pada `POST /api/nyalakan`:

- Diulang sampai `--coba-nyala` kali (bawaan 5) selama chip menjawab `BAD_HELPER` atau `PUF_FAIL`. Tiap percobaan menulis ulang ambang, topeng, dan nilai cek, lalu mengukur osilator dari awal.
- Kode chip lain dan galat bus tidak diulang.
- Sebelum percobaan pertama, panjang topeng tersimpan dibandingkan dengan panjang dari CAPS. Bila berbeda, PUF_RECON tidak dikirim sama sekali dan API menjawab `409 TOPENG_TIDAK_SESUAI`.
- Chip tiruan gagal di semua percobaan, jadi hasilnya tetap PALSU (`DATA_BANTU_DITOLAK`) setelah N percobaan. Jumlah percobaan dilaporkan di `percobaan_nyala` (hasil pemeriksaan) dan `percobaan` (hasil penyalaan).

Pada papan dengan parameter bawaan, satu PUF_RECON sekitar 106 ms (taksiran 5,3 juta siklus pada 50 MHz), jadi lima percobaan yang semuanya gagal butuh sekitar setengah detik. Dengan WIN_LOG2 16 dan VOTES 15, satu PUF_RECON sekitar 5 detik, sehingga lima percobaan chip tiruan sekitar 25 detik. Karena itu batas waktu tiap permintaan HTTP ikut dihitung dari CAPS saat layanan mulai: `maks(20 detik, --coba-nyala x batas PUF + 15 detik)`, yaitu 40 detik untuk parameter bawaan dan sekitar 221 detik untuk (1025, 15, 16). Batas tulis server HTTP 25 detik lebih panjang dari itu.

## Pengujian

```
go vet ./...
go test ./...
```

Yang diuji:

- LMS terhadap vektor RFC 8554 Lampiran F (Test Case 1 dan 2, di `internal/lms/testdata`): pemeriksaan kedua tingkat HSS, pembangkitan kunci dari SEED dan I Test Case 2 (akar pohon sama dengan kunci publik vektor), dan tanda tangan yang dibuat ulang sama persis byte demi byte. Juga ribuan tanda tangan yang diubah satu bit, semuanya ditolak.
- Tanda tangan stateful: tiap daun dipakai sekali, berhenti setelah daun habis, tidak ada tanda tangan bila indeks gagal disimpan atau sumber acak mati, aman dipakai serentak.
- Sertifikat: susunan byte kanonik terhadap nilai tetap, pembacaan balik, JSON, dan setiap ruas yang diubah ditolak.
- Backend sim terhadap fixture dari model acuan Python (`internal/chip/testdata/fixture_pemeriksa.json`): benih (d, z), nilai cek data bantu, ek, H(ek), dan tag untuk tiap pasangan (c, konteks), untuk tiga kunci PUF tetap dan tujuh chip model PUF (benih 1, 2, 3, dan 12648430, ambang 64 sampai 900, 768, 801, dan 1025 osilator), k = 3 dan 4.
- Perilaku register seperti uji RTL `tb/top/test_top.py`: register setelah boot, kode galat, penjaga akses selama PROVE, pembatas laju, WIPE, chip tiruan, data bantu yang diubah.
- Alur pemeriksaan: chip asli ASLI, chip tiruan PALSU (dua cara), bukti lama diputar ulang PALSU, sertifikat diubah ditolak tanpa menanya chip, jalur pembatas laju.
- Percobaan ulang PUF_RECON: chip asli yang gagal sekali lalu berhasil tetap ASLI (`percobaan_nyala` 2), chip yang baru pulih pada percobaan terakhir tetap ASLI, chip tiruan PALSU setelah tepat N PUF_RECON tanpa satu pun PROVE, kode lain dan galat bus tidak diulang.
- Panjang topeng: CAPS dengan 768, 801, dan 1025 osilator, topeng 128 byte, data bantu yang panjangnya tidak sama ditolak sebelum PUF_RECON, berkas data bantu versi 1 dan 2.
- Peringatan mode PUF untuk tiap kombinasi mode dan debug pada backend `mmio`, dan tidak ada peringatan pada `sim`. Ambang bawaan dari `--ambang`.
- CAPS dan batas waktu: pembacaan CAPS dengan WIN_LOG2 1, 4, 12, 16, dan 20, penolakan WIN_LOG2 0, 21, dan 31, batas PUF untuk parameter bawaan (5 detik) dan untuk (1025, 15, 16) (41,29 detik), batas permintaan HTTP. Dengan jam tiruan (tanpa tidur sungguhan): PUF_RECON 6 detik dan PUF_ENROLL 20 detik pada (1025, 15, 16) selesai tanpa dipotong, PUF_RECON 45 detik berhenti di batas 41,29 detik, PUF_RECON 6 detik pada parameter bawaan dan ENROLL 6 detik tetap berhenti di 5 detik, dan menunggu chip menganggur ikut menunggu perintah PUF lama yang masih berjalan.
- API HTTP dengan `httptest`: alur demo lengkap, validasi masukan, penolakan lintas situs dan host asing, pemetaan galat, berkas statis.

Fixture dibuat ulang dengan:

```
make fixture
```

Perintah itu menjalankan `python3 tools/gen_fixture_pemeriksa.py` dari akar repositori.

Hasilnya deterministik; berkas yang sama akan tertulis lagi bila model acuan tidak berubah.

## Batasan

- Hanya k = 3 dan k = 4. Pustaka standar Go tidak menyediakan ML-KEM-512, jadi k = 2 tidak didukung walaupun perangkat keras mendukungnya.
- Kunci penerbit LMS hanya bisa membuat 1024 tanda tangan (satu per pendaftaran). Setelah habis, buat direktori data baru.
- Jumlah siklus pada backend `sim` adalah angka tetap, bukan hasil pengukuran.
- Penerbit dan pemeriksa berjalan di satu proses untuk keperluan demo. Pada pemakaian sebenarnya pemeriksa hanya membawa `kunci-publik.json`.
- Implementasi LMS ini lolos vektor RFC 8554, tetapi bukan modul kriptografi tervalidasi. NIST SP 800-208 mensyaratkan pembangkitan kunci dan penandatanganan di modul perangkat keras.
- Tidak ada autentikasi HTTP. Riwayat hanya di memori.
