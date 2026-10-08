# Panduan langkah di papan DE10-Nano

Semua yang bisa dikerjakan tanpa papan sudah selesai dan teruji di simulasi. Dokumen ini berisi langkah yang harus dikerjakan di laptop Windows (Quartus) dan di papan. Urutannya disusun supaya tiap langkah bisa diperiksa sebelum lanjut.

Yang dibutuhkan:

- Intel Quartus Prime Lite dengan dukungan Cyclone V, di laptop Windows. Versi 18.1 paling aman karena sama dengan proyek acuan Terasic.
- Papan DE10-Nano, kartu microSD berisi image Linux dari Terasic, kabel USB atau Ethernet ke laptop.
- DE10-Nano System CD dari Terasic, yang berisi proyek acuan HPS (GHRD, `DE10_NANO_SoC_GHRD`).
- Untuk membangun perangkat lunak di laptop: WSL (Ubuntu) dengan `make`, `gcc-arm-linux-gnueabihf`, `python3`, dan Go 1.24. Bisa juga dibangun langsung di papan, lihat Langkah 3.

Catatan versi Quartus: proyek di repo ini ditandai versi 18.1. Versi yang lebih baru akan menawarkan peningkatan proyek; terima saja. Bila System CD lebih tua daripada Quartus yang terpasang, Platform Designer akan meminta peningkatan IP HPS di GHRD (menu Tools, IP Upgrade); kerjakan sebelum menambah komponen GEMBOK.

Perintah Quartus di bawah memakai menu GUI. Skrip `quartus/kompilasi.sh` hanya untuk Linux, WSL, atau Git Bash yang punya Quartus di PATH.

## Langkah 1. Kompilasi mandiri untuk angka resource dan timing

Tujuannya mendapat angka resmi dari Quartus untuk proposal dan laporan, tanpa HPS.

1. Salin seluruh folder repo ke laptop Windows.
2. Buka `quartus/gembok.qpf` di Quartus. Periksa pilihan revisi di toolbar (atau menu Project, Revisions): yang aktif harus `gembok`. Revisi `gembok` memakai kunci pengembangan (`PUF_MODE = 2`), jadi tidak ada osilator dan kompilasinya paling sederhana.
3. Jalankan Processing, Start Compilation. Dari WSL atau Git Bash bisa juga `bash quartus/kompilasi.sh gembok`, yang mencetak ringkasan fitter dan timing.
4. Catat dari laporan:
   - Fitter, Resource Usage Summary: jumlah ALM, register, blok M10K, blok DSP.
   - Fitter, RAM Summary: `u_pmem` (2048 x 24) dan `u_bmem` (8192 x 8) harus tercatat sebagai M10K. Bila salah satunya dibangun dari register, kirim laporannya.
   - Timing Analyzer: slack setup terburuk untuk `clk50` (20 ns). Slack positif berarti 50 MHz terpenuhi.
5. Pindah ke revisi `gembok_puf` (`PUF_MODE = 0`, osilator asli) lewat pilihan revisi, lalu kompilasi lagi. Selisih resource-nya adalah biaya PUF.
6. Di revisi `gembok_puf`, buka Timing Analyzer, Report Clocks. Harus ada `clk50`, `puf_osc_a`, `puf_osc_b`, `puf_half_a`, dan `puf_half_b`, dan slack keempat jam PUF harus positif. Artinya pencacah osilator sanggup mengikuti osilator sampai 400 MHz. Bila jam PUF tidak muncul dan ada peringatan "Ignored filter" dari `gembok_puf.sdc`, kirim teks peringatannya. Kompilasi tetap bisa dipakai, hanya pencacahnya tidak diperiksa.

Perkiraan dari Yosys (bukan Quartus) ada di `docs/arsitektur.md` Bagian 13 dan bisa diulang dengan `tools/sintesis_yosys.sh`. Bila angka Quartus jauh berbeda, angka Quartus yang dipakai.

Kalau ada galat kompilasi, kirim isi pesan galatnya (jendela Messages, atau berkas `quartus/output_files/*.map.rpt` dan `*.fit.rpt`) supaya bisa diperbaiki.

