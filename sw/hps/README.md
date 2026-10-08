# Perangkat lunak HPS untuk GEMBOK

Driver C dan alat baris perintah untuk IP GEMBOK (`rtl/gembok_top.sv`) di DE10-Nano, beserta pengujiannya terhadap RTL lewat Verilator. Kontrak antara perangkat keras dan perangkat lunak ada di `docs/peta-register.md`, langkah di papan ada di `docs/panduan-papan.md`.

Driver yang sama dipakai di papan dan di simulasi. Yang berbeda hanya sambungannya: di papan lewat `/dev/mem`, di simulasi lewat model Verilator dari RTL yang sebenarnya.

## Isi folder

| Berkas | Isi |
|---|---|
| `gembok.h`, `gembok.c` | Driver C11, hanya memakai libc. Akses bus lewat dua penunjuk fungsi (`read32(offset)`, `write32(offset, nilai)`) dan satu penunjuk konteks |
| `gembok_io.h` | Antarmuka sambungan: `gembok_io_open`, `gembok_io_close`, `gembok_io_name` |
| `io_devmem.c` | Sambungan papan: `mmap` dari `/dev/mem` pada alamat fisik dasar (bawaan `0xFF240000`), rentang 64 KB |
| `io_sim.cpp` | Sambungan simulasi: menjalankan model Verilator `gembok_top` dan transaksi Avalon-MM |
| `sha3.h`, `sha3.c` | SHA3-256 dan SHAKE256 kecil untuk sisi pemeriksa |
| `gembok_cli.c` | Alat baris perintah `gembok-cli` |
| `test_sha3.c` | Uji SHA3-256 dan SHAKE256 terhadap nilai dari `hashlib` Python |
| `test_driver.c` | Uji setiap fungsi driver, termasuk jalur galat, terhadap RTL (hanya simulasi) |
| `Makefile` | Bangun dan uji |
| `../../tools/acvp_to_txt.py` | Mengubah dua berkas vektor NIST ACVP (JSON) menjadi satu berkas teks untuk `gembok-cli acvp` |

## Membangun

### Untuk papan

```
make
```

`make` memakai `arm-linux-gnueabihf-gcc` bila ada di PATH, selain itu `cc`. Kompiler lain bisa dipilih dengan `make CC=...`. Hasilnya `build/gembok-cli`. Semua berkas C dikompilasi dengan `-std=c11 -Wall -Wextra -Werror -pedantic`.

Membangun silang di PC (Linux atau WSL), lalu menyalin hasilnya ke papan: glibc di image papan biasanya lebih tua daripada glibc toolchain di PC, sehingga program yang ditautkan dinamis bisa menolak jalan. Tautkan secara statis:

```
make CC=arm-linux-gnueabihf-gcc CFLAGS="-O2 -static"
```

Membangun langsung di papan: cukup `make` (yang dipakai `cc` milik papan). Bila gcc di papan lebih tua dan memberi peringatan yang dianggap galat oleh `-Werror`, buang `-Werror` dan `-pedantic` dengan:

```
make WARN="-Wall -Wextra"
```

Berkas vektor ACVP untuk dibawa ke papan dibuat dengan `make vectors` (butuh `python3`), hasilnya `build/acvp.txt`.

### Untuk simulasi

Butuh Verilator 5.020 atau lebih baru, `g++`, dan `python3`.

| Perintah | Hasil |
|---|---|
| `make sim` | `build/sim/gembok-sim` dan `build/sim/test-driver` |
| `make test` | seluruh uji terhadap RTL dengan `PUF_MODE=2` |
| `make test-puf` | uji yang sama terhadap RTL dengan model PUF simulasi, untuk 768 dan 1025 osilator |
| `make clean` | hapus folder `build` |

