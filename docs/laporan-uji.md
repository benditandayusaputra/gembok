# Laporan uji

Semua uji di bawah berjalan di simulasi pada 8 Oktober 2026. Belum ada yang dijalankan di papan. Simulator utama Verilator 5.020 dengan cocotb 1.9.2. Simulator kedua Icarus Verilog 12.

Seluruh rangkaian dijalankan ulang dari keadaan bersih dengan `make semua` dan selesai dalam 18 menit 34 detik tanpa kegagalan, sekitar 6 menit di antaranya untuk konfigurasi `ro` yang mensimulasikan osilator perilaku. Uji mutasi (`make mutasi`) selesai dalam 9 menit 3 detik.

## Ringkasan

| Lapisan | Uji | Hasil |
|---|---|---|
| Model acuan | `tests/run_acvp.py`, `tests/test_model.py` | 240 dari 240 vektor NIST, 25 uji lolos |
| Tabel langkah di atas model | `tools/ucode_sim.py` | 240 dari 240 vektor NIST, 84 dari 84 uji protokol identitas |
| Lint | `verilator --lint-only -Wall` untuk PUF_MODE 0, 1, 2, dan untuk osilator perilaku | bersih, tanpa pengecualian lebar bit (pengecualian lain di `tools/verilator_lint.vlt`) |
| `modmul` | `tb/modmul`, menyeluruh | 3329 x 3329 pasangan, 0 salah |
| `keccak_core` | `tb/keccak`, 5 uji | lolos |
| `poly_arith` | `tb/poly_arith`, 5 uji | lolos |
| `sampler` | `tb/sampler`, 3 uji | lolos |
| `codec` | `tb/codec`, 4 uji | lolos |
| `mlkem_ctrl` | `tb/mlkem`, 3 uji | 240 dari 240 vektor NIST, operasi identitas sama dengan model |
| `gembok_top`, kunci pengembangan | `tb/top CONFIG=dev`, 13 uji | lolos, termasuk 240 dari 240 vektor NIST lewat bus |
| `gembok_top`, model PUF | `tb/top CONFIG=puf`, 4 uji | lolos |
| `gembok_top`, model PUF dengan VOTES = 15 | `tb/top CONFIG=votes15`, 2 uji | lolos |
| `gembok_top`, chip lain | `tb/top CONFIG=clone`, 1 uji | lolos |
| `gembok_top`, PUF berderau | `tb/top CONFIG=noise`, 2 uji | lolos |
| `gembok_top`, bangunan debug | `tb/top CONFIG=debug`, 1 uji | lolos |
| `gembok_top`, cabang osilator asli dengan osilator perilaku | `tb/top CONFIG=ro`, 1 uji | lolos |
| `gembok_top`, osilator perilaku, bangunan debug | `tb/top CONFIG=rodbg`, 1 uji | lolos |
| Simulator kedua | `tb/mlkem` di Icarus, uji `asap` dan `identitas` | lolos, jumlah siklus sama persis dengan Verilator |
| Uji mutasi | `tools/uji_mutasi.py` | 16 dari 16 mutasi tertangkap |
| Driver C | `sw/hps`, `make test` dan `make test-puf` | 240 dari 240 vektor NIST lewat driver, selftest LULUS, 470 dan 501 pemeriksaan driver |
| Layanan pemeriksa | `sw/verifier`, `go test ./...` | semua paket lolos, termasuk vektor RFC 8554 dan pengulangan PUF_RECON |

## Uji per modul

**`modmul`.** Testbench SystemVerilog murni yang dikompilasi Verilator, menyuapkan semua 11.082.241 pasangan (x, y) dan membandingkan keluaran kombinasional dan teregister dengan (x * y) mod 3329.

**`keccak_core`.** Dibandingkan dengan `hashlib` untuk keempat mode:
- panjang pesan di sekitar batas blok (0, 1, laju - 1, laju, laju + 1, dua dan tiga blok),
- 40 pesan acak dengan jeda acak di sisi serap dan sisi peras,
- keluaran per 3 byte melewati beberapa blok SHAKE128,
- `init` di tengah permutasi,
- permutasi tepat 24 siklus dan peras 1 byte per siklus.

