import { buatDunia } from "./dunia.js";
import { animasi, tunggu, mudah, modeRekam } from "./jam.js";

const T = {
  id: {
    sub: "Simulasi 3D kasus nyata",
    memuat: "Menyiapkan adegan 3D...",
    tentang: "Tentang",
    model3d: "Model chip",
    dokumen: "Dokumen ber-chip",
    penerbit: "Penerbit LMS",
    menyiapkan: "menyiapkan 1.024 kunci sekali pakai",
    siap: "siap",
    galatMuat: "gagal memuat modul, muat ulang halaman",
    terbitkan: "Terbitkan di pabrik",
    menerbitkan: "Menerbitkan...",
    sudahTerbit: "Sudah diterbitkan",
    menungguPenerbit: "Menunggu penerbit siap...",
    pilihSkenario: "Siapa yang datang ke pemeriksa?",
    jejak: "Jejak pemeriksaan",
    konteksLbl: "Konteks bukti",
    kosongBelum: "Terbitkan dokumen di pabrik dulu, lalu pilih siapa yang datang ke pemeriksa.",
    kosongSiap: "Pilih skenario untuk memulai pemeriksaan.",
    buktiChip: "Bukti yang diterima",
    buktiHarapan: "Bukti yang diharapkan",
    petunjuk: "Seret untuk memutar · gulir untuk memperbesar",
    idChip: "ID chip",
    asli: "ASLI",
    palsu: "PALSU",
    tutup: "Tutup",
    tentangJ: "Apa yang terjadi di halaman ini",
    nyataJ: "Yang sungguhan",
    nyataP: "ML-KEM-768 sesuai FIPS 203 (crypto/mlkem Go), tanda tangan hash LMS penerbit, dan SHA3-256. Kodenya sama dengan pemeriksa di repositori, dikompilasi ke WebAssembly dan dijalankan di peramban Anda.",
    modelJ: "Yang disimulasikan",
    modelP: "Chip dijalankan sebagai model perangkat lunak dengan peta register, kode galat, dan driver yang sama dengan perangkat keras. Jumlah siklus diambil dari simulasi RTL. Adegan 3D adalah ilustrasi.",
    klonJ: "Kenapa salinan gagal",
    klonP: "Kunci rahasia tidak disimpan di mana pun. Kunci dibentuk ulang dari selisih kecepatan 768 osilator setiap kali chip menyala, dan selisih itu berbeda pada setiap keping silikon.",
    pab1: "Kunci PUF didaftarkan", pab1d: (r) => `${fmt(r.siklus_puf)} siklus osilator, data bantu disimpan di dokumen`,
    pab2: "Kunci ML-KEM-768 dibuat di chip", pab2d: (r) => `kunci publik ${r.panjang_ek} byte · ${r.ek_awal.slice(0, 16)}...`,
    pab3: "Sertifikat ditandatangani", pab3d: (r) => `tanda tangan LMS ${r.panjang_ttd} byte`,
    sk: {
      asli: ["Dokumen asli", "Pemegang sah menempelkan dokumen yang diterbitkan pabrik."],
      klon: ["Chip dikloning", "Sertifikat dan data bantu disalin ke chip lain."],
      klon_daftar: ["Tiruan daftar sendiri", "Chip tiruan membuat kunci PUF sendiri dan memakai sertifikat curian."],
      ulang: ["Putar ulang bukti", "Perekam di antara chip dan pemeriksa mengganti jawaban dengan bukti lama."]
    },
    l: { sertifikat: "Periksa sertifikat", nyala: "Nyalakan chip, pulihkan kunci PUF", tantangan: "Kirim tantangan acak", bukti: "Chip menjawab", cocokkan: "Cocokkan bukti" },
    d: {
      sertifikat: (r) => `Tanda tangan LMS penerbit sah untuk ${r.id}.`,
      nyalaOk: (r) => `Kunci pulih pada percobaan ke-${r.percobaan}. Kunci rahasia tidak pernah keluar dari chip.`,
      nyalaGagal: () => `Lima kali dicoba, kunci tidak pulih. Osilator chip ini berbeda, data bantu salinan ditolak.`,
      nyalaSendiri: () => `Chip tiruan membentuk kunci dari PUF-nya sendiri, bukan pasangan sertifikat.`,
      tantangan: (r) => `Encaps ML-KEM-768 ke kunci di sertifikat: sandi ${fmt(r.panjang_sandi)} byte, rahasia 32 byte disimpan pemeriksa.`,
      bukti: (r) => `Chip membuka sandi dan menghitung bukti 32 byte dalam ${fmt(r.siklus)} siklus (${fmt(r.us_chip)} µs pada 50 MHz).`,
      buktiUlang: () => `Jawaban chip dicegat. Yang sampai ke pemeriksa adalah bukti rekaman dari pemeriksaan sebelumnya.`,
      cocokOk: () => `SHA3-256 atas rahasia dan konteks sama persis dengan bukti.`,
      cocokGagal: () => `Bukti berbeda dari yang diharapkan pemeriksa.`,
      lewat: () => `Tidak dijalankan.`
    },
    alasan: {
      asli: "Bukti cocok dengan kunci publik di sertifikat.",
      klon: "Chip ini tidak bisa memulihkan kunci dari PUF yang didaftarkan.",
      klon_daftar: "Chip tiruan tidak memegang kunci pasangan sertifikat.",
      ulang: "Bukti lama tidak berlaku untuk tantangan yang baru."
    },
    ringkas: (r) => `${fmt(r.ms_total, 1)} ms di peramban ini`,
    perangkat: { paspor: "Autogate · Imigrasi", ijazah: "Verifikasi Ijazah", cukai: "Pemindai Cukai", akses: "Pintu B2" },
    layar: {
      siaga: "Siap memeriksa", siagaRinci: "Tempelkan chip ke pembaca",
      pabrik: "Personalisasi", pabrikRinci: "PUF, kunci, dan sertifikat",
      dekatkan: "Membaca chip", membaca: "Cek sertifikat", nyala: "Menyalakan chip",
      pufGagal: "Kunci tidak pulih", pufGagalRinci: "Data bantu ditolak",
      tantangan: "Kirim tantangan", cocok: "Cocokkan bukti",
      asli: { paspor: "SILAKAN MASUK", ijazah: "IJAZAH ASLI", cukai: "PITA ASLI", akses: "AKSES DITERIMA" },
      palsu: { paspor: "DITOLAK", ijazah: "IJAZAH PALSU", cukai: "PITA PALSU", akses: "DITOLAK" },
      asliRinci: "Bukti cocok dengan sertifikat", palsuRinci: "Hubungi petugas"
    },
    holo: {
      sertifikat: "SERTIFIKAT · LMS", ttd: "tanda tangan penerbit", sah: "SAH", tidakSah: "TIDAK SAH",
      pufDaftar: "PUF · mendaftarkan 768 osilator", kunciDibuat: "Kunci ML-KEM-768 terbentuk", pufBaca: "PUF · mengukur 768 osilator",
      kunciPulih: "Kunci PUF pulih", kunciGagal: "Kunci tidak pulih", percobaan: "PUF · percobaan", pufSendiri: "PUF chip tiruan", kunciLain: "Kunci lain, bukan pasangan sertifikat"
    },
    tag: { klon: "Chip salinan", tiruan: "Chip tiruan", perekam: "Perekam", buktiLama: "bukti lama" },
    kasus: {
      paspor: ["Paspor elektronik", "Autogate imigrasi Terminal 3"],
      ijazah: ["Ijazah digital", "Aplikasi HRD saat rekrutmen"],
      cukai: ["Pita cukai", "Petugas di gudang distributor"],
      akses: ["Kartu akses", "Pintu ruang server B2"]
    }
  },
  en: {
    sub: "3D real-case simulation",
    memuat: "Preparing the 3D scene...",
    tentang: "About",
    model3d: "Chip model",
    dokumen: "Chip-bearing document",
    penerbit: "LMS issuer",
    menyiapkan: "preparing 1,024 one-time keys",
    siap: "ready",
    galatMuat: "failed to load the module, reload the page",
    terbitkan: "Issue at the factory",
    menerbitkan: "Issuing...",
    sudahTerbit: "Issued",
    menungguPenerbit: "Waiting for the issuer...",
    pilihSkenario: "Who walks up to the verifier?",
    jejak: "Verification trace",
    konteksLbl: "Proof context",
    kosongBelum: "Issue the document at the factory first, then pick who walks up to the verifier.",
    kosongSiap: "Pick a scenario to start the check.",
    buktiChip: "Proof received",
    buktiHarapan: "Expected proof",
    petunjuk: "Drag to orbit · scroll to zoom",
    idChip: "Chip ID",
    asli: "GENUINE",
    palsu: "FORGED",
    tutup: "Close",
    tentangJ: "What happens on this page",
    nyataJ: "What is real",
    nyataP: "ML-KEM-768 per FIPS 203 (Go crypto/mlkem), the issuer's LMS hash-based signature, and SHA3-256. Same code as the verifier in the repository, compiled to WebAssembly and running in your browser.",
    modelJ: "What is simulated",
    modelP: "The chip runs as a software model with the same register map, error codes, and driver as the hardware. Cycle counts come from RTL simulation. The 3D scene is an illustration.",
    klonJ: "Why copies fail",
    klonP: "The secret key is stored nowhere. It is rebuilt from the speed differences of 768 oscillators every time the chip powers up, and those differences are unique to each piece of silicon.",
    pab1: "PUF key enrolled", pab1d: (r) => `${fmt(r.siklus_puf)} oscillator cycles, helper data stored on the document`,
    pab2: "ML-KEM-768 key generated in the chip", pab2d: (r) => `public key ${r.panjang_ek} bytes · ${r.ek_awal.slice(0, 16)}...`,
    pab3: "Certificate signed", pab3d: (r) => `LMS signature ${r.panjang_ttd} bytes`,
    sk: {
      asli: ["Genuine document", "The rightful holder taps the document issued at the factory."],
      klon: ["Cloned chip", "Certificate and helper data copied onto another chip."],
      klon_daftar: ["Self-enrolled fake", "A fake chip enrolls its own PUF key and reuses a stolen certificate."],
      ulang: ["Replayed proof", "A recorder between chip and verifier swaps the answer for an old proof."]
    },
    l: { sertifikat: "Check the certificate", nyala: "Power up, recover the PUF key", tantangan: "Send a random challenge", bukti: "Chip answers", cocokkan: "Compare the proof" },
    d: {
      sertifikat: (r) => `Issuer LMS signature is valid for ${r.id}.`,
      nyalaOk: (r) => `Key recovered on attempt ${r.percobaan}. The secret key never leaves the chip.`,
      nyalaGagal: () => `Five attempts, no key. This chip's oscillators differ, so the copied helper data is rejected.`,
      nyalaSendiri: () => `The fake chip derives a key from its own PUF, not the one paired with the certificate.`,
      tantangan: (r) => `ML-KEM-768 Encaps to the certificate's key: ${fmt(r.panjang_sandi)}-byte ciphertext, 32-byte secret kept by the verifier.`,
      bukti: (r) => `The chip decapsulates and computes a 32-byte proof in ${fmt(r.siklus)} cycles (${fmt(r.us_chip)} µs at 50 MHz).`,
      buktiUlang: () => `The chip's answer is intercepted. What reaches the verifier is a proof recorded from an earlier check.`,
      cocokOk: () => `SHA3-256 over the secret and context matches the proof exactly.`,
      cocokGagal: () => `The proof differs from what the verifier expects.`,
      lewat: () => `Not run.`
    },
    alasan: {
      asli: "The proof matches the public key in the certificate.",
      klon: "This chip cannot recover the key from the enrolled PUF.",
      klon_daftar: "The fake chip does not hold the key paired with the certificate.",
      ulang: "An old proof is not valid for a new challenge."
    },
    ringkas: (r) => `${fmt(r.ms_total, 1)} ms in this browser`,
    perangkat: { paspor: "Autogate · Immigration", ijazah: "Diploma check", cukai: "Excise scanner", akses: "Door B2" },
    layar: {
      siaga: "Ready", siagaRinci: "Tap the chip on the reader",
      pabrik: "Personalising", pabrikRinci: "PUF, key, and certificate",
      dekatkan: "Reading chip", membaca: "Checking certificate", nyala: "Powering chip",
      pufGagal: "Key not recovered", pufGagalRinci: "Helper data rejected",
      tantangan: "Sending challenge", cocok: "Comparing proof",
      asli: { paspor: "PLEASE PROCEED", ijazah: "GENUINE DIPLOMA", cukai: "GENUINE STAMP", akses: "ACCESS GRANTED" },
      palsu: { paspor: "REJECTED", ijazah: "FORGED DIPLOMA", cukai: "FORGED STAMP", akses: "DENIED" },
      asliRinci: "Proof matches the certificate", palsuRinci: "Contact an officer"
    },
    holo: {
      sertifikat: "CERTIFICATE · LMS", ttd: "issuer signature", sah: "VALID", tidakSah: "INVALID",
      pufDaftar: "PUF · enrolling 768 oscillators", kunciDibuat: "ML-KEM-768 key formed", pufBaca: "PUF · measuring 768 oscillators",
      kunciPulih: "PUF key recovered", kunciGagal: "Key not recovered", percobaan: "PUF · attempt", pufSendiri: "Fake chip PUF", kunciLain: "Different key, not the certificate's"
    },
    tag: { klon: "Cloned chip", tiruan: "Fake chip", perekam: "Recorder", buktiLama: "old proof" },
    kasus: {
      paspor: ["E-passport", "Immigration autogate, Terminal 3"],
      ijazah: ["Digital diploma", "HR app during hiring"],
      cukai: ["Excise stamp", "Inspector at the distributor warehouse"],
      akses: ["Access card", "Server room door B2"]
    }
  }
};