`build/sim/gembok-sim` adalah `gembok_cli.c` yang sama dengan alat untuk papan, hanya disambungkan ke model Verilator. Tiap kali dijalankan, simulasi mulai dari reset. Model dibangun ulang otomatis bila berkas di `rtl/` berubah.

## Menjalankan di papan

1. Muat bitstream yang berisi GEMBOK di jembatan lightweight HPS-to-FPGA (langkah 2 di `docs/panduan-papan.md`).
2. Salin `build/gembok-cli` dan `build/acvp.txt` ke papan.
3. Jalankan sebagai root (butuh akses `/dev/mem`):

```
./gembok-cli info
./gembok-cli selftest
./gembok-cli acvp acvp.txt --jumlah 240
```

Alamat fisik bawaan `0xFF240000` adalah `0xFF200000` (jembatan lightweight) ditambah alamat dasar IP `0x40000` di Platform Designer, sesuai panduan papan. Bila IP dipasang di alamat dasar lain, misalnya `0x50000`, berikan alamat fisiknya: `./gembok-cli --base 0xFF250000 info`.

`info` harus menampilkan ID `GEMB`, versi `1.0.0`, dan status `BOOTED`. Contoh bagian CAPS untuk parameter bawaan dengan `PUF_MODE = 2`:

```
CAPS        0x03000562: mode PUF 2 (kunci pengembangan tetap, tidak aman), bukan bangunan debug
            768 osilator, 5 suara, jendela 2^12 siklus (81,92 us), topeng data bantu 96 byte
            PUF_ENROLL dan PUF_RECON paling lama 15943152 siklus (318,863 ms)
```

### Perintah

| Perintah | Kegunaan |
|---|---|
| `info` | Register ID, VERSION, CAPS (diurai: mode PUF, osilator, suara, jendela ukur, panjang topeng, lama PUF paling lama), STATUS, PARAM, CTXLEN, CYCLES, COOLDOWN, PROOFS, PUF_THRESH |
| `selftest [-k K] [--thresh N]` | Uji mandiri tanpa data luar. Tanpa `-k`, ketiga tingkat diuji |
| `acvp BERKAS [--jumlah N]` | Jalankan vektor NIST ACVP. `--jumlah` membuat hasil gagal bila jumlah kasus berbeda |
| `puf-enroll [--thresh N] [--helper BERKAS]` | PUF_ENROLL, simpan data bantu. `--thresh` mengatur PUF_THRESH sebelum pendaftaran |
| `puf-recon --helper BERKAS` | PUF_RECON dari data bantu. Berkas ditolak bila jumlah osilatornya berbeda dari CAPS chip |
| `enroll [-k K] [--ek BERKAS]` | DAFTAR, tulis kunci enkapsulasi chip |
| `prove [-k K] --ct BERKAS [--ctx TEKS \| --ctx-hex HEX] [--tag BERKAS] [--kunci-bersama BERKAS]` | BUKTIKAN. Dengan `--kunci-bersama`, bukti dihitung ulang dengan SHA3-256 dan dicetak `ASLI` atau `PALSU` |
| `wipe` | Hapus semua memori dan lupakan kunci PUF |
| `keygen`, `encaps`, `decaps` | Mode terbuka. Benih yang tidak diberikan (`--d`, `--z`, `--m`) diambil acak |
| `puf-measure [--idx N] [--jumlah N]` | Hitungan mentah osilator c dan c+1 (20 bit, dari PUF_DBG dan PUF_DBG1), hanya pada bangunan debug. Nomor pasangan 0 sampai N_RO - 2 |

`-k` menerima 2, 3, 4 atau 512, 768, 1024. Bawaannya 3 (ML-KEM-768). Beberapa perintah bisa dirangkai dengan tanda `+` dan dijalankan berurutan pada satu sambungan, misalnya `puf-recon --helper bantu.txt + prove --ct c.hex`.