**`poly_arith`.** Dibandingkan dengan `model/mlkem.py` pada polinomial acak dan polinomial sudut (nol, semua 3328, impuls, berselang-seling):
- NTT dan NTT invers (tanpa skala),
- PWM biasa dan akumulasi, termasuk a = b, dan masukan tidak berubah,
- LIN keempat varian, dengan slot tujuan terpisah, sama dengan x, dan sama dengan y,
- rangkaian seperti K-PKE.Encrypt,
- jumlah siklus tiap operasi tetap untuk data apa pun: NTT 940, NTT invers 940, PWM 520, LIN 264.

**`sampler`.** SampleNTT pada 24 rho acak, ditambah dua rho yang sengaja dicari karena butuh 4 blok SHAKE128. CBD eta 2 dan eta 3 pada benih nol, benih 0xFF, dan benih acak.

**`codec`.** Dekode dan enkode untuk d = 1, 4, 5, 10, 11, 12, dengan dan tanpa jeda. Dekode d = 12 dengan nilai 3329, 3330, dan 4095 di beberapa posisi: `range_err` harus naik dan nilai direduksi mod q. Kompresi untuk semua 3329 nilai koefisien pada tiap d. Dekode tidak boleh mengambil byte melebihi panjangnya.

## Uji inti dan tingkat atas

**`tb/mlkem`** (inti tanpa bus):
- `asap`: KeyGen, Encaps, Decaps, dan Decaps dengan ciphertext rusak untuk k = 2, 3, 4, dibanding model.
- `vektor_nist`: seluruh 240 kasus ACVP.
- `identitas`: DAFTAR dan BUKTIKAN dibanding `model/gembok.py` dengan konteks kosong, pendek, dan 255 byte; slot rahasia dan wilayah DK harus nol sesudahnya.

**`tb/top`** (lewat bus Avalon-MM, seperti driver di HPS). Konfigurasi `ro` dan `rodbg` memakai cabang osilator asli (`PUF_MODE = 0`) dengan osilator perilaku di balik makro `GEMBOK_SIM_RO`, jadi token, gerbang AND, pembagi dua, dan pencacah yang disintesis ikut diuji.

