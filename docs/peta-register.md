# Peta register dan cara pakai dari host

Dokumen ini adalah kontrak antara IP GEMBOK (`rtl/gembok_top.sv`) dan perangkat lunak di HPS.

## Bus

- Avalon-MM slave, data 32 bit, alamat dalam kata. Offset byte = alamat kata x 4.
- Latensi baca tetap 1 siklus, tanpa `waitrequest`.
- Rentang 64 KB (0x0000 sampai 0xFFFF).
- Pada DE10-Nano IP ini dipasang di jembatan lightweight HPS-to-FPGA, yang di Linux muncul di alamat fisik `0xFF200000` ditambah alamat dasar IP di Platform Designer.

## Register

| Offset | Nama | Akses | Isi |
|---|---|---|---|
| 0x0000 | ID | R | `0x47454D42` (ASCII "GEMB") |
| 0x0004 | VERSION | R | `0x00010000`: bit 31..16 versi mayor, 15..8 minor, 7..0 tambalan (1.0.0) |
| 0x0008 | CTRL | W | bit 3..0: kode perintah |
| 0x000C | STATUS | R | lihat tabel STATUS |
| 0x0010 | PARAM | R/W | bit 2..0: k. 2 = ML-KEM-512, 3 = ML-KEM-768, 4 = ML-KEM-1024 |
| 0x0014 | CTXLEN | R/W | bit 7..0: panjang konteks bukti (0 sampai 255) |
| 0x0018 | CYCLES | R | lama perintah terakhir dalam siklus clock (50 MHz, 20 ns per siklus). Bernilai 1 untuk perintah yang langsung ditolak. Sesudah reset berisi lama penghapusan awal |
| 0x001C | CAPS | R | bit 1..0 mode PUF (0 osilator asli, 1 model simulasi, 2 kunci pengembangan), bit 2 bangunan debug, bit 7..3 WIN_LOG2 (jendela ukur 2^WIN_LOG2 siklus), bit 15..8 jumlah pengukuran per pasangan (VOTES), bit 27..16 jumlah osilator (N_RO), bit 31..28 nol. Contoh: `0x03000560` untuk PUF osilator dengan parameter bawaan |
| 0x0020 | COOLDOWN | R | sisa hitung mundur pembatas laju, dalam siklus. Bawaan: 1.048.576 siklus (sekitar 21 ms) setelah tiap BUKTIKAN. Reset tidak mengosongkannya: hitung mundur berhenti selama reset lalu berlanjut. Hanya konfigurasi ulang FPGA yang mengembalikannya ke nol sebelum habis |
| 0x0024 | PROOFS | R | jumlah BUKTIKAN yang diterima sejak reset |
| 0x0028 | PUF_THRESH | R/W | bit 15..0: ambang selisih hitungan untuk memilih pasangan andal saat PUF_ENROLL. Nilai awal 64. Tidak dipakai saat PUF_RECON |
| 0x002C | PUF_IDX | R/W | bit 9..0: nomor pasangan c (0 sampai N_RO - 2) untuk PUF_MEASURE (hanya bangunan debug) |
| 0x0030 | PUF_DBG | R | bit 19..0: hitungan osilator c dalam satu jendela ukur (hanya bangunan debug, selain itu 0) |
| 0x0034 | PUF_DBG1 | R | bit 19..0: hitungan osilator c+1 dalam jendela yang sama (hanya bangunan debug, selain itu 0) |

Selama BUSY = 1, tulisan ke semua register (termasuk CTRL, PARAM, CTXLEN, PUF_THRESH, PUF_IDX) diabaikan. BUSY sudah 1 sejak siklus tepat sesudah CTRL ditulis. Nilai PARAM dan CTXLEN dikunci saat perintah diterima.

### STATUS

STATUS yang dibaca sesudah CTRL ditulis selalu menunjukkan perintah baru: BUSY = 1 sampai perintah itu selesai, lalu DONE = 1 beserta kode galatnya. Perintah yang langsung ditolak (galat 4 sampai 7) selesai dalam beberapa siklus.

| Bit | Nama | Arti |
|---|---|---|
| 0 | BUSY | perintah sedang berjalan |
| 1 | DONE | perintah terakhir selesai. Turun saat perintah baru diterima |
| 2 | ERROR | kode galat bukan nol |
| 7..4 | ERR | kode galat perintah terakhir |
| 8 | PUF_READY | kunci PUF sudah ada di brankas |
| 9 | COOLDOWN | pembatas laju masih menghitung mundur |
| 10 | BOOTED | penghapusan memori sesudah reset sudah selesai |

### Kode perintah (CTRL)