Kode keluar: 0 berhasil, 1 gagal (termasuk galat chip, berkas yang ditolak, dan hasil `PALSU`), 2 salah pemakaian. Pada perintah yang mengeluarkan data, pesan keadaan ditulis ke stderr dan data (heksadesimal, `ASLI`, `PALSU`) ke stdout.

### Alur mode brankas

Sekali saat pendaftaran:

```
./gembok-cli puf-enroll --thresh 64 --helper bantu.txt
./gembok-cli enroll -k 3 --ek chip.ek
```

Tiap chip menyala:

```
./gembok-cli puf-recon --helper bantu.txt
```

Tiap pemeriksaan keaslian. Di pemakaian sebenarnya pemeriksa menjalankan Encaps di perangkatnya sendiri. Untuk mencoba, Encaps milik chip bisa dipakai:

```
./gembok-cli encaps -k 3 --ek chip.ek --ct tantangan.ct --kunci-bersama rahasia.k
./gembok-cli prove -k 3 --ct tantangan.ct --ctx pintu-1 --tag bukti.tag --kunci-bersama rahasia.k
```

Bila pembatas laju masih aktif (galat RATE), `prove` dan `selftest` membaca COOLDOWN, menunggu sampai nol, lalu mengulang.

### Hal yang perlu diperhatikan

- `selftest` menjalankan PUF_ENROLL. Pada PUF sungguhan, kunci di brankas bisa berganti sampai `puf-recon` dijalankan lagi dengan data bantu yang tersimpan.
- `wipe` melupakan kunci PUF.
- Di mode terbuka, benih, `dk_pke`, dan kunci bersama tetap ada di memori byte chip sampai ditimpa atau `wipe`.
- Berkas `dk` dan kunci bersama dibuat dengan izin 0600.
- `--benih HEX` (64 digit) membuat semua bilangan acak berasal dari benih itu. `selftest` mencetak benih yang dipakai, sehingga jalannya bisa diulang persis.
- PUF_THRESH hanya dipakai PUF_ENROLL. `puf-recon` tidak menulisnya.
- Mode PUF 2 (kunci pengembangan tetap) tidak aman. Pada mode ini topeng data bantu tidak dibuat oleh chip: PUF_ENROLL menghitung nilai cek atas isi HELPER_MASK yang ada di memori (nol sesudah boot).

## Driver

### Fungsi

| Fungsi | Isi |
|---|---|
| `gembok_init`, `gembok_probe`, `gembok_wait_ready` | Siapkan driver, cek ID dan versi mayor 1 (lalu simpan CAPS), tunggu BOOTED = 1 dan BUSY = 0 |
| `gembok_status`, `gembok_get_caps`, `gembok_decode_caps`, `gembok_reg_read` | Baca STATUS, CAPS (sudah diurai, lihat di bawah), register apa pun |
| `gembok_helper_mask_len` | Panjang topeng data bantu chip ini: (N_RO + 6) / 8 byte, N_RO dari CAPS bit 27..16 |
| `gembok_keygen`, `gembok_encaps`, `gembok_decaps` | Mode terbuka. Driver menyusun `dk = DK \|\| EK \|\| H \|\| Z` dan memecahnya lagi untuk Decaps |
| `gembok_check_ek`, `gembok_check_dk` | Cek kunci. `check_dk` hanya menulis bagian EK dan H dari dk |
| `gembok_puf_enroll`, `gembok_puf_recon` | Perintah PUF. Panjang topeng harus sama dengan `gembok_helper_mask_len`. Ambang diatur dengan `gembok_set_thresh` |
| `gembok_puf_measure` | PUF_MEASURE: hitungan osilator c dari PUF_DBG dan c+1 dari PUF_DBG1, masing-masing 20 bit |
| `gembok_enroll`, `gembok_prove`, `gembok_wipe` | DAFTAR, BUKTIKAN, WIPE |
| `gembok_last_cycles`, `gembok_cycles` | CYCLES yang dibaca saat perintah terakhir selesai, atau dibaca langsung |
| `gembok_cooldown`, `gembok_cooldown_wait`, `gembok_proofs` | Pembatas laju |
| `gembok_set_poll_limit`, `gembok_command_poll_limit` | Batas tunggu biasa, dan batas yang dipakai untuk satu kode perintah |
| `gembok_command`, `gembok_mem_read`, `gembok_mem_write` | Akses tingkat rendah |