| Uji | Konfigurasi | Yang diperiksa |
|---|---|---|
| `register_dan_boot` | dev | ID, VERSION, CAPS, seluruh memori nol sesudah boot |
| `galat_perintah` | dev | galat BAD_PARAM, NO_KEY, BAD_CMD; perintah benar tetap jalan sesudahnya |
| `nist_lewat_bus` | dev | 240 dari 240 kasus ACVP |
| `brankas_terhadap_model` | dev | DAFTAR dan BUKTIKAN untuk k = 2, 3, 4 sama dengan model; nilai cek data bantu benar |
| `penjaga_akses` | dev | selama BUKTIKAN lebih dari 100 bacaan acak ke slot rahasia, DK, dan EK semuanya nol; tulisan ke CT, TAG, dan perintah WIPE saat sibuk tidak berpengaruh; sesudahnya seluruh 8 KB memori dan semua register tidak memuat potongan 8 byte dari kunci PUF, d, z, sigma, dk_pke, m', r', K, atau K_bar |
| `pembatas_laju` | dev | BUKTIKAN kedua langsung ditolak (galat 6), PROOFS tidak bertambah, DAFTAR tidak dibatasi, BUKTIKAN diterima lagi setelah hitung mundur |
| `pembatas_laju_tahan_reset` | dev | reset sesudah BUKTIKAN tidak mengosongkan COOLDOWN (2.965 sebelum reset, 2.960 sesudahnya) |
| `wipe_melupakan_kunci` | dev | WIPE menghapus memori dan kunci PUF |
| `reset_di_tengah_operasi` | dev | reset pada 4 titik berbeda di tengah BUKTIKAN: sesudah boot seluruh memori nol dan kunci hilang |
| `siklus_konstan` | dev | Decaps pada 10 masukan rahasia berbeda per tingkat (pesan lain, ciphertext rusak, s dan z lain dengan ek yang sama): jumlah siklus selalu sama (19.769, 29.755, 41.795) |
| `status_tidak_basi` | dev | STATUS yang dibaca tepat sesudah CTRL tidak menampilkan hasil perintah lama |
| `register_terkunci_saat_sibuk` | dev | PARAM, CTXLEN, PUF_THRESH yang ditulis saat sibuk diabaikan |
| `tulis_memori_sesudah_ctrl` | dev | tulisan ke slot D pada siklus tepat sesudah CTRL = KEYGEN diabaikan; EK sama dengan model untuk D yang asli |
| `pendaftaran_puf` | puf, votes15 | topeng dan kunci hasil RTL sama dengan model PUF Python; EK sama dengan model untuk k = 2, 3, 4 |
| `pemulihan_setelah_reset` | puf, votes15 | tiga kali reset lalu PUF_RECON: EK selalu sama |
| `data_bantu_diubah` | puf | pasangan dibuang, ditambah, ditukar, nilai cek diubah, topeng kosong, topeng penuh: semuanya ditolak dan kunci tidak bisa dipakai; data bantu benar kembali berhasil |
| `ambang_terlalu_tinggi` | puf | ambang 900 memberi PUF_FAIL; ambang 200 memberi kunci yang sesuai model |
| `chip_tiruan` | clone | data bantu chip asli ditolak chip lain; pendaftaran chip lain memberi EK lain dan buktinya ditolak pemeriksa |
| `kunci_stabil_dengan_derau` | noise | dengan derau, 8 kali reset dan pemulihan: EK selalu sama |
| `ambang_rendah_terdeteksi` | noise | ambang 1 dengan derau: pemulihan yang salah selalu terdeteksi (11 berhasil, 1 ditolak, 0 kunci salah) |
| `baca_hitungan_mentah` | debug | PUF_MEASURE di bangunan debug memberi hitungan yang sama dengan model, lewat PUF_DBG dan PUF_DBG1 |
| `ro_daftar_dan_pulih` | ro | PUF_ENROLL lewat osilator perilaku memilih tepat 256 pasangan (118.013 siklus dengan WIN_LOG2 = 6), dua kali reset lalu PUF_RECON memberi EK yang sama, topeng yang diubah ditolak |
| `ro_hitungan_20_bit` | rodbg | hitungan osilator sesuai periode osilator perilaku (selisih paling banyak 2) dan hitungan di atas 65.535 terbaca utuh (68.267 sampai 92.045) |

## Uji mutasi

`tools/uji_mutasi.py` menyalin RTL, menyisipkan satu kesalahan, lalu menjalankan uji yang relevan. Uji dianggap bermakna bila setiap mutasi membuatnya gagal.