Peringatan yang wajar di revisi `gembok_puf`: combinational loop (itulah osilatornya) dan ripple clock (pencacah yang diclock flip-flop pembagi dua). Jalur antara pencacah osilator dan domain `clk50` sudah ditandai false path di `quartus/gembok_puf.sdc`, karena pencacah dihapus sebelum osilator menyala dan hanya dibaca setelah osilator berhenti.

## Langkah 2. Pasang IP ke sistem HPS

1. Salin proyek GHRD dari System CD ke folder kerja, misalnya `DE10_NANO_SoC_GHRD`.
2. Salin folder `rtl/` serta berkas `quartus/gembok_hw.tcl` dan `quartus/gembok_puf.sdc` dari repo ini ke folder `ip/gembok/` di proyek GHRD, sehingga strukturnya:
   - `ip/gembok/gembok_hw.tcl`
   - `ip/gembok/gembok_puf.sdc`
   - `ip/gembok/rtl/*.sv`
3. Buka `soc_system.qsys` di Platform Designer. Komponen "GEMBOK ML-KEM identity core" muncul di IP Catalog, grup GEMBOK. Bila tidak muncul, tambahkan `ip/**/*` ke IP Search Path (Tools, Options).
4. Tambahkan komponen itu, lalu hubungkan:
   - `clock` ke sumber clock 50 MHz yang dipakai GHRD (`clk_0.clk`).
   - `reset` ke reset sistem (`clk_0.clk_reset`, dan reset dari HPS bila GHRD memakainya).
   - `s0` ke `hps_0.h2f_lw_axi_master`.
   - `led` boleh diekspor lalu dibiarkan tidak terhubung, atau dihubungkan ke LED bila LED tidak dipakai komponen lain.
5. Beri alamat dasar `s0` = `0x0004_0000`. Rentangnya 64 KB. Pastikan tidak tumpang tindih dengan komponen GHRD lain (Platform Designer akan memperingatkan bila bentrok).
6. Parameter komponen: untuk bring-up pertama pakai `PUF_MODE = 2`. Setelah Langkah 4 berhasil, ganti ke `PUF_MODE = 0`. Komponen hanya menerima nilai yang aman: `PUF_MODE` 0 atau 2, `RO_STAGES` ganjil 3 sampai 13, `VOTES` ganjil 1 sampai 15, `N_RO` 600 sampai 1025, `WIN_LOG2` 4 sampai 16.
7. Generate HDL, kembali ke Quartus, lalu kompilasi proyek GHRD. Batasan timing osilator (`gembok_puf.sdc`) ikut terbawa oleh komponen. Pada `PUF_MODE = 2` berkas itu hanya memberi peringatan "Ignored filter", karena osilatornya tidak ada.
8. Ubah `.sof` menjadi `.rbf` untuk dimuat Linux saat boot. Lewat GUI: File, Convert Programming Files. Pilih Programming file type Raw Binary File (.rbf), Mode Passive Parallel x16, nama berkas `soc_system.rbf`. Di Input files to convert, pilih SOF Data, Add File, lalu pilih `output_files/DE10_NANO_SoC_GHRD.sof`. Pilih berkas `.sof` itu, klik Properties, centang Compression. Klik Generate.
   Lewat Command Prompt, `quartus_cpf` biasanya tidak ada di PATH, jadi pakai jalur lengkapnya (sesuaikan dengan folder instalasi):
   `C:\intelFPGA_lite\18.1\quartus\bin64\quartus_cpf -c -o bitstream_compression=on output_files\DE10_NANO_SoC_GHRD.sof soc_system.rbf`
9. Salin `soc_system.rbf` ke partisi FAT di microSD (menggantikan berkas dengan nama yang sama). Image Terasic memuat berkas ini dan mengaktifkan jembatan HPS-FPGA saat boot. Bila image Anda berbeda, jembatan lightweight bisa diaktifkan dari Linux dengan `echo 1 > /sys/class/fpga-bridge/lwhps2fpga/enable` (nama berkas bisa berbeda per image).

Alamat fisik IP dari sisi HPS = `0xFF200000 + 0x40000 = 0xFF240000`. Ini alamat bawaan `gembok-cli` dan `gembok-verifier`. Bila IP dipasang di alamat dasar lain, berikan alamat fisiknya dengan `--base`.