### CAPS

| Bit | Isi | Di `gembok_caps` |
|---|---|---|
| 1..0 | mode PUF: 0 osilator asli, 1 model simulasi, 2 kunci pengembangan | `puf_mode` |
| 2 | bangunan debug | `debug` |
| 7..3 | WIN_LOG2: satu pengukuran osilator memakai jendela 2^WIN_LOG2 siklus clock | `win_log2` |
| 15..8 | VOTES: jumlah pengukuran per pasangan | `votes` |
| 27..16 | N_RO: jumlah osilator | `oscillators` |
| 31..28 | nol | |

Dari CAPS, `gembok_decode_caps` juga menghitung `helper_mask_len` = (N_RO + 6) / 8 dan `puf_cycle_bound` = B, batas atas lama PUF_ENROLL dan PUF_RECON dalam siklus (lihat Batas waktu). Komponen Platform Designer hanya menerima N_RO 600 sampai 1025, VOTES ganjil 1 sampai 15, dan WIN_LOG2 4 sampai 16. Driver menerima N_RO 2 sampai 1025 supaya bangunan simulasi juga bisa dipakai.

Semua panjang, nilai k, dan nomor pasangan diperiksa sebelum perintah dikirim. Panjang masukan dan keluaran harus tepat sama dengan ukuran untuk k yang dipilih. Panjang topeng data bantu mengikuti CAPS, paling panjang 128 byte (N_RO sampai 1025).

### Kode galat

`gembok_err` memisahkan dua sumber galat:

- `GEMBOK_OK` = 0.
- `GEMBOK_CHIP_*` = 1 sampai 15, sama dengan STATUS.ERR dari chip (BAD_EK, BAD_DK, BAD_HELPER, BAD_PARAM, NO_KEY, RATE, BAD_CMD, PUF_FAIL). Kode chip yang belum dikenal (9 sampai 15) dikembalikan apa adanya.
- `GEMBOK_DRV_*` mulai 0x100: argumen, k, panjang, ID, versi, chip tetap sibuk, batas waktu, CAPS di luar jangkauan.

`gembok_err_is_chip()` membedakan keduanya dan `gembok_strerror()` memberi keterangan.

### Urutan satu perintah

1. Tunggu STATUS.BOOTED = 1 dan BUSY = 0.
2. Tulis PARAM, CTXLEN bila perlu, lalu masukan ke memori byte.
3. Tulis kode perintah ke CTRL.
4. Buang satu bacaan STATUS. Ini hanya pengaman. RTL sekarang sudah menunjukkan BUSY = 1 sejak siklus pertama sesudah CTRL ditulis dan mengabaikan tulisan memori pada siklus itu. Pada RTL sebelum perbaikan itu, STATUS yang dibaca tepat satu siklus sesudah CTRL masih berisi DONE dan ERR perintah sebelumnya.
5. Baca STATUS sampai DONE = 1 dan BUSY = 0, lalu baca CYCLES.
6. Bila ERR = 0, baca keluaran.

Driver tidak pernah menulis register atau memori saat BUSY = 1.

### Batas waktu

Setiap penantian dibatasi jumlah bacaan STATUS. Satu bacaan paling cepat satu siklus clock, jadi N bacaan selalu mencakup sedikitnya N siklus. Semua hitungan memakai bilangan 64 bit.