| Mutasi | Ditangkap oleh |
|---|---|
| Konstanta Barrett 5039 menjadi 5038 | `asap` |
| Satu konstanta ronde Keccak salah | `asap` |
| Satu nilai zeta NTT salah | `asap` |
| Pembulatan kompresi d = 10 salah (berpengaruh pada satu dari 3329 nilai) | `kompresi_semua_nilai` |
| Batas penolakan SampleNTT `<` menjadi `<=` | `vektor_nist` |
| Penolakan implisit dimatikan (K selalu K') | `asap` |
| Penghapusan rahasia sesudah DAFTAR dihapus dari tabel langkah | `identitas` |
| Cek modulus kunci enkapsulasi dimatikan | `status_tidak_basi` |
| Penjaga akses memori dimatikan | `penjaga_akses` |
| Pembatas laju dimatikan | `pembatas_laju` |
| STATUS basi sesudah CTRL | `status_tidak_basi` |
| Register bisa diubah saat sibuk | `register_terkunci_saat_sibuk` |
| Cek data bantu PUF dilewati | `data_bantu_diubah` |
| Tulisan memori diterima pada siklus sesudah CTRL | `tulis_memori_sesudah_ctrl` |
| Reset mengosongkan pembatas laju | `pembatas_laju_tahan_reset` |
| Pencacah osilator dipersempit menjadi 16 bit | `ro_hitungan_20_bit` |

Hasil: 16 dari 16 tertangkap. Mutasi pembulatan kompresi sengaja diuji dengan uji unit kodek, karena kesalahan yang hanya mengenai satu dari 3329 nilai bisa lolos dari uji operasi lengkap yang memakai sedikit kasus. Uji unit kodek memeriksa semua nilai.

## Jumlah siklus

Lihat tabel di `docs/arsitektur.md` Bagian 12. Ringkasnya untuk ML-KEM-768: KeyGen sekitar 18.100, Encaps sekitar 21.200, Decaps sekitar 29.800, DAFTAR 18.110, BUKTIKAN 45.357 siklus (0,907 ms pada 50 MHz).

## Perangkat lunak

**Driver C** (`sw/hps`, rincian di `sw/hps/README.md`): driver dijalankan terhadap model Verilator dari `gembok_top`.
- 240 dari 240 vektor NIST lewat driver.
- selftest LULUS: ASLI untuk chip asli, PALSU untuk sandi rusak.
- 470 pemeriksaan driver dengan kunci pengembangan, 501 dengan model PUF 768 osilator, dan 501 dengan model PUF 1025 osilator, 15 suara, WIN_LOG2 = 4 (topeng 128 byte). Termasuk jalur galat, batas waktu perintah PUF yang diturunkan dari CAPS, CAPS di luar jangkauan, dan kontrak bus sesudah CTRL.

**Layanan pemeriksa** (`sw/verifier`, rincian di `sw/verifier/README.md`):
- tanda tangan LMS (RFC 8554, `LMS_SHA256_M32_H10`, `LMOTS_SHA256_N32_W8`) lolos vektor uji Lampiran F RFC 8554,
- chip simulasi di Go sama byte demi byte dengan model Python (benih, nilai cek, topeng, ek, H(ek), dan 32 tag) lewat berkas fixture dari `tools/gen_fixture_pemeriksa.py`, untuk 768, 801, dan 1025 osilator,
- skenario ASLI, PALSU (chip tiruan), bukti diputar ulang, sertifikat dirusak, pembatas laju, dan semua endpoint HTTP,
- PUF_RECON diulang: chip asli yang sekali gagal tetap ASLI, chip tiruan PALSU setelah tepat 5 percobaan tanpa BUKTIKAN,
- panjang topeng dan batas waktu perintah PUF dari CAPS (N_RO, VOTES, WIN_LOG2), termasuk perintah PUF 20 detik pada parameter terbesar dengan jam tiruan, peringatan mode PUF, dan ambang bawaan dari opsi `--ambang`.

## Yang belum diuji

- Kompilasi Quartus: sumber daya resmi, inferensi M10K, timing pada 50 MHz, dan batasan timing osilator di `quartus/gembok_puf.sdc`.
- Semua hal di papan: jembatan HPS, alamat dasar, driver `/dev/mem`, kecepatan nyata.
- PUF osilator asli di silikon: frekuensi, kestabilan, keunikan antarpapan dengan bitstream yang sama, bias, ambang yang tepat. Simulasi hanya membuktikan logika di sekitar osilator.
- Ketahanan saluran samping (daya, elektromagnetik) dan injeksi galat.

## Cara mengulang

| Perintah | Isi |
|---|---|
| `make semua` | semua uji di atas kecuali mutasi, Icarus, dan Yosys |
| `make mutasi` | uji mutasi |
| `make -C tb/mlkem SIM=icarus TESTCASE=asap,identitas` lalu `python3 tb/cek_hasil.py tb/mlkem/results.xml` | simulator kedua |
| `tools/sintesis_yosys.sh 2` | perkiraan sumber daya Yosys |

Satu uji tingkat atas tertentu: `make -C tb/top CONFIG=dev TESTCASE=penjaga_akses`.