## Langkah 3. Bangun perangkat lunak untuk papan

Driver dan program uji C (`sw/hps`, rincian di `sw/hps/README.md`):

- Di laptop (WSL): `make CC=arm-linux-gnueabihf-gcc CFLAGS="-O2 -static"`. Penautan statis perlu karena glibc di image papan biasanya lebih tua daripada glibc toolchain di laptop.
- Langsung di papan: `make`. Bila gcc di papan lebih tua dan memberi galat karena `-Werror`, pakai `make WARN="-Wall -Wextra"`.
- Hasilnya `build/gembok-cli`.
- Vektor uji untuk papan: `make vectors` membuat `build/acvp.txt`, berkas teks yang dibaca `gembok-cli acvp`.
- Bila dibangun di laptop, salin keduanya ke papan:
  `scp build/gembok-cli build/acvp.txt root@<alamat papan>:/home/root/`

Layanan pemeriksa dan dasbor (`sw/verifier`, rincian di `sw/verifier/README.md`):

- `make board` menghasilkan biner ARM statis `bin/gembok-verifier-arm`.
- Salin ke papan dengan nama `gembok-verifier`:
  `scp bin/gembok-verifier-arm root@<alamat papan>:/home/root/gembok-verifier`
  lalu di papan `chmod 755 /home/root/gembok-verifier`.

## Langkah 4. Uji fungsi di papan (dengan PUF_MODE = 2)

Jalankan sebagai root di papan:

1. `./gembok-cli info`
   Harus menampilkan ID `GEMB`, versi `1.0.0`, mode PUF 2, dan status `BOOTED`. Bila BOOTED tetap 0, periksa sambungan `reset` di Platform Designer: IP menjalankan penghapusan memori sesudah reset dan baru menerima perintah setelah BOOTED = 1.
2. `./gembok-cli selftest`
   Menjalankan KeyGen, Encaps, Decaps untuk k = 2, 3, 4, lalu DAFTAR dan BUKTIKAN. Harus berakhir LULUS dan menampilkan ASLI.
3. `./gembok-cli acvp acvp.txt --jumlah 240`
   Harus 240 dari 240 kasus lolos.

Catat jumlah siklus yang ditampilkan. Untuk DAFTAR dan BUKTIKAN angkanya harus sama persis dengan simulasi, karena kunci pengembangan dan konteks `selftest` tetap: DAFTAR 11.294, 18.110, 26.884 siklus dan BUKTIKAN 29.440, 45.380, 65.322 siklus untuk k = 2, 3, 4. Jumlah siklus tidak bergantung pada kecepatan bus HPS. Angka KeyGen, Encaps, dan Decaps sedikit berubah tiap kali `selftest` dijalankan, karena benihnya acak. Waktu total yang dilihat aplikasi lebih lama karena akses bus dari HPS.

Bila ada yang gagal di langkah ini, PUF belum terlibat, jadi masalahnya ada di integrasi (alamat dasar, jembatan, reset) atau di RTL. Kirim keluaran lengkap `info` dan `selftest`.

## Langkah 5. Karakterisasi PUF

Langkah ini menentukan apakah PUF cukup stabil dan unik. Hanya bisa dikerjakan di papan.

Hal penting sebelum mulai: penempatan osilator ditentukan Quartus setiap kali kompilasi. Bangunan debug dan bangunan akhir adalah dua kompilasi berbeda, jadi frekuensi tiap osilator di keduanya tidak sama. Data dari bangunan debug dipakai untuk statistik (frekuensi rata-rata, derau, sebaran selisih, pilihan ambang), bukan untuk nilai per pasangan. Pendaftaran, uji kestabilan, dan demo memakai bangunan akhir yang sama persis.

### 5.1 Bangunan debug

Kompilasi ulang dengan `PUF_MODE = 0` dan `PUF_DEBUG = 1`. Bangunan debug membuka perintah PUF_MEASURE yang membaca hitungan mentah osilator. Jangan pakai bangunan ini untuk demo, karena hitungan mentah sama dengan membocorkan kunci. Dasbor menampilkan peringatan merah bila bangunan debug sedang dimuat.