const KASUS = [
  { id: "paspor", w: "var(--biru)", benih: 101, chip: "PASPOR-B1234567", konteks: "autogate|CGK-T3|gerbang-07",
    data: { id: [["Pemegang", "Contoh Pemegang"], ["Jenis", "Paspor biasa 48 halaman"]], en: [["Holder", "Sample Holder"], ["Type", "Ordinary passport, 48 pages"]] } },
  { id: "ijazah", w: "var(--ok)", benih: 102, chip: "IJAZAH-2026-00417", konteks: "verifikasi-ijazah|hrd|lowongan-221",
    data: { id: [["Lulusan", "Contoh Lulusan"], ["Program", "S1 Teknik Perangkat Lunak"]], en: [["Graduate", "Sample Graduate"], ["Program", "BSc Software Engineering"]] } },
  { id: "cukai", w: "var(--oranye)", benih: 103, chip: "CUKAI-HT-7731905", konteks: "inspeksi-cukai|gudang-cikarang|lot-88",
    data: { id: [["Barang", "Hasil tembakau, 1 slop"], ["Seri pita", "HT-7731905"]], en: [["Goods", "Tobacco product, 1 carton"], ["Stamp series", "HT-7731905"]] } },
  { id: "akses", w: "var(--ungu)", benih: 104, chip: "AKSES-DC-0092", konteks: "pintu|ruang-server-b2|shift-malam",
    data: { id: [["Pemilik", "Contoh Karyawan"], ["Hak", "Ruang server lantai B2"]], en: [["Owner", "Sample Employee"], ["Access", "Server room, level B2"]] } }
];
const SKENARIO = [
  { id: "asli", w: "var(--ok)" },
  { id: "klon", w: "var(--oranye)" },
  { id: "klon_daftar", w: "var(--gagal)" },
  { id: "ulang", w: "var(--ungu)" }
];
const LANGKAH = ["sertifikat", "nyala", "tantangan", "bukti", "cocokkan"];