| Kode | Nama | Mode | Masukan | Keluaran |
|---|---|---|---|---|
| 1 | KEYGEN | terbuka | D, Z | EK, DK, H |
| 2 | ENCAPS | terbuka | EK, M | CT, K |
| 3 | DECAPS | terbuka | DK, EK, H, Z, CT | K |
| 4 | CHECK_EK | terbuka | EK | hanya kode galat |
| 5 | CHECK_DK | terbuka | EK, H | hanya kode galat |
| 6 | ENROLL (DAFTAR) | brankas | tidak ada | EK, H |
| 7 | PROVE (BUKTIKAN) | brankas | CT, CTX, CTXLEN | TAG |
| 8 | PUF_ENROLL | brankas | PUF_THRESH | HELPER_MASK, HELPER_CHK |
| 9 | PUF_RECON | brankas | HELPER_MASK, HELPER_CHK | hanya kode galat |
| 10 | PUF_MEASURE | debug | PUF_IDX | PUF_DBG, PUF_DBG1 |
| 15 | WIPE | semua | tidak ada | semua memori nol, kunci PUF dilupakan |

Semua perintah ML-KEM memakai k dari PARAM.

### Kode galat (STATUS bit 7..4)

| Kode | Nama | Arti |
|---|---|---|
| 0 | OK | berhasil |
| 1 | BAD_EK | kunci enkapsulasi gagal cek modulus (FIPS 203 Bagian 7.2) |
| 2 | BAD_DK | kunci dekapsulasi gagal cek hash (FIPS 203 Bagian 7.3) |
| 3 | BAD_HELPER | data bantu PUF tidak cocok dengan kunci yang dipulihkan |
| 4 | BAD_PARAM | k bukan 2, 3, atau 4 |
| 5 | NO_KEY | kunci PUF belum siap (jalankan PUF_ENROLL atau PUF_RECON dulu) |
| 6 | RATE | pembatas laju masih aktif, ulangi setelah COOLDOWN nol |
| 7 | BAD_CMD | kode perintah tidak dikenal atau tidak tersedia di bangunan ini |
| 8 | PUF_FAIL | PUF tidak memberi tepat 256 bit: saat PUF_ENROLL pasangan andal kurang dari 256 (ambang terlalu tinggi), saat PUF_RECON topeng tidak berisi tepat 256 pasangan |

## Memori byte

Byte ke-i memori byte ada di offset `0x8000 + 4*i`, i = 0 sampai 8191. Tiap akses bus membawa satu byte di bit 7..0.

Memori byte hanya bisa diakses saat BUSY = 0. Saat sibuk, baca menghasilkan 0 dan tulis diabaikan. Tulisan pada siklus tepat sesudah CTRL ditulis juga diabaikan.

| Alamat byte | Nama | Panjang | Isi |
|---|---|---|---|
| 0x0000 | EK | 384k + 32 | kunci enkapsulasi: ByteEncode12(t) lalu rho |
| 0x0800 | CT | 32(du k + dv) | ciphertext: c1 lalu c2 |
| 0x1000 | DK | 384k | dk_pke = ByteEncode12(s). Hanya mode terbuka |
| 0x1800 | D | 32 | benih d |
| 0x1820 | Z | 32 | benih z |
| 0x1840 | M | 32 | pesan m |
| 0x1860 | K | 32 | kunci bersama |
| 0x1880 | SIGMA | 32 | benih derau (internal) |
| 0x18A0 | KBAR | 32 | kunci penolakan implisit (internal) |
| 0x18C0 | H | 32 | H(ek) |
| 0x18E0 | TAG | 32 | bukti keaslian |
| 0x1900 | CTX | 255 | konteks bukti |
| 0x1A00 | HELPER_MASK | (N_RO + 6) / 8 | topeng pasangan PUF (data bantu, publik). Bit c = 1 berarti pasangan c dipakai. Panjangnya (N_RO + 6) / 8 byte dengan pembagian bulat: 96 untuk N_RO = 768 bawaan, paling banyak 128 untuk N_RO = 1025. N_RO dibaca dari CAPS |
| 0x1A80 | HELPER_CHK | 16 | nilai cek data bantu (publik) |

Ukuran per tingkat:

| k | Tingkat | EK | CT | DK (dk_pke) | dk lengkap FIPS 203 |
|---|---|---|---|---|---|
| 2 | ML-KEM-512 | 800 | 768 | 768 | 1632 |
| 3 | ML-KEM-768 | 1184 | 1088 | 1152 | 2400 |
| 4 | ML-KEM-1024 | 1568 | 1568 | 1536 | 3168 |

Kunci dekapsulasi lengkap menurut FIPS 203 adalah `dk = dk_pke || ek || h || z`. Chip menyimpan keempat bagian di tempat terpisah (DK, EK, H, Z), dan driver yang menyambung atau memecahnya.

## Urutan pemakaian

Pola umum satu perintah:

1. Pastikan STATUS.BUSY = 0.
2. Tulis PARAM (dan CTXLEN bila perlu), lalu tulis masukan ke memori byte.
3. Tulis kode perintah ke CTRL.
4. Tunggu STATUS.DONE = 1.
5. Baca STATUS.ERR. Bila 0, baca keluaran dari memori byte.