### 5.2 Ukur semua pasangan

Ukur semua pasangan berulang kali, misalnya 20 putaran:

```
for i in $(seq 20); do ./gembok-cli puf-measure --idx 0 --jumlah 767; done > ukur.txt
```

Tiap baris berisi nomor pasangan, hitungan osilator c dan c+1 (20 bit) dalam satu jendela ukur, dan selisihnya. Opsi persisnya ada di `sw/hps/README.md`.

### 5.3 Periksa hitungan

Dengan `WIN_LOG2 = 12` lama jendela 2^12 siklus = 81,92 mikrodetik, jadi frekuensi = hitungan / 81,92 mikrodetik.

- Hitungan wajar berada di kisaran ribuan sampai puluhan ribu (puluhan sampai ratusan MHz).
- Hitungan harus di bawah 32.768 (400 MHz), karena batasan timing di `gembok_puf.sdc` memeriksa pencacah sampai 400 MHz. Bila lebih, ubah periode `puf_osc_a` dan `puf_osc_b` di `ip/gembok/gembok_puf.sdc` menjadi sedikit di bawah 1/frekuensi terbesar (periode `puf_half_*` dua kalinya), jalankan Generate HDL lagi di Platform Designer (berkas SDC disalin saat itu), kompilasi ulang, dan pastikan slack tetap positif. Cara lain: naikkan `RO_STAGES` (ganjil).
- Hitungan yang sangat kecil (di bawah sekitar 1.000) atau nol berarti osilator tidak berosilasi. Periksa bahwa `RO_STAGES` ganjil dan kirim peringatan Quartus.
- Pencacah 20 bit tidak meluap sampai 1.048.575, jadi `WIN_LOG2` boleh dinaikkan sampai 16 bila selisih antarpasangan terlalu kecil.

### 5.4 Pilih ambang

Dari data itu hitung:

- sebaran selisih tiap pasangan dan derau antar-pengukuran,
- berapa pasangan yang selisihnya selalu di atas ambang (bawaan 64).

Pilih ambang (`PUF_THRESH`) sehingga minimal 256 pasangan terpilih dengan margin jauh di atas derau. Ambang bisa dicoba tanpa kompilasi ulang: `./gembok-cli puf-enroll --thresh N --helper coba.txt` berakhir PUF_FAIL bila pasangan andal kurang dari 256. Bila pasangan andal terlalu sedikit, naikkan `N_RO` (paling banyak 1025) atau `WIN_LOG2`.

### 5.5 Bangunan akhir dan uji kestabilan

1. Kompilasi dengan `PUF_MODE = 0` dan `PUF_DEBUG = 0`. Simpan salinan `soc_system.rbf` ini dengan nama yang jelas, misalnya `soc_system_final_YYYYMMDD.rbf`. Mulai saat ini bitstream dibekukan: kompilasi ulang apa pun (parameter, perubahan GHRD, versi Quartus lain) bisa memindahkan osilator, dan data bantu serta sertifikat lama tidak berlaku lagi. Bila terpaksa kompilasi ulang, daftarkan ulang chip.
2. Daftarkan sekali, dengan ambang dari 5.4:
   ```
   ./gembok-cli puf-enroll --thresh N --helper bantu.txt + enroll -k 3 --ek ek0.hex
   ```
3. Pemulihan berulang tanpa mematikan papan, 100 kali:
   ```
   ok=0; for i in $(seq 100); do ./gembok-cli wipe + puf-recon --helper bantu.txt + enroll -k 3 --ek ek.hex && cmp -s ek0.hex ek.hex && ok=$((ok+1)); done; echo "$ok dari 100"
   ```
4. Penyalaan dingin, sebanyak yang sempat (misalnya 10 kali): matikan papan sekitar satu menit, nyalakan, lalu:
   ```
   ./gembok-cli puf-recon --helper bantu.txt + enroll -k 3 --ek ek.hex && cmp ek0.hex ek.hex && echo SAMA
   ```
   Bila bisa, ulangi beberapa kali pada suhu ruangan yang berbeda.