let bahasa = new URLSearchParams(location.search).get("bahasa") === "en" ? "en" : "id";
let aktif = KASUS.find((k) => k.id === location.hash.slice(1)) || KASUS[0];
let penerbitSiap = false;
let sibuk = false;
const terbit = {};
const terakhir = {};
const direkam = {};
const $ = (s) => document.querySelector(s);
const t = (k) => T[bahasa][k];

function fmt(n, d = 0) {
  return Number(n).toLocaleString(bahasa === "id" ? "id-ID" : "en-US", { maximumFractionDigits: d, minimumFractionDigits: d });
}

if (modeRekam) {
  document.body.classList.add("rekam");
  new MutationObserver(() => window.__kotor && window.__kotor()).observe(document.body, { subtree: true, childList: true, characterData: true, attributes: true });
}

const pekerja = new Worker("pekerja.js");
let urut = 0;
const menunggu = new Map();
pekerja.onmessage = (e) => {
  const { id, ok, hasil, galat } = e.data;
  const p = menunggu.get(id);
  menunggu.delete(id);
  ok ? p.resolve(hasil) : p.reject(new Error(galat));
};
function panggil(fn, ...args) {
  return new Promise((resolve, reject) => {
    const id = ++urut;
    menunggu.set(id, { resolve, reject });
    pekerja.postMessage({ id, fn, args });
  });
}