- Batas biasa: `poll_limit`, bawaan 50.000.000 bacaan (sedikitnya 1 detik pada 50 MHz), diubah dengan `gembok_set_poll_limit`. Dipakai untuk semua perintah selain perintah PUF, dan untuk menunggu chip siap sebelum perintah dikirim.
- PUF_ENROLL dan PUF_RECON: max(poll_limit, 2 x B) bacaan, dengan B = (N_RO - 1) x (VOTES x (2^WIN_LOG2 + 32) + 16) + 100.000 siklus dari CAPS. Pengekstrak mengukur tiap pasangan VOTES kali dan satu pengukuran memakan sekitar 2^WIN_LOG2 + 16 siklus, jadi B adalah batas atas lama perintah PUF. Dengan parameter bawaan (768 osilator, 5 suara, WIN_LOG2 = 12) B = 15.943.152 siklus (sekitar 0,32 detik), sehingga 2 x B = 31.886.304 masih di bawah batas biasa. Dengan parameter terbesar yang diizinkan (1025 osilator, 15 suara, WIN_LOG2 = 16) B = 1.007.240.864 siklus (sekitar 20 detik) dan batasnya 2.014.481.728 bacaan.
- PUF_MEASURE: max(poll_limit, 2 x (2^WIN_LOG2 + 2048)) bacaan.
- `gembok_cooldown_wait` membaca COOLDOWN lalu menunggu paling banyak COOLDOWN + 1024 bacaan.

`gembok_command_poll_limit(dev, kode)` memberi batas yang dipakai untuk satu kode perintah, termasuk lewat `gembok_command`. Bila perintah PUF yang panjang diputus di tengah (misalnya dengan Ctrl-C), perintah berikutnya bisa melapor chip tetap sibuk. Ulangi setelah perintah PUF itu selesai.

## Pengujian di simulasi

### Sambungan simulasi

`io_sim.cpp` memberi reset 5 siklus, lalu driver menunggu BOOTED. Transaksi dibuat sesuai aturan bus:

- Baca: alamat dan `avs_read` dipasang satu siklus, data diambil pada siklus berikutnya.
- Tulis: alamat, `avs_write`, dan `avs_writedata` dipasang satu siklus.
- Masukan diubah saat clock rendah, sebelum tepi naik.

Bawaannya tidak ada siklus kosong di antara transaksi, yaitu laju tercepat yang sah di Avalon-MM. Variabel lingkungan:

| Variabel | Arti |
|---|---|
| `GEMBOK_SIM_JEDA=n` | sisipkan n siklus kosong sesudah tiap transaksi, meniru jembatan HPS yang lebih lambat |
| `GEMBOK_SIM_STAT=1` | cetak jumlah siklus, baca, dan tulis saat selesai |

Parameter model diatur dengan `VPARAMS`. Bawaannya `PUF_MODE=2 COOLDOWN_CYCLES=20000` di `build/sim` (N_RO, VOTES, dan WIN_LOG2 bawaan RTL: 768, 5, 12). `make test-puf` memakai `PUF_MODE=1 CHIP_SEED=1 WIN_LOG2=3 PUF_DEBUG=1 COOLDOWN_CYCLES=20000` di `build/sim-puf`, lalu `PUF_MODE=1 CHIP_SEED=1 WIN_LOG2=4 VOTES=15 PUF_DEBUG=1 COOLDOWN_CYCLES=20000 N_RO=1025` di `build/sim-puf1025`. Model dibangun ulang otomatis bila `VPARAMS` atau berkas di `rtl/` berubah.

### Isi `make test`