### Mode terbuka (akselerator ML-KEM)

- **KeyGen** (FIPS 203 Algoritma 16): tulis D dan Z, perintah 1. Baca EK (384k + 32 byte), DK (384k byte), H. Susun `dk = DK || EK || H || Z`.
- **Encaps** (Algoritma 17): tulis EK dan M, perintah 2. Baca K dan CT. Galat 1 bila EK gagal cek modulus.
- **Decaps** (Algoritma 18): pecah dk menjadi DK, EK, H, Z dan tulis masing-masing, tulis CT, perintah 3. Baca K. Galat 2 bila H tidak sama dengan H(EK).
- **Cek kunci**: perintah 4 untuk EK, perintah 5 untuk bagian EK dan H dari dk. Panjang kunci diperiksa oleh driver.

Benih D, Z, dan M harus acak. Di mode terbuka host yang menyediakannya.

### Mode brankas (chip identitas)

Sekali saat chip dibuat (pendaftaran):

1. Tulis PUF_THRESH bila ingin nilai selain 64.
2. Perintah 8 (PUF_ENROLL). Galat 8 bila pasangan andal kurang dari 256.
3. Baca HELPER_MASK ((N_RO + 6) / 8 byte, 96 untuk bawaan) dan HELPER_CHK (16 byte), simpan sebagai data bantu chip bersama N_RO. Data ini publik.
4. Perintah 6 (ENROLL) dengan k yang diinginkan. Baca EK. Kirim EK ke penerbit untuk disertifikasi.

Tiap chip menyala:

1. Tulis HELPER_MASK dan HELPER_CHK dari data bantu yang disimpan.
2. Perintah 9 (PUF_RECON). Galat 3 bila kunci yang dipulihkan tidak cocok, galat 8 bila topeng tidak berisi tepat 256 pasangan.
3. Pada PUF osilator asli, pemulihan sesekali bisa gagal karena derau. Ulangi PUF_RECON beberapa kali sebelum menyimpulkan chip palsu. `gembok-verifier` mencoba sampai 5 kali (opsi `--coba-nyala`). Chip tiruan gagal di setiap percobaan.

Tiap pemeriksaan keaslian:

1. Pemeriksa membuat (K, c) dengan Encaps terhadap EK chip yang sudah disertifikasi.
2. Tulis c ke CT, konteks ke CTX, panjangnya ke CTXLEN. Perintah 7 (PROVE).
3. Baca TAG (32 byte).
4. Pemeriksa menghitung `SHA3-256("GEMBOK-v1/bukti" || K || byte(len(ctx)) || ctx)` dan membandingkannya dengan TAG.

Galat 6 berarti pembatas laju masih aktif. Baca COOLDOWN, tunggu, lalu ulangi. Hanya BUKTIKAN yang dibatasi. DAFTAR tidak dibatasi, alasannya ada di `docs/arsitektur.md` Bagian 9.

### Turunan kunci di dalam chip

- `(d, z) = SHAKE256("GEMBOK-v1/benih" || kunci PUF)`, 64 byte, d lebih dulu.
- `HELPER_CHK = SHAKE256("GEMBOK-v1/cek" || kunci PUF || HELPER_MASK)`, 16 byte.
- Kunci ML-KEM dibangkitkan ulang dari (d, z) pada tiap DAFTAR dan BUKTIKAN, lalu dihapus.

## Yang dijamin perangkat keras

- Kunci PUF hanya ada di register brankas. Tidak ada register atau alamat memori yang menampilkannya.
- Sesudah DAFTAR dan BUKTIKAN, slot D, Z, M, K, SIGMA, KBAR dan seluruh memori polinomial bernilai nol sebelum BUSY turun.
- Sesudah reset, seluruh memori dihapus sebelum perintah pertama diterima.
- Jumlah siklus Decaps tidak bergantung pada kunci rahasia, pesan, atau sah tidaknya ciphertext.

## Yang tidak dijamin

- Ketahanan terhadap serangan daya, elektromagnetik, atau injeksi galat.
- Ketahanan terhadap penyerang yang bisa memuat bitstream lain atau memakai JTAG.
- Mode PUF 2 (kunci pengembangan tetap) tidak aman dan hanya untuk menyalakan papan pertama kali.
- Mode PUF 1 (model simulasi) hanya untuk simulasi. Kuncinya bisa dihitung dari parameter publik, jadi komponen Platform Designer hanya menerima mode 0 dan 2.

## Catatan mode PUF 2 (kunci pengembangan)

- PUF_ENROLL dan PUF_RECON memuat kunci tetap tanpa mengukur osilator.
- PUF_ENROLL tidak menulis HELPER_MASK. HELPER_CHK dihitung atas isi HELPER_MASK yang ada (nol sesudah boot).
- PUF_RECON hanya bisa gagal dengan galat 3, tidak pernah galat 8.
