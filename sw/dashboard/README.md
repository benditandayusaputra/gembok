# Dasbor GEMBOK

Konsol demo satu halaman untuk layanan `gembok-verifier`. Dibangun dengan SvelteKit 2, Svelte 5 (runes), TypeScript, TailwindCSS 4, dan `@sveltejs/adapter-static`. Hasil bangunnya berkas statis yang disematkan ke biner Go, jadi dasbor ikut jalan di papan tanpa internet.

## Isi tampilan

- Gembok di bagian atas: tombol Periksa chip, lalu hasil ASLI atau PALSU dengan waktu chip (dari jumlah siklus pada 50 MHz), waktu total, jumlah siklus, dan alasan bila PALSU. Bila chip dinyalakan otomatis, catatan di bawahnya menyebut percobaan PUF_RECON: pada percobaan ke berapa kunci pulih, atau berapa percobaan yang semuanya ditolak (chip tiruan). Bila chip belum terdaftar, tombol utamanya menjadi Daftarkan chip.
- Chip di pembaca (hanya backend `sim`): pilih chip asli atau chip tiruan, dengan pilihan agar chip tiruan mendaftarkan PUF-nya sendiri.
- Coba baca kunci: menjalankan satu bukti lalu menampilkan isi memori yang dilihat host di wilayah rahasia (semuanya nol) dan wilayah publik H serta TAG sebagai pembanding.
- Status chip: backend, parameter, kunci PUF, sumber PUF, jumlah osilator, panjang topeng data bantu, dan jendela ukur 2^WIN_LOG2 siklus (semuanya dari CAPS), pendaftaran, jumlah bukti, pemakaian tanda tangan penerbit, pembatas laju.
- Peringatan keamanan PUF: bila `GET /api/status` mengisi `peringatan` (backend `mmio` dengan PUF_MODE 1, 2, atau 3, atau bangunan debug), kotak merah muncul di atas halaman dan sumber PUF di Status chip ditandai merah. Pada mode itu kunci chip bukan rahasia, jadi hasil ASLI tidak berarti apa-apa. Backend `sim` tidak memunculkan peringatan ini; label "Model simulasi" di Status chip sudah cukup.
- Pendaftaran dan penyalaan: daftar atau daftar ulang (dengan konfirmasi karena memakai tanda tangan penerbit) dan nyalakan chip. Pesan hasil penyalaan menyebut percobaan, misalnya "percobaan 1 dari 5".
- Ambang PUF ada di bagian "Pengaturan lanjutan" yang tertutup secara bawaan, karena biasanya tidak perlu diubah. Bila dikosongkan, permintaan tidak membawa `ambang` dan layanan memakai nilai `--ambang` (bawaan 64), yang juga ditampilkan di petunjuk. Isi bila karakterisasi papan menghasilkan ambang lain; nilainya bilangan bulat 1 sampai 65535.
- Riwayat pemeriksaan terbaru.
- Bahasa Indonesia (bawaan) dan Inggris, tema gelap (bawaan) dan terang. Pilihan bahasa dan tema disimpan di `localStorage` peramban.
- Bila layanan tidak terjangkau, muncul pemberitahuan dan semua tombol aksi dinonaktifkan; dasbor mencoba lagi setiap 3 detik.

## Membangun

Butuh Node 20.19 atau lebih baru.

```
cd sw/dashboard
npm ci
npm run check
npm run build
```

`npm run check` menjalankan svelte-check dan gagal bila ada galat atau peringatan. `npm run build` menulis berkas statis ke `build/`.

Untuk menyematkan hasilnya ke layanan, jalankan dari `sw/verifier`:

```
make dashboard
```

Target itu menjalankan ketiga perintah di atas lalu menyalin `build/` ke `sw/verifier/internal/httpapi/web/`. Isi folder itu disimpan di repositori supaya `go build` tidak butuh Node.

## Mengembangkan

Jalankan layanan di satu terminal, lalu server pengembangan Vite di terminal lain:

```
cd sw/verifier && go run ./cmd/gembok-verifier --backend sim
cd sw/dashboard && npm run dev
```

Buka alamat yang ditampilkan Vite (biasanya `http://localhost:5173`). Permintaan `/api` diteruskan ke `http://127.0.0.1:8080`.

## Catatan

- Tidak ada font luar atau CDN. Huruf memakai font sistem, ikon dan pola guilloche digambar sebagai SVG di dalam halaman.
- Halaman memakai Content Security Policy dengan hash untuk skrip pembuka SvelteKit (`kit.csp` di `svelte.config.js`). Layanan Go menambah `frame-ancestors 'none'` dan header keamanan lain.
- `kit.version.name` dibuat tetap supaya hasil bangun yang sama menghasilkan berkas yang sama.
- Halaman dirender di peramban (`ssr = false`), jadi berkas `index.html` hanya berisi kerangka.
- `npm audit` melaporkan paket `cookie` versi lama di dalam `@sveltejs/kit` 2. Paket itu hanya dipakai server SvelteKit; dasbor ini tidak menjalankan server SvelteKit karena disajikan sebagai berkas statis oleh layanan Go.