const status = {
  get t() {
    return T[bahasa];
  },
  idChip: () => aktif.chip
};

function terjemah() {
  document.documentElement.lang = bahasa;
  document.querySelectorAll("[data-t]").forEach((el) => (el.textContent = t(el.dataset.t)));
  document.querySelectorAll("[data-bahasa]").forEach((b) => b.setAttribute("aria-pressed", b.dataset.bahasa === bahasa));
  $("#penerbitStatus").textContent = penerbitSiap ? t("siap") : t("menyiapkan");
}

function isiKasus() {
  $("#kasus").innerHTML = KASUS.map((k) => `<button role="tab" data-kasus="${k.id}" aria-selected="${k === aktif}" style="--w:${k.w}" ${sibuk ? "disabled" : ""}><i></i><span>${t("kasus")[k.id][0]}</span></button>`).join("");
  $("#kasus").querySelectorAll("button").forEach((b) => b.addEventListener("click", () => pilih(b.dataset.kasus)));
}

function isiKiri() {
  const [judul, tempat] = t("kasus")[aktif.id];
  $("#kepalaKiri").style.setProperty("--w", aktif.w);
  $("#judulKasus").textContent = judul;
  $("#pemeriksa").textContent = tempat;
  $("#data").innerHTML = aktif.data[bahasa].map(([a, b]) => `<dt>${a}</dt><dd>${b}</dd>`).join("") + `<dt>${t("idChip")}</dt><dd class="mono">${aktif.chip}</dd>`;
  const r = terbit[aktif.id];
  const tb = $("#terbitkan");
  tb.textContent = r ? t("sudahTerbit") : penerbitSiap ? t("terbitkan") : t("menungguPenerbit");
  tb.disabled = !!r || !penerbitSiap || sibuk;
  const ul = $("#pabrik");
  if (r) {
    ul.innerHTML = [["var(--oranye)", t("pab1"), t("pab1d")(r)], ["var(--ok)", t("pab2"), t("pab2d")(r)], ["var(--ungu)", t("pab3"), t("pab3d")(r)]]
      .map(([w, a, b]) => `<li><i style="--w:${w}"></i><div><b>${a}</b><span>${b}</span></div></li>`).join("");
    ul.classList.add("tampil");
  } else {
    ul.classList.remove("tampil");
    ul.innerHTML = "";
  }
  $("#pilihan").innerHTML = SKENARIO.map((s) => {
    const [a, b] = t("sk")[s.id];
    return `<button data-mode="${s.id}" style="--w:${s.w}" ${!r || sibuk ? "disabled" : ""}><b>${a}</b><span>${b}</span></button>`;
  }).join("");
  $("#pilihan").querySelectorAll("button").forEach((b) => b.addEventListener("click", () => periksa(b.dataset.mode)));
  const akhir = terakhir[aktif.id];
  if (akhir) $("#pilihan").querySelector(`[data-mode="${akhir.mode}"]`)?.classList.add("aktif");
}

