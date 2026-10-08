const rekam = new URLSearchParams(location.search).has("rekam");
let virtual = 0;
const pengatur = [];
const pendengar = new Set();

export const modeRekam = rekam;

export function sekarang() {
  return rekam ? virtual : performance.now();
}

export function tunggu(ms) {
  return new Promise((resolve) => pengatur.push({ pada: sekarang() + ms, resolve }));
}

export const mudah = {
  linear: (p) => p,
  keluar: (p) => 1 - Math.pow(1 - p, 3),
  masuk: (p) => p * p * p,
  halus: (p) => (p < 0.5 ? 4 * p * p * p : 1 - Math.pow(-2 * p + 2, 3) / 2),
  pegas: (p) => 1 + 2.2 * Math.pow(p - 1, 3) + 1.2 * Math.pow(p - 1, 2)
};

export function animasi(durasi, langkah, ease = mudah.halus) {
  return new Promise((resolve) => {
    const mulai = sekarang();
    const a = { mulai, durasi, langkah, ease, resolve };
    pendengar.add(a);
    langkah(0);
  });
}

export function setiapBingkai(fn) {
  const a = { terus: fn };
  pendengar.add(a);
  return () => pendengar.delete(a);
}

export function detak() {
  const t = sekarang();
  for (let i = pengatur.length - 1; i >= 0; i--) {
    if (pengatur[i].pada <= t) {
      const p = pengatur.splice(i, 1)[0];
      p.resolve();
    }
  }
  for (const a of [...pendengar]) {
    if (a.terus) {
      a.terus(t);
      continue;
    }
    const p = Math.min(1, (t - a.mulai) / a.durasi);
    a.langkah(a.ease(p));
    if (p >= 1) {
      pendengar.delete(a);
      a.resolve();
    }
  }
}

export function adaAnimasi() {
  for (const a of pendengar) if (!a.terus) return true;
  return false;
}

export function majukan(ms) {
  virtual += ms;
}