Satu `puf-recon` yang gagal tidak berarti chip palsu: pemeriksa (`gembok-verifier`) mengulang PUF_RECON sampai 5 kali. Yang penting dicatat adalah tingkat keberhasilan per percobaan, dan bahwa EK yang dipulihkan tidak pernah berbeda dari `ek0.hex`.

### 5.6 Uji keunikan dengan papan kedua

Langkah ini sangat disarankan, karena hanya ini yang membuktikan PUF berbeda dari chip ke chip. Pakai papan kedua dengan `.rbf` akhir yang sama persis.

1. Di papan kedua, `./gembok-cli puf-recon --helper bantu.txt` (data bantu papan pertama) harus gagal (galat 3 BAD_HELPER atau galat 8 PUF_FAIL), juga bila diulang beberapa kali.
2. Daftarkan papan kedua dengan ambang yang sama. EK-nya harus berbeda dari `ek0.hex`.
3. Bila sempat, ukur kedua papan dengan bangunan debug yang sama, lalu bandingkan arah tiap pasangan (bit = hitungan c lebih besar dari c+1). Idealnya sekitar separuh bit berbeda. Bila hanya sedikit yang berbeda, selisih frekuensi didominasi rute yang sama di kedua papan (`docs/arsitektur.md` Bagian 10.5), dan tata letak osilator harus dikunci dulu sebelum hasilnya bisa disebut PUF.

Catat hasilnya untuk laporan: jumlah osilator, frekuensi rata-rata, ambang, jumlah pasangan terpilih, tingkat galat bit sebelum voting, keberhasilan pemulihan dari 100 percobaan dan dari penyalaan dingin, serta hasil uji papan kedua.

## Langkah 6. Demo

1. Muat bitstream akhir yang dibekukan di Langkah 5.5.
2. Di papan jalankan layanan pemeriksa, dengan ambang dari Langkah 5.4:
   `./gembok-verifier --backend mmio --base 0xFF240000 --data /home/root/gembok --listen 0.0.0.0:8080 --ambang N`
3. Buka dasbor dari laptop atau HP di `http://<alamat papan>:8080`. Bila bitstream yang dimuat bukan `PUF_MODE = 0` atau masih bangunan debug, dasbor menampilkan peringatan merah di atas halaman.
4. Adegan pertama: Daftarkan chip (sekali), lalu Periksa. Hasilnya ASLI beserta waktu proses chip. Ambang pendaftaran diambil dari `--ambang`, atau bisa diisi di bagian Pengaturan lanjutan pada dasbor.
5. Adegan kedua (chip tiruan):
   - Paling meyakinkan: papan kedua dengan `.rbf` akhir yang sama. Pindahkan kartu microSD atau jalankan layanan di papan kedua dengan direktori data yang sama, lalu Periksa. Hasilnya PALSU karena data bantu ditolak di setiap percobaan.
   - Tanpa papan kedua: muat bitstream hasil kompilasi dengan Fitter seed lain (`set_global_assignment -name SEED 2` di berkas QSF proyek GHRD), lalu Periksa. Hasilnya juga PALSU. Sampaikan dengan jujur bahwa adegan ini meniru "instansi PUF lain" (penempatan osilator berbeda), bukan salinan chip yang sama. Adegan ini menunjukkan bahwa data bantu dan sertifikat terikat pada satu PUF, tetapi tidak membuktikan keunikan antarchip.
   - Sesudah adegan ini, muat lagi bitstream akhir yang asli. Chip asli kembali ASLI, karena data bantu dan sertifikatnya masih tersimpan di layanan.
6. Adegan ketiga: tombol Coba baca kunci. Dasbor menampilkan isi memori rahasia yang terlihat host, semuanya nol.

Video demo 3 sampai 5 menit cukup merekam ketiga adegan ini ditambah keluaran `acvp` di terminal dan laporan resource dari Quartus.

## Yang perlu dikirim balik bila ada masalah

- Pesan galat Quartus (map, fit, atau timing), dan untuk revisi PUF laporan Report Clocks dari Timing Analyzer.
- Keluaran `gembok-cli info` dan `selftest`.
- Untuk PUF: berkas hasil `puf-measure` (`ukur.txt`) dan hasil uji kestabilan.

Dengan data itu RTL dan perangkat lunak bisa diperbaiki tanpa harus memegang papannya.