function teksLangkah(kunci, r) {
  const d = t("d");
  const s = r.langkah.find((x) => x.langkah === kunci);
  if (!s) return [d.lewat(), "lewat"];
  const data = { id: aktif.chip, ...r };
  if (kunci === "sertifikat") return [d.sertifikat(data), s.ok ? "ok" : "gagal"];
  if (kunci === "nyala") {
    if (r.mode === "klon_daftar") return [d.nyalaSendiri(), "gagal"];
    return s.ok ? [d.nyalaOk(data), "ok"] : [d.nyalaGagal(), "gagal"];
  }
  if (kunci === "tantangan") return [d.tantangan(data), "ok"];
  if (kunci === "bukti") return r.mode === "ulang" ? [d.buktiUlang(), "gagal"] : [d.bukti(data), "ok"];
  return s.ok ? [d.cocokOk(), "ok"] : [d.cocokGagal(), "gagal"];
}

function tulisJejak(r, selesai, jalan = -1) {
  $("#kosong").hidden = true;
  const ol = $("#jejak");
  ol.hidden = false;
  ol.innerHTML = LANGKAH.map((k, i) => {
    let kelas = "", teks = "", wkt = "";
    if (r && i < selesai) {
      [teks, kelas] = teksLangkah(k, r);
      const s = r.langkah.find((x) => x.langkah === k);
      if (s) wkt = `${fmt(s.ms, 1)} ms`;
    } else if (i === jalan) kelas = "jalan";
    const simbol = kelas === "ok" ? "✓" : kelas === "gagal" ? "✕" : kelas === "lewat" ? "·" : i + 1;
    return `<li class="${kelas}"><span class="ikon">${simbol}</span><div><b>${t("l")[k]}</b><p>${teks}</p></div><span class="wkt">${wkt}</span></li>`;
  }).join("");
}