1. `test-sha3`: SHA3-256 (pesan kosong, "abc", 200 byte 0xA3, sejuta "a", batas blok 135 sampai 273 byte, semua panjang 0 sampai 600 byte, potongan bertahap) dan SHAKE256, dibandingkan dengan nilai dari `hashlib`.
2. `gembok-sim acvp`: 240 kasus NIST ACVP lewat driver C. keyGen membandingkan ek dan dk lengkap, encapsulation membandingkan c dan K, decapsulation membandingkan K, kedua cek kunci membandingkan hasil lolos atau tidak.
3. `gembok-sim info` dan `gembok-sim selftest`, lalu `selftest` sekali lagi dengan `GEMBOK_SIM_JEDA=5`.
4. `test-driver`, dua kali (tanpa jeda dan dengan jeda 5). Isinya:
   - tiruan chip: ID salah, versi salah, chip selalu sibuk, DONE tidak pernah datang, COOLDOWN macet, CAPS dengan 2, 600, 768, 769, 770, 1025, dan 1026 osilator, WIN_LOG2 3, 4, 12, 16, dan 31, nilai B dan batas tunggu tiap kode perintah (termasuk 1025 osilator, 15 suara, WIN_LOG2 = 16), jumlah bacaan sampai batas waktu PUF habis, hitungan PUF_DBG dan PUF_DBG1 yang dipotong ke 20 bit;
   - semua penolakan argumen tanpa akses bus, termasuk panjang topeng yang tidak sesuai CAPS;
   - galat chip NO_KEY, BAD_CMD, BAD_PARAM;
   - CAPS chip cocok dengan rumus B, dan batas tunggu PUF sama dengan max(poll_limit, 2 x B);
   - kontrak bus: STATUS yang dibaca tepat sesudah CTRL menunjukkan BUSY, tulisan memori tepat sesudah CTRL diabaikan;
   - batas waktu lalu pulih;
   - mode terbuka termasuk BAD_EK dan BAD_DK;
   - mode brankas dengan kunci pengembangan: nilai cek data bantu, EK dari DAFTAR sama dengan KeyGen dari benih turunan dan sama dengan model acuan `model/gembok.py` (SHA3-256 dari EK model disimpan di `test_driver.c`), bukti untuk konteks 0, 1, 17, dan 255 byte, pembatas laju, slot rahasia nol sesudah DAFTAR dan BUKTIKAN, WIPE, PUF_RECON dengan data bantu benar dan salah (memori topeng diisi sampah dulu, supaya topeng yang tertulis kurang ikut ketahuan). PUF_ENROLL dan PUF_RECON dijalankan dengan batas biasa hanya 8 bacaan dan tetap berhasil karena memakai batas PUF, dan CYCLES-nya tidak melebihi B.
5. Alur lewat `gembok-sim` dengan berkas: keygen, encaps, decaps lalu kedua kunci bersama dibandingkan; puf-enroll, enroll, encaps, prove, wipe, puf-recon, enroll, prove lalu EK dan bukti dibandingkan; prove dengan kunci bersama yang salah harus keluar dengan kode 1 dan mencetak `PALSU`; data bantu dengan jumlah osilator lain dan data bantu dengan topeng terpotong harus ditolak dengan kode 1.

`make test-puf` menjalankan langkah 3 sampai 5 pada kedua bangunan model PUF. Di sana `test-driver` juga memeriksa topeng dan nilai cek terhadap model PUF yang sama dengan `tb/common/puf_model.py`, ambang 900 (PUF_FAIL, hampir semua pasangan diukur, CYCLES tetap di bawah B), ambang 200, dan `puf-measure` sampai pasangan terakhir.

## Format berkas

Berkas kunci, sandi, kunci bersama, dan bukti berisi heksadesimal. Spasi dan baris baru diabaikan saat dibaca.

Data bantu PUF (`puf-enroll --helper`), format 2:

```
gembok-helper 2
osilator 768
ambang 64
topeng <(osilator + 6) / 8 byte heksadesimal, 96 byte untuk 768 osilator>
cek <16 byte heksadesimal>
```

`puf-recon` menolak berkas bila panjang topeng tidak sama dengan (osilator + 6) / 8, atau bila jumlah osilator berbeda dari CAPS chip. `ambang` hanya catatan nilai PUF_THRESH saat pendaftaran.

Vektor ACVP (`tools/acvp_to_txt.py`): satu isian per baris, nama lalu nilai. Heksadesimal ditulis huruf kecil.

```
gembok-acvp 1
kasus 240
uji keyGen 2 1
d ...
z ...
ek ...
dk ...
selesai
uji encapsulationKeyCheck 2 116
ek ...
sah 1
selesai
```

Isian per fungsi: keyGen `d z ek dk`, encapsulation `ek m c k`, decapsulation `dk c k`, encapsulationKeyCheck `ek sah`, decapsulationKeyCheck `dk sah`. Baris terpanjang 6339 karakter.

## Hasil terukur (8 Oktober 2026)

Lingkungan: Verilator 5.020, gcc 13.3.0, Python 3.13. RTL dengan perbaikan `cmd_pend`, PUF_DBG1, pencacah osilator 20 bit, dan WIN_LOG2 di CAPS. Waktu pada 50 MHz (20 ns per siklus).

| Uji | Hasil |
|---|---|
| `test-sha3` | 38 dari 38 pemeriksaan lolos |
| ACVP lewat driver C | 240 dari 240 kasus lolos |
| `selftest` (jeda 0 dan jeda 5, ketiga bangunan) | LULUS, ketiga tingkat `ASLI`, sandi rusak `PALSU` |
| `test-driver`, PUF_MODE=2 | 470 dari 470 pemeriksaan lolos (jeda 0 dan jeda 5) |
| `test-driver`, model PUF, 768 osilator, 5 suara, WIN_LOG2 = 3 | 501 dari 501 pemeriksaan lolos (jeda 0 dan jeda 5) |
| `test-driver`, model PUF, 1025 osilator, 15 suara, WIN_LOG2 = 4 | 501 dari 501 pemeriksaan lolos (jeda 0 dan jeda 5) |
| Alur berkas `gembok-sim` | lolos pada ketiga bangunan |
| `make test` dari keadaan bersih | sekitar 15 detik, termasuk membangun model |
| `make test-puf` | sekitar 30 detik, termasuk membangun dua model |

Keluaran `acvp` (siklus dibaca driver dari register CYCLES):

```
parameter    fungsi                    lolos  total   siklus min  siklus maks
ML-KEM-512   keyGen                       25     25        10870        10923
ML-KEM-512   encapsulation                25     25        13467        13507
ML-KEM-512   decapsulation                10     10        19763        19795
ML-KEM-512   encapsulationKeyCheck        10     10          795          795
ML-KEM-512   decapsulationKeyCheck        10     10          995          995
ML-KEM-768   keyGen                       25     25        18069        18123
ML-KEM-768   encapsulation                25     25        21169        21249
ML-KEM-768   decapsulation                10     10        29730        29805
ML-KEM-768   encapsulationKeyCheck        10     10         1187         1187
ML-KEM-768   decapsulationKeyCheck        10     10         1451         1451
ML-KEM-1024  keyGen                       25     25        27228        27322
ML-KEM-1024  encapsulation                25     25        30803        30867
ML-KEM-1024  decapsulation                10     10        41742        41828
ML-KEM-1024  encapsulationKeyCheck        10     10         1579         1579
ML-KEM-1024  decapsulationKeyCheck        10     10         1907         1907

TOTAL 240 dari 240 kasus lolos
simulasi: 4586737 siklus clock, 4312008 baca, 274720 tulis, jeda 0 siklus
```

Mode brankas dengan kunci pengembangan (PUF_MODE=2):

| Perintah | ML-KEM-512 | ML-KEM-768 | ML-KEM-1024 |
|---|---|---|---|
| DAFTAR | 11294 siklus (0,226 ms) | 18110 siklus (0,362 ms) | 26884 siklus (0,538 ms) |
| BUKTIKAN, konteks 0 byte | 29417 siklus (0,588 ms) | 45357 siklus (0,907 ms) | 65299 siklus (1,306 ms) |
| BUKTIKAN, konteks 255 byte | 29721 siklus (0,594 ms) | 45661 siklus (0,913 ms) | 65603 siklus (1,312 ms) |