function heks(a, b) {
  let out = "";
  for (let i = 0; i < a.length; i += 2) {
    const x = a.slice(i, i + 2);
    out += `<i class="${b && b.slice(i, i + 2) === x ? "sama" : "beda"}">${x}</i>`;
  }
  return out;
}

function tulisBanding(r) {
  if (r && r.bukti) {
    $("#hBukti").innerHTML = heks(r.bukti, r.harapan);
    $("#hHarapan").innerHTML = heks(r.harapan, r.bukti);
    $("#banding").classList.add("tampil");
  } else $("#banding").classList.remove("tampil");
}

function isiKanan() {
  const r = terakhir[aktif.id];
  $("#konteks").textContent = r ? r.konteks : aktif.konteks + "|...";
  if (!r) {
    $("#kosong").hidden = false;
    $("#kosong").textContent = terbit[aktif.id] ? t("kosongSiap") : t("kosongBelum");
    $("#jejak").hidden = true;
    tulisBanding(null);
    return;
  }
  tulisJejak(r, LANGKAH.length);
  tulisBanding(r);
}

function render() {
  terjemah();
  isiKasus();
  isiKiri();
  isiKanan();
}

const elPutusan = $("#putusan");
async function tampilPutusan(r) {
  const asli = r.putusan === "ASLI";
  elPutusan.className = "putusan kaca " + (asli ? "asli" : "palsu");
  $("#cap1").textContent = asli ? t("asli") : t("palsu");
  $("#alasan").textContent = t("alasan")[r.mode];
  $("#ringkas").textContent = t("ringkas")(r);
  await animasi(420, (k) => {
    elPutusan.style.opacity = k;
    elPutusan.style.transform = `translateX(-50%) scale(${0.9 + 0.1 * k})`;
  }, mudah.pegas);
}
function sembunyiPutusan() {
  elPutusan.style.opacity = 0;
}

function kunci(v) {
  sibuk = v;
  isiKasus();
  isiKiri();
}

let dunia;

async function pilih(id) {
  if (sibuk || id === aktif.id) return;
  aktif = KASUS.find((k) => k.id === id);
  sembunyiPutusan();
  history.replaceState(null, "", location.pathname + location.search + "#" + id);
  sibuk = true;
  render();
  await dunia.pilihKasus(id);
  sibuk = false;
  render();
}