Catatan:

- Jumlah siklus KeyGen, Encaps, dan Decaps berbeda sedikit antar-kasus karena SampleNTT bergantung pada rho (data publik). Untuk satu kunci, Decaps dengan sandi sah dan sandi rusak selalu sama panjang.
- PUF_ENROLL dan PUF_RECON pada PUF_MODE=2 hanya 256 dan 259 siklus karena tidak ada pengukuran osilator. Pada model PUF simulasi hasilnya 38901 dan 34402 siklus (768 osilator, 5 suara, WIN_LOG2 = 3, B = 265672) serta 145801 dan 127654 siklus (1025 osilator, 15 suara, WIN_LOG2 = 4, B = 853664). PUF_ENROLL dengan ambang 900, yang mengukur hampir semua pasangan, memakan 95153 siklus (36 persen dari B) dan 491834 siklus (58 persen dari B). Angka ini tidak mewakili papan, karena jendela pengukuran di papan 2^12 siklus atau lebih.

### Pemeriksaan negatif yang sudah dijalankan

- Satu byte bukti harapan di `selftest` dibalik sementara: ketiga tingkat menjadi `PALSU`, `HASIL: GAGAL, 3 pemeriksaan tidak lolos`, kode keluar 1. Sesudah dikembalikan: `LULUS`.
- Satu digit sandi (encapsulation tcId 30) dan satu digit K (decapsulation tcId 90) diubah di salinan berkas vektor: `TOTAL 238 dari 240 kasus lolos`, kode keluar 1.
- Driver sengaja dirusak di salinan terpisah. Urutan H dan Z di dk ditukar: 165 dari 240 kasus ACVP lolos. H tidak ditulis untuk cek dk: 195 dari 240. Z diambil dari tempat yang salah: 225 dari 240. PARAM tidak mengikuti k: 105 dari 240. Topeng data bantu ditulis satu byte kurang: `test-driver` gagal pada ketiga bangunan. Label bukti, CTXLEN, dan padding SHA3 yang dirusak tertangkap oleh `selftest`.
- Tanpa pembuangan satu bacaan STATUS (langkah 4 di atas): pada RTL sebelum perbaikan `cmd_pend` hanya 28 dari 240 kasus ACVP lolos tanpa jeda bus. Pada RTL sekarang 240 dari 240 lolos, jadi bacaan itu tinggal pengaman.

### Bangunan untuk papan

`make` di mesin pengembangan memakai `cc` (gcc 13.3.0, x86-64) karena `arm-linux-gnueabihf-gcc` tidak terpasang. Sebagai pemeriksaan tambahan, sumber yang sama dikompilasi silang dengan clang 18 untuk armv7-a hard-float memakai header dan pustaka glibc 2.39 armhf dari paket Ubuntu, dengan bendera peringatan yang sama, tanpa peringatan, baik ditautkan dinamis maupun statis. `test-sha3` versi ARM lolos 38 dari 38 di bawah `qemu-arm` (cortex-a9). `gembok-cli` belum dijalankan di papan sungguhan.

## Yang belum diuji

- Papan DE10-Nano sungguhan: jembatan lightweight, alamat dasar IP, dan bitstream.
- PUF_MODE=0 (osilator asli) dan hitungan osilator di atas 16 bit, yang hanya muncul pada osilator sungguhan.
- Waktu nyata di papan. Angka di atas adalah siklus clock IP pada 50 MHz, belum termasuk waktu akses bus dari HPS. Pada ML-KEM-768, satu perintah memindahkan antara sekitar 1200 byte (DAFTAR) dan 3500 byte (Decaps) lewat bus, satu byte per akses.