async function terbitkan() {
  if (sibuk || terbit[aktif.id] || !penerbitSiap) return;
  const k = aktif;
  kunci(true);
  $("#terbitkan").textContent = t("menerbitkan");
  try {
    const [r] = await Promise.all([panggil("personalisasi", k.id, k.chip, k.benih), dunia.terbitkan()]);
    terbit[k.id] = r;
  } catch (e) {
    console.error(e);
  }
  kunci(false);
  isiKanan();
}

async function periksa(mode) {
  if (sibuk || !terbit[aktif.id]) return;
  const k = aktif;
  kunci(true);
  sembunyiPutusan();
  $("#pilihan").querySelectorAll("button").forEach((b) => b.classList.toggle("aktif", b.dataset.mode === mode));
  const waktu = new Date().toISOString().slice(0, 19).replace("T", " ");
  const konteks = `${k.konteks}|${waktu}`;
  $("#konteks").textContent = konteks;
  tulisBanding(null);
  tulisJejak(null, 0, -1);
  try {
    if (mode === "ulang" && !direkam[k.id]) {
      await panggil("periksa", k.id, "asli", `${k.konteks}|rekaman`);
      direkam[k.id] = true;
    }
    const kerja = panggil("periksa", k.id, mode, konteks);
    await dunia.mulaiPeriksa(mode);
    const r = await kerja;
    r.mode = mode;
    r.konteks = konteks;
    if (mode === "asli") direkam[k.id] = true;
    const ada = (n) => r.langkah.find((x) => x.langkah === n);
    tulisJejak(r, 0, 0);
    await dunia.langkahSertifikat(ada("sertifikat").ok);
    tulisJejak(r, 1, 1);
    await dunia.langkahNyala({ mode, nyalaOk: ada("nyala").ok });
    if (ada("tantangan")) {
      tulisJejak(r, 2, 2);
      await dunia.langkahTantangan(r.panjang_sandi);
      tulisJejak(r, 3, 3);
      await dunia.langkahBukti(mode === "ulang");
      tulisJejak(r, 4, 4);
      await dunia.langkahCocok();
    }
    tulisJejak(r, LANGKAH.length);
    tulisBanding(r);
    terakhir[k.id] = r;
    await Promise.all([tampilPutusan(r), dunia.putusan(r.putusan === "ASLI")]);
  } catch (e) {
    console.error(e);
  }
  kunci(false);
}

const elCap = $("#cap");
window.__cap = async (teks, warna) => {
  if (!teks) {
    await animasi(300, (k) => (elCap.style.opacity = 1 - k));
    return;
  }
  if (Number(elCap.style.opacity) > 0) await animasi(200, (k) => (elCap.style.opacity = 1 - k));
  elCap.textContent = teks;
  elCap.style.borderLeftColor = warna || "#2fd67b";
  await animasi(300, (k) => (elCap.style.opacity = k));
};

window.__sim = {
  get siap() {
    return penerbitSiap;
  },
  get sibuk() {
    return sibuk;
  },
  pilih,
  terbitkan,
  periksa,
  bahasa: (b) => {
    bahasa = b;
    render();
    dunia.segarkanLayar();
  },
  terbang: (nama, ms) => dunia.terbang(dunia.kamera()[nama], ms)
};

document.querySelectorAll("[data-bahasa]").forEach((b) => b.addEventListener("click", () => {
  bahasa = b.dataset.bahasa;
  render();
  dunia && dunia.segarkanLayar();
}));
$("#terbitkan").addEventListener("click", terbitkan);
$("#bukaInfo").addEventListener("click", () => $("#modal").classList.add("buka"));
$("#tutupInfo").addEventListener("click", () => $("#modal").classList.remove("buka"));
$("#modal").addEventListener("click", (e) => e.target.id === "modal" && $("#modal").classList.remove("buka"));

render();
dunia = await buatDunia($("#dunia"), status);
await dunia.pilihKasus(aktif.id, true);
$("#muat").classList.add("hilang");
window.__dunia = true;

panggil("siapkan").then((r) => {
  penerbitSiap = true;
  $("#penerbit").classList.add("siap");
  $("#penerbitKunci").textContent = r.kunci_publik.slice(0, 12) + "...";
  render();
}).catch(() => ($("#penerbitStatus").textContent = t("galatMuat")));
