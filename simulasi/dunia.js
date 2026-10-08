import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { RoomEnvironment } from "three/addons/environments/RoomEnvironment.js";
import { RoundedBoxGeometry } from "three/addons/geometries/RoundedBoxGeometry.js";
import { EffectComposer } from "three/addons/postprocessing/EffectComposer.js";
import { RenderPass } from "three/addons/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/addons/postprocessing/UnrealBloomPass.js";
import { OutputPass } from "three/addons/postprocessing/OutputPass.js";
import { animasi, tunggu, mudah, setiapBingkai, detak, sekarang, majukan, modeRekam, adaAnimasi } from "./jam.js";

const SANS = '"IBM Plex Sans", system-ui, sans-serif';
const MONO = '"IBM Plex Mono", ui-monospace, monospace';
const WARNA = { siaga: "#4aa3ff", ok: "#2fd67b", gagal: "#ff4d4d", oranye: "#ff9a3c", ungu: "#a98bff", toska: "#33d1c6" };

function tekstur(w, h, gambar) {
  const c = document.createElement("canvas");
  c.width = w;
  c.height = h;
  const g = c.getContext("2d");
  gambar(g, w, h);
  const t = new THREE.CanvasTexture(c);
  t.colorSpace = THREE.SRGBColorSpace;
  t.anisotropy = 8;
  t.userData.kanvas = c;
  t.userData.gambar = (fn) => {
    g.clearRect(0, 0, w, h);
    fn(g, w, h);
    t.needsUpdate = true;
  };
  return t;
}

function derau(g, w, h, kuat, jumlah) {
  for (let i = 0; i < jumlah; i++) {
    const a = Math.random() * kuat;
    g.fillStyle = Math.random() < 0.5 ? `rgba(0,0,0,${a})` : `rgba(255,255,255,${a})`;
    g.fillRect(Math.random() * w, Math.random() * h, 2, 2);
  }
}

function bulat(g, x, y, w, h, r) {
  g.beginPath();
  g.moveTo(x + r, y);
  g.arcTo(x + w, y, x + w, y + h, r);
  g.arcTo(x + w, y + h, x, y + h, r);
  g.arcTo(x, y + h, x, y, r);
  g.arcTo(x, y, x + w, y, r);
  g.closePath();
}

function cahayaBulat() {
  return tekstur(128, 128, (g) => {
    const r = g.createRadialGradient(64, 64, 0, 64, 64, 64);
    r.addColorStop(0, "rgba(255,255,255,1)");
    r.addColorStop(0.25, "rgba(255,255,255,.55)");
    r.addColorStop(1, "rgba(255,255,255,0)");
    g.fillStyle = r;
    g.fillRect(0, 0, 128, 128);
  });
}

const BAHAN = {};
function bahan() {
  if (BAHAN.siap) return BAHAN;
  Object.assign(BAHAN, {
    siap: true,
    logam: new THREE.MeshStandardMaterial({ color: 0xc3c9d1, metalness: 0.9, roughness: 0.3 }),
    logamGelap: new THREE.MeshStandardMaterial({ color: 0x3a414b, metalness: 0.75, roughness: 0.38 }),
    hitam: new THREE.MeshStandardMaterial({ color: 0x14171c, metalness: 0.25, roughness: 0.42 }),
    hitamDop: new THREE.MeshStandardMaterial({ color: 0x1b1f25, metalness: 0.05, roughness: 0.8 }),
    putih: new THREE.MeshStandardMaterial({ color: 0xe9ecef, metalness: 0.05, roughness: 0.55 }),
    kaca: new THREE.MeshPhysicalMaterial({ color: 0xbfe3f5, metalness: 0, roughness: 0.08, transparent: true, opacity: 0.14, envMapIntensity: 0.5, side: THREE.DoubleSide }),
    kacaGelap: new THREE.MeshPhysicalMaterial({ color: 0x0b1520, metalness: 0.1, roughness: 0.22, clearcoat: 0.6, clearcoatRoughness: 0.2, envMapIntensity: 0.3 }),
    emas: new THREE.MeshStandardMaterial({ color: 0xd8b25a, metalness: 1, roughness: 0.28 })
  });
  return BAHAN;
}

function kotak(w, h, d, mat, r = 0.004, seg = 3) {
  const m = new THREE.Mesh(new RoundedBoxGeometry(w, h, d, seg, Math.min(r, w / 2, h / 2, d / 2)), mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

function bidang(w, h, map, opsi = {}) {
  const mat = opsi.cahaya
    ? new THREE.MeshBasicMaterial({ map, toneMapped: false, transparent: !!opsi.transparan })
    : new THREE.MeshStandardMaterial({ map, roughness: opsi.kasar ?? 0.6, metalness: opsi.logam ?? 0, transparent: !!opsi.transparan });
  const m = new THREE.Mesh(new THREE.PlaneGeometry(w, h), mat);
  m.receiveShadow = !opsi.cahaya;
  return m;
}

function gambarLayar(g, w, h, d) {
  const warna = d.warna || WARNA.siaga;
  const bg = g.createLinearGradient(0, 0, 0, h);
  bg.addColorStop(0, "#0d1622");
  bg.addColorStop(1, "#060a10");
  g.fillStyle = bg;
  g.fillRect(0, 0, w, h);
  g.fillStyle = warna;
  g.globalAlpha = 0.14;
  g.fillRect(0, 0, w, h);
  g.globalAlpha = 1;
  g.fillStyle = warna;
  g.fillRect(0, 0, w, Math.round(h * 0.035));
  const s = w / 512;
  g.fillStyle = "rgba(220,230,240,.75)";
  g.font = `600 ${22 * s}px ${SANS}`;
  g.textBaseline = "top";
  g.fillText(d.judul || "", 26 * s, 28 * s);
  g.textAlign = "right";
  g.fillStyle = "rgba(220,230,240,.45)";
  g.font = `500 ${18 * s}px ${MONO}`;
  g.fillText("GEMBOK", w - 26 * s, 30 * s);
  g.textAlign = "left";
  if (d.ikon === "ok" || d.ikon === "gagal") {
    const cx = 26 * s + 44 * s, cy = h * 0.52;
    g.beginPath();
    g.arc(cx, cy, 40 * s, 0, Math.PI * 2);
    g.fillStyle = warna;
    g.fill();
    g.strokeStyle = "#071018";
    g.lineWidth = 9 * s;
    g.lineCap = "round";
    g.beginPath();
    if (d.ikon === "ok") {
      g.moveTo(cx - 18 * s, cy + 1 * s);
      g.lineTo(cx - 5 * s, cy + 14 * s);
      g.lineTo(cx + 20 * s, cy - 13 * s);
    } else {
      g.moveTo(cx - 15 * s, cy - 15 * s);
      g.lineTo(cx + 15 * s, cy + 15 * s);
      g.moveTo(cx + 15 * s, cy - 15 * s);
      g.lineTo(cx - 15 * s, cy + 15 * s);
    }
    g.stroke();
  } else if (d.ikon === "putar") {
    const cx = 26 * s + 44 * s, cy = h * 0.52, a = (sekarang() / 180) % (Math.PI * 2);
    g.strokeStyle = warna;
    g.lineWidth = 8 * s;
    g.lineCap = "round";
    g.beginPath();
    g.arc(cx, cy, 34 * s, a, a + Math.PI * 1.4);
    g.stroke();
  }
  const x0 = d.ikon ? 26 * s + 108 * s : 26 * s;
  const muat = w - x0 - 22 * s;
  let ukuran = (d.besar || 44) * s;
  g.font = `700 ${ukuran}px ${SANS}`;
  while (g.measureText(d.status || "").width > muat && ukuran > 10) {
    ukuran -= 1;
    g.font = `700 ${ukuran}px ${SANS}`;
  }
  g.fillStyle = "#f2f6fa";
  g.textBaseline = "middle";
  g.fillText(d.status || "", x0, h * 0.47);
  let kecil = 21 * s;
  g.font = `500 ${kecil}px ${SANS}`;
  while (g.measureText(d.rinci || "").width > muat && kecil > 8) {
    kecil -= 1;
    g.font = `500 ${kecil}px ${SANS}`;
  }
  g.fillStyle = "rgba(220,230,240,.7)";
  g.fillText(d.rinci || "", x0, h * 0.47 + Math.max(ukuran * 0.62, 30 * s) + 8 * s);
  if (d.kaki) {
    g.fillStyle = "rgba(220,230,240,.4)";
    g.font = `500 ${16 * s}px ${MONO}`;
    g.textBaseline = "bottom";
    g.fillText(d.kaki, 26 * s, h - 20 * s);
  }
}

function labelSprite(teks, warna, tinggi = 0.022) {
  const t = tekstur(512, 96, (g, w, h) => {
    g.font = `600 44px ${SANS}`;
    const lebar = Math.min(w - 8, g.measureText(teks).width + 70);
    const x = (w - lebar) / 2;
    bulat(g, x, 8, lebar, h - 16, 38);
    g.fillStyle = "rgba(8,12,18,.86)";
    g.fill();
    g.lineWidth = 4;
    g.strokeStyle = warna;
    g.stroke();
    g.fillStyle = warna;
    g.beginPath();
    g.arc(x + 32, h / 2, 9, 0, Math.PI * 2);
    g.fill();
    g.fillStyle = "#f3f6fa";
    g.textBaseline = "middle";
    g.fillText(teks, x + 52, h / 2 + 2);
  });
  const s = new THREE.Sprite(new THREE.SpriteMaterial({ map: t, depthTest: false, transparent: true, toneMapped: false }));
  s.scale.set(tinggi * (512 / 96), tinggi, 1);
  s.renderOrder = 10;
  return s;
}

function lantaiUbin(warna1, warna2, ukuran, ulang) {
  const t = tekstur(1024, 1024, (g, w, h) => {
    g.fillStyle = warna1;
    g.fillRect(0, 0, w, h);
    derau(g, w, h, 0.05, 9000);
    g.strokeStyle = warna2;
    g.lineWidth = 3;
    const n = ukuran;
    for (let i = 0; i <= n; i++) {
      g.beginPath();
      g.moveTo((i * w) / n, 0);
      g.lineTo((i * w) / n, h);
      g.stroke();
      g.beginPath();
      g.moveTo(0, (i * h) / n);
      g.lineTo(w, (i * h) / n);
      g.stroke();
    }
  });
  t.wrapS = t.wrapT = THREE.RepeatWrapping;
  t.repeat.set(ulang, ulang);
  return t;
}

function beton(warna, ulang) {
  const t = tekstur(1024, 1024, (g, w, h) => {
    g.fillStyle = warna;
    g.fillRect(0, 0, w, h);
    for (let i = 0; i < 60; i++) {
      const r = g.createRadialGradient(Math.random() * w, Math.random() * h, 0, Math.random() * w, Math.random() * h, 160 + Math.random() * 200);
      r.addColorStop(0, `rgba(0,0,0,${Math.random() * 0.05})`);
      r.addColorStop(1, "rgba(0,0,0,0)");
      g.fillStyle = r;
      g.fillRect(0, 0, w, h);
    }
    derau(g, w, h, 0.08, 30000);
  });
  t.wrapS = t.wrapT = THREE.RepeatWrapping;
  t.repeat.set(ulang, ulang);
  return t;
}

function pose(obj) {
  return { p: obj.position.clone(), q: obj.quaternion.clone() };
}

function poseDari(p, rx = 0, ry = 0, rz = 0) {
  return { p: new THREE.Vector3(...p), q: new THREE.Quaternion().setFromEuler(new THREE.Euler(rx, ry, rz)) };
}

function gerakKe(obj, tujuan, durasi, ease = mudah.halus) {
  const a = pose(obj);
  return animasi(durasi, (k) => {
    obj.position.lerpVectors(a.p, tujuan.p, k);
    obj.quaternion.slerpQuaternions(a.q, tujuan.q, k);
  }, ease);
}

function gerakBusur(obj, tujuan, durasi, angkat = 0.08) {
  const a = pose(obj);
  return animasi(durasi, (k) => {
    obj.position.lerpVectors(a.p, tujuan.p, k);
    obj.position.y += Math.sin(k * Math.PI) * angkat;
    obj.quaternion.slerpQuaternions(a.q, tujuan.q, k);
  });
}

function layarPerangkat(lebar, tinggi, resolusi = 512) {
  const t = tekstur(resolusi, Math.round(resolusi * (tinggi / lebar)), (g, w, h) => gambarLayar(g, w, h, { judul: "", status: "" }));
  const m = bidang(lebar, tinggi, t, { cahaya: true });
  m.userData.tulis = (d) => t.userData.gambar((g, w, h) => gambarLayar(g, w, h, d));
  return m;
}

function lampuLed(geo, warna) {
  const mat = new THREE.MeshBasicMaterial({ color: new THREE.Color(warna).multiplyScalar(4), toneMapped: false });
  const m = new THREE.Mesh(geo, mat);
  m.userData.atur = (w, kuat = 4) => mat.color.set(w).multiplyScalar(kuat);
  return m;
}

function sampulPaspor() {
  return tekstur(560, 800, (g, w, h) => {
    const bg = g.createLinearGradient(0, 0, w, h);
    bg.addColorStop(0, "#173a59");
    bg.addColorStop(1, "#0d2539");
    g.fillStyle = bg;
    g.fillRect(0, 0, w, h);
    derau(g, w, h, 0.06, 26000);
    const emas = g.createLinearGradient(0, 0, w, 0);
    emas.addColorStop(0, "#b38b3a");
    emas.addColorStop(0.5, "#f0d78e");
    emas.addColorStop(1, "#b38b3a");
    g.fillStyle = emas;
    g.strokeStyle = emas;
    g.textAlign = "center";
    g.font = `600 30px ${SANS}`;
    g.fillText("REPUBLIK INDONESIA", w / 2, 120);
    g.lineWidth = 5;
    g.beginPath();
    g.arc(w / 2, 330, 92, 0, Math.PI * 2);
    g.stroke();
    g.lineWidth = 2;
    g.beginPath();
    g.arc(w / 2, 330, 78, 0, Math.PI * 2);
    g.stroke();
    g.beginPath();
    for (let i = 0; i < 5; i++) {
      const a = -Math.PI / 2 + (i * 4 * Math.PI) / 5;
      g.lineTo(w / 2 + Math.cos(a) * 48, 330 + Math.sin(a) * 48);
    }
    g.closePath();
    g.fill();
    g.font = `700 64px ${SANS}`;
    g.fillText("PASPOR", w / 2, 550);
    g.font = `500 30px ${SANS}`;
    g.fillText("PASSPORT", w / 2, 598);
    g.lineWidth = 4;
    bulat(g, w / 2 - 44, 668, 88, 56, 6);
    g.stroke();
    g.beginPath();
    g.arc(w / 2, 696, 15, 0, Math.PI * 2);
    g.stroke();
    g.beginPath();
    g.moveTo(w / 2 - 44, 682);
    g.lineTo(w / 2 - 22, 682);
    g.moveTo(w / 2 + 22, 682);
    g.lineTo(w / 2 + 44, 682);
    g.moveTo(w / 2 - 44, 710);
    g.lineTo(w / 2 - 22, 710);
    g.moveTo(w / 2 + 22, 710);
    g.lineTo(w / 2 + 44, 710);
    g.stroke();
  });
}

function kertasIjazah() {
  return tekstur(1188, 840, (g, w, h) => {
    g.fillStyle = "#fbf7ec";
    g.fillRect(0, 0, w, h);
    derau(g, w, h, 0.03, 30000);
    g.strokeStyle = "#2e6b4a";
    g.lineWidth = 10;
    g.strokeRect(34, 34, w - 68, h - 68);
    g.lineWidth = 2;
    g.strokeRect(54, 54, w - 108, h - 108);
    g.fillStyle = "#23402f";
    g.textAlign = "center";
    g.font = `600 30px Georgia, serif`;
    g.fillText("UNIVERSITAS CONTOH NUSANTARA", w / 2, 150);
    g.font = `700 86px Georgia, serif`;
    g.fillText("IJAZAH", w / 2, 270);
    g.font = `italic 30px Georgia, serif`;
    g.fillText("diberikan kepada", w / 2, 340);
    g.font = `700 50px Georgia, serif`;
    g.fillText("Contoh Lulusan", w / 2, 420);
    g.font = `28px Georgia, serif`;
    g.fillText("Sarjana Komputer, Teknik Perangkat Lunak", w / 2, 480);
    g.fillStyle = "#c9c2ad";
    for (let i = 0; i < 3; i++) g.fillRect(w / 2 - 300 + i * 30, 540 + i * 34, 600 - i * 60, 8);
    g.strokeStyle = "#23402f";
    g.lineWidth = 2;
    g.beginPath();
    g.moveTo(200, 720);
    g.lineTo(440, 720);
    g.stroke();
    g.fillStyle = "#2e6b4a";
    g.beginPath();
    g.arc(w - 250, 690, 64, 0, Math.PI * 2);
    g.fill();
    g.fillStyle = "#fbf7ec";
    g.font = `700 22px ${SANS}`;
    g.fillText("RESMI", w - 250, 698);
    const hol = g.createLinearGradient(w - 150, 620, w - 70, 760);
    hol.addColorStop(0, "#d6c6ff");
    hol.addColorStop(0.35, "#b9f3e6");
    hol.addColorStop(0.7, "#ffe2a8");
    hol.addColorStop(1, "#c6d8ff");
    g.fillStyle = hol;
    bulat(g, w - 150, 640, 90, 110, 10);
    g.fill();
    g.fillStyle = "#d8b25a";
    bulat(g, w - 128, 668, 46, 38, 5);
    g.fill();
    g.strokeStyle = "#8c6c24";
    g.lineWidth = 2;
    g.strokeRect(w - 114, 676, 18, 22);
  });
}

function pitaCukai() {
  return tekstur(200, 1100, (g, w, h) => {
    const bg = g.createLinearGradient(0, 0, w, h);
    bg.addColorStop(0, "#f6c7a2");
    bg.addColorStop(0.5, "#ffe0c6");
    bg.addColorStop(1, "#f3b98c");
    g.fillStyle = bg;
    g.fillRect(0, 0, w, h);
    g.strokeStyle = "rgba(160,70,20,.35)";
    g.lineWidth = 1.5;
    for (let y = 0; y < h; y += 9) {
      g.beginPath();
      for (let x = 0; x <= w; x += 8) g.lineTo(x, y + Math.sin(x / 14 + y / 30) * 3);
      g.stroke();
    }
    g.fillStyle = "#8a3d10";
    g.save();
    g.translate(w / 2, h / 2);
    g.rotate(-Math.PI / 2);
    g.textAlign = "center";
    g.font = `700 50px ${SANS}`;
    g.fillText("PITA CUKAI", -230, 18);
    g.font = `600 34px ${MONO}`;
    g.fillText("HT-7731905", 250, 14);
    g.restore();
    const hol = g.createLinearGradient(30, h / 2 - 70, 170, h / 2 + 70);
    hol.addColorStop(0, "#d6c6ff");
    hol.addColorStop(0.5, "#b9f3e6");
    hol.addColorStop(1, "#ffe2a8");
    g.fillStyle = hol;
    bulat(g, 30, h / 2 - 70, 140, 140, 14);
    g.fill();
    g.fillStyle = "#d8b25a";
    bulat(g, 66, h / 2 - 34, 68, 68, 8);
    g.fill();
    g.strokeStyle = "#8c6c24";
    g.lineWidth = 3;
    g.strokeRect(86, h / 2 - 20, 28, 40);
  });
}

function kartuAkses() {
  return tekstur(856, 540, (g, w, h) => {
    g.fillStyle = "#f7f6fb";
    g.fillRect(0, 0, w, h);
    const hd = g.createLinearGradient(0, 0, w, 0);
    hd.addColorStop(0, "#4b2f8f");
    hd.addColorStop(1, "#6f52c4");
    g.fillStyle = hd;
    g.fillRect(0, 0, w, 130);
    g.fillStyle = "#fff";
    g.font = `700 46px ${SANS}`;
    g.fillText("GEMBOK DC", 44, 82);
    g.font = `500 26px ${SANS}`;
    g.textAlign = "right";
    g.fillText("KARTU AKSES", w - 44, 80);
    g.textAlign = "left";
    g.fillStyle = "#e3def3";
    bulat(g, 44, 170, 190, 240, 14);
    g.fill();
    g.fillStyle = "#b9addd";
    g.beginPath();
    g.arc(139, 262, 50, 0, Math.PI * 2);
    g.fill();
    g.beginPath();
    g.ellipse(139, 400, 80, 70, 0, Math.PI, 0);
    g.fill();
    g.fillStyle = "#2a2f3a";
    g.font = `700 40px ${SANS}`;
    g.fillText("Contoh Karyawan", 270, 220);
    g.fillStyle = "#6a7180";
    g.font = `500 28px ${SANS}`;
    g.fillText("Ruang server, lantai B2", 270, 268);
    g.font = `500 26px ${MONO}`;
    g.fillText("AKSES-DC-0092", 270, 312);
    g.fillStyle = "#d8b25a";
    bulat(g, w - 190, 360, 120, 96, 14);
    g.fill();
    g.strokeStyle = "#8c6c24";
    g.lineWidth = 3;
    g.beginPath();
    g.moveTo(w - 190, 392);
    g.lineTo(w - 70, 392);
    g.moveTo(w - 190, 424);
    g.lineTo(w - 70, 424);
    g.moveTo(w - 130, 360);
    g.lineTo(w - 130, 456);
    g.stroke();
  });
}

let bahanKardus = null;
function kardus(w, h, d) {
  const g = new THREE.Group();
  if (!bahanKardus) {
    const t = tekstur(512, 512, (c, W, H) => {
      c.fillStyle = "#b98d5a";
      c.fillRect(0, 0, W, H);
      derau(c, W, H, 0.08, 20000);
      c.fillStyle = "rgba(90,60,30,.25)";
      for (let y = 0; y < H; y += 6) c.fillRect(0, y, W, 1);
    });
    bahanKardus = {
      badan: new THREE.MeshStandardMaterial({ map: t, roughness: 0.92 }),
      lak: new THREE.MeshStandardMaterial({ color: 0xc8a776, roughness: 0.35, metalness: 0.1 })
    };
  }
  const b = kotak(w, h, d, bahanKardus.badan, 0.006, 2);
  g.add(b);
  const lak = new THREE.Mesh(new THREE.PlaneGeometry(w * 0.18, d + 0.002), bahanKardus.lak);
  lak.rotation.x = -Math.PI / 2;
  lak.position.y = h / 2 + 0.0008;
  g.add(lak);
  return g;
}

function bangunAutogate(s) {
  const B = bahan();
  const grup = new THREE.Group();
  const lantai = new THREE.Mesh(new THREE.PlaneGeometry(14, 14), new THREE.MeshStandardMaterial({ map: lantaiUbin("#8f979f", "#7a828b", 8, 7), roughness: 0.38, metalness: 0.05 }));
  lantai.rotation.x = -Math.PI / 2;
  lantai.receiveShadow = true;
  grup.add(lantai);
  const dinding = new THREE.Mesh(new THREE.PlaneGeometry(14, 5), new THREE.MeshStandardMaterial({ color: 0x2b3440, roughness: 0.9 }));
  dinding.position.set(0, 2.5, -3.2);
  grup.add(dinding);
  const langit = tekstur(1024, 512, (g, w, h) => {
    const l = g.createLinearGradient(0, 0, 0, h);
    l.addColorStop(0, "#2b5b8a");
    l.addColorStop(0.6, "#8fb6d6");
    l.addColorStop(1, "#d7e6f0");
    g.fillStyle = l;
    g.fillRect(0, 0, w, h);
    for (let i = 0; i < 14; i++) {
      g.fillStyle = `rgba(255,255,255,${0.08 + Math.random() * 0.1})`;
      g.beginPath();
      g.ellipse(Math.random() * w, h * (0.15 + Math.random() * 0.35), 120 + Math.random() * 160, 18 + Math.random() * 14, 0, 0, Math.PI * 2);
      g.fill();
    }
    g.fillStyle = "rgba(40,50,62,.9)";
    let x = 0;
    while (x < w) {
      const lebar = 30 + Math.random() * 70, tinggi = 40 + Math.random() * 120;
      g.fillRect(x, h * 0.86 - tinggi, lebar, tinggi + h * 0.14);
      x += lebar + 4;
    }
    g.fillStyle = "rgba(70,80,92,1)";
    g.fillRect(0, h * 0.86, w, h * 0.14);
  });
  for (let i = -2; i <= 2; i++) {
    const jendela = bidang(2.5, 2.6, langit, { cahaya: true });
    jendela.material.color.setScalar(0.85);
    jendela.position.set(i * 2.6, 1.9, -3.18);
    grup.add(jendela);
    const tiang = kotak(0.08, 3, 0.08, B.logamGelap, 0.01);
    tiang.position.set(i * 2.6 + 1.3, 1.5, -3.12);
    grup.add(tiang);
  }
  const pita = new THREE.Mesh(new THREE.BoxGeometry(14, 0.12, 0.1), B.logamGelap);
  pita.position.set(0, 3.2, -3.12);
  grup.add(pita);

  const kabinet = (x) => {
    const k = new THREE.Group();
    const badan = kotak(0.3, 1.0, 1.5, B.logam, 0.03);
    badan.position.y = 0.5;
    k.add(badan);
    const atas = kotak(0.32, 0.03, 1.52, B.hitam, 0.012);
    atas.position.y = 1.01;
    k.add(atas);
    const led = lampuLed(new THREE.BoxGeometry(0.02, 0.012, 1.4), WARNA.siaga);
    led.position.set(x > 0 ? -0.155 : 0.155, 0.96, 0);
    k.add(led);
    k.position.x = x;
    k.userData.led = led;
    return k;
  };
  const kiri = kabinet(-0.48);
  const kanan = kabinet(0.48);
  grup.add(kiri, kanan);

  const daun = [];
  for (const sisi of [-1, 1]) {
    const engsel = new THREE.Group();
    engsel.position.set(sisi * 0.33, 0.55, 0.05);
    const panel = new THREE.Mesh(new RoundedBoxGeometry(0.31, 0.85, 0.014, 2, 0.006), B.kaca);
    panel.position.x = -sisi * 0.155;
    const tepi = new THREE.Mesh(new THREE.BoxGeometry(0.31, 0.02, 0.016), B.logamGelap);
    tepi.position.set(-sisi * 0.155, 0.42, 0);
    engsel.add(panel, tepi);
    grup.add(engsel);
    daun.push({ engsel, sisi });
  }

  const pembaca = new THREE.Group();
  const alas = kotak(0.26, 0.07, 0.22, B.hitam, 0.012);
  pembaca.add(alas);
  const kacaBaca = new THREE.Mesh(new RoundedBoxGeometry(0.2, 0.006, 0.16, 2, 0.003), B.kacaGelap);
  kacaBaca.position.y = 0.037;
  pembaca.add(kacaBaca);
  const ring = lampuLed(new THREE.TorusGeometry(0.105, 0.002, 6, 64), WARNA.siaga);
  ring.rotation.x = Math.PI / 2;
  ring.scale.set(1, 0.8, 1);
  ring.position.y = 0.041;
  pembaca.add(ring);
  pembaca.position.set(0.48, 1.07, 0.52);
  pembaca.rotation.x = 0.28;
  grup.add(pembaca);

  const tiangLayar = kotak(0.03, 0.34, 0.03, B.logamGelap, 0.008);
  tiangLayar.position.set(0.48, 1.19, 0.3);
  grup.add(tiangLayar);
  const bingkai = kotak(0.3, 0.2, 0.025, B.hitam, 0.01);
  bingkai.position.set(0.48, 1.42, 0.3);
  bingkai.rotation.x = -0.12;
  grup.add(bingkai);
  const layar = layarPerangkat(0.28, 0.18);
  layar.position.set(0, 0, 0.0131);
  bingkai.add(layar);

  const papan = kotak(1.6, 0.26, 0.08, B.hitam, 0.02);
  papan.position.set(0, 2.25, -0.2);
  grup.add(papan);
  const tulisan = tekstur(1024, 160, (g, w, h) => {
    g.fillStyle = "#06090d";
    g.fillRect(0, 0, w, h);
    g.fillStyle = "#3dff9a";
    g.font = `700 76px ${SANS}`;
    g.textBaseline = "middle";
    g.fillText("➜", 40, h / 2 + 4);
    g.fillStyle = "#f2f6fa";
    g.font = `600 66px ${SANS}`;
    g.fillText("AUTOGATE", 150, h / 2 + 4);
    g.fillStyle = "#ffd25a";
    g.font = `600 44px ${SANS}`;
    g.textAlign = "right";
    g.fillText("WNI · INDONESIAN", w - 40, h / 2 + 4);
  });
  const papanTulis = bidang(1.54, 0.22, tulisan, { cahaya: true });
  papanTulis.position.set(0, 2.25, -0.159);
  grup.add(papanTulis);
  const tiangPapan = kotak(0.04, 1.2, 0.04, B.logamGelap, 0.01);
  tiangPapan.position.set(-0.7, 2.9, -0.2);
  const tiangPapan2 = tiangPapan.clone();
  tiangPapan2.position.x = 0.7;
  grup.add(tiangPapan, tiangPapan2);

  const garis = new THREE.Mesh(new THREE.PlaneGeometry(0.66, 0.06), new THREE.MeshStandardMaterial({ color: 0xf2c230, roughness: 0.5 }));
  garis.rotation.x = -Math.PI / 2;
  garis.position.set(0, 0.002, 1.05);
  grup.add(garis);

  const paspor = new THREE.Group();
  const isi = kotak(0.088, 0.008, 0.125, new THREE.MeshStandardMaterial({ color: 0xf2efe6, roughness: 0.8 }), 0.003, 2);
  const sampul = kotak(0.09, 0.0016, 0.127, new THREE.MeshStandardMaterial({ color: 0x143450, roughness: 0.55 }), 0.0008, 1);
  sampul.position.y = 0.0045;
  const muka = bidang(0.088, 0.125, sampulPaspor(), { kasar: 0.5, logam: 0.15 });
  muka.rotation.x = -Math.PI / 2;
  muka.position.y = 0.0055;
  const belakang = sampul.clone();
  belakang.position.y = -0.0045;
  paspor.add(isi, sampul, muka, belakang);
  grup.add(paspor);
  const diam = poseDari([0.72, 1.22, 0.86], -1.05, 0.35, 0.12);
  const baca = poseDari([0.48, 1.118, 0.52], 0.28, 0, 0);
  paspor.position.copy(diam.p);
  paspor.quaternion.copy(diam.q);

  return {
    grup,
    kamera: {
      awal: { pos: [2.3, 1.85, 2.9], target: [0.2, 1.0, 0.2] },
      periksa: { pos: [1.45, 1.72, 1.55], target: [0.4, 1.22, 0.36] },
      dekat: { pos: [0.86, 1.4, 0.86], target: [0.47, 1.16, 0.46] }
    },
    penggerak: paspor,
    diam,
    baca,
    titikChip: () => paspor.localToWorld(new THREE.Vector3(0, 0, 0.02)),
    titikPembaca: () => pembaca.localToWorld(new THREE.Vector3(0, 0.05, -0.05)),
    titikPerekam: poseDari([0.68, 1.1, 0.5], 0, -0.5, 0),
    layar: (d) => layar.userData.tulis({ judul: s.t.perangkat.paspor, ...d }),
    lampu: (w) => {
      ring.userData.atur(w);
      kiri.userData.led.userData.atur(w);
      kanan.userData.led.userData.atur(w);
    },
    async reaksi(asli) {
      if (!asli) return;
      await animasi(900, (k) => daun.forEach(({ engsel, sisi }) => (engsel.rotation.y = sisi * k * 1.45)), mudah.keluar);
    },
    async pulihkan() {
      const a = daun[0].engsel.rotation.y;
      if (Math.abs(a) > 0.01) await animasi(700, (k) => daun.forEach(({ engsel, sisi }) => (engsel.rotation.y = sisi * Math.abs(a) * (1 - k))));
    },
    lampuSiaga: WARNA.siaga
  };
}

function bangunHrd(s) {
  const B = bahan();
  const grup = new THREE.Group();
  const lantai = new THREE.Mesh(new THREE.PlaneGeometry(12, 12), new THREE.MeshStandardMaterial({ map: lantaiUbin("#8a7458", "#7a654c", 24, 6), roughness: 0.55 }));
  lantai.rotation.x = -Math.PI / 2;
  lantai.receiveShadow = true;
  grup.add(lantai);
  const dinding = new THREE.Mesh(new THREE.PlaneGeometry(12, 4), new THREE.MeshStandardMaterial({ color: 0xd9d4ca, roughness: 0.95 }));
  dinding.position.set(0, 2, -1.2);
  dinding.receiveShadow = true;
  grup.add(dinding);
  const kayu = tekstur(1024, 512, (g, w, h) => {
    g.fillStyle = "#7a5534";
    g.fillRect(0, 0, w, h);
    for (let i = 0; i < 160; i++) {
      g.strokeStyle = `rgba(${40 + Math.random() * 40},${25 + Math.random() * 20},10,${0.08 + Math.random() * 0.12})`;
      g.lineWidth = 1 + Math.random() * 3;
      g.beginPath();
      const y = Math.random() * h;
      g.moveTo(0, y);
      for (let x = 0; x <= w; x += 32) g.lineTo(x, y + Math.sin(x / 90 + i) * 6);
      g.stroke();
    }
  });
  const meja = kotak(1.5, 0.04, 0.78, new THREE.MeshStandardMaterial({ map: kayu, roughness: 0.42, metalness: 0.02 }), 0.012);
  meja.position.set(0, 0.75, 0);
  grup.add(meja);
  for (const [x, z] of [[-0.7, -0.34], [0.7, -0.34], [-0.7, 0.34], [0.7, 0.34]]) {
    const kaki = kotak(0.04, 0.73, 0.04, B.logamGelap, 0.01);
    kaki.position.set(x, 0.365, z);
    grup.add(kaki);
  }
  const rak = kotak(1.2, 0.03, 0.26, new THREE.MeshStandardMaterial({ map: kayu, roughness: 0.5 }), 0.008);
  rak.position.set(-0.2, 1.55, -1.07);
  grup.add(rak);
  const warnaBuku = [0x2a6496, 0xb3261e, 0x2e7d4f, 0xc2621a, 0x5b3f99, 0x5f6b7a, 0x16808a];
  for (let i = 0; i < 14; i++) {
    const tinggi = 0.2 + Math.random() * 0.08;
    const buku = kotak(0.035, tinggi, 0.18, new THREE.MeshStandardMaterial({ color: warnaBuku[i % 7], roughness: 0.7 }), 0.004, 1);
    buku.position.set(-0.7 + i * 0.045, 1.565 + tinggi / 2, -1.06);
    grup.add(buku);
  }
  const pot = kotak(0.16, 0.2, 0.16, new THREE.MeshStandardMaterial({ color: 0xece6da, roughness: 0.6 }), 0.03);
  pot.position.set(-0.6, 0.87, -0.2);
  grup.add(pot);
  for (let i = 0; i < 9; i++) {
    const daun = new THREE.Mesh(new THREE.SphereGeometry(0.06, 12, 8), new THREE.MeshStandardMaterial({ color: 0x3f7a3a, roughness: 0.7 }));
    daun.scale.set(0.5, 1.6, 0.25);
    const a = (i / 9) * Math.PI * 2;
    daun.position.set(-0.6 + Math.cos(a) * 0.05, 1.06 + Math.random() * 0.06, -0.2 + Math.sin(a) * 0.05);
    daun.rotation.set(Math.sin(a) * 0.5, a, Math.cos(a) * 0.5);
    daun.castShadow = true;
    grup.add(daun);
  }

  const laptop = new THREE.Group();
  const basis = kotak(0.34, 0.016, 0.24, B.logam, 0.008);
  basis.position.y = 0.008;
  laptop.add(basis);
  const papanKetik = new THREE.Mesh(new THREE.PlaneGeometry(0.28, 0.1), B.hitamDop);
  papanKetik.rotation.x = -Math.PI / 2;
  papanKetik.position.set(0, 0.0165, -0.03);
  laptop.add(papanKetik);
  const engsel = new THREE.Group();
  engsel.position.set(0, 0.016, -0.118);
  engsel.rotation.x = -0.32;
  const tutup = kotak(0.34, 0.23, 0.008, B.logam, 0.008);
  tutup.position.y = 0.115;
  engsel.add(tutup);
  const layar = layarPerangkat(0.31, 0.195, 768);
  layar.position.set(0, 0.117, 0.0042);
  engsel.add(layar);
  laptop.add(engsel);
  laptop.position.set(0.18, 0.77, -0.08);
  laptop.rotation.y = -0.18;
  grup.add(laptop);

  const pembaca = new THREE.Group();
  const puck = kotak(0.1, 0.022, 0.13, B.hitam, 0.01);
  puck.position.y = 0.011;
  pembaca.add(puck);
  const ring = lampuLed(new THREE.TorusGeometry(0.034, 0.0018, 6, 48), WARNA.siaga);
  ring.rotation.x = Math.PI / 2;
  ring.position.y = 0.0225;
  pembaca.add(ring);
  const kabel = new THREE.Mesh(new THREE.TubeGeometry(new THREE.CatmullRomCurve3([new THREE.Vector3(0.0, 0.006, -0.065), new THREE.Vector3(0.02, 0.004, -0.12), new THREE.Vector3(0.08, 0.004, -0.14), new THREE.Vector3(0.12, 0.004, -0.13)]), 24, 0.003, 6), B.hitam);
  pembaca.add(kabel);
  pembaca.position.set(-0.17, 0.77, 0.06);
  grup.add(pembaca);

  const ijazah = new THREE.Group();
  const kertas = kotak(0.297, 0.0012, 0.21, new THREE.MeshStandardMaterial({ color: 0xd9d3c4, roughness: 0.9 }), 0.0005, 1);
  const muka = bidang(0.297, 0.21, kertasIjazah(), { kasar: 0.9 });
  muka.material.color.setScalar(0.82);
  muka.rotation.x = -Math.PI / 2;
  muka.position.y = 0.0007;
  ijazah.add(kertas, muka);
  grup.add(ijazah);
  const diam = poseDari([-0.38, 0.7715, 0.2], 0, 0.22, 0);
  const baca = poseDari([-0.27, 0.795, 0.12], 0, 0.04, 0);
  ijazah.position.copy(diam.p);
  ijazah.quaternion.copy(diam.q);

  return {
    grup,
    kamera: {
      awal: { pos: [1.15, 1.6, 1.45], target: [-0.08, 0.8, 0.0] },
      periksa: { pos: [-0.12, 1.42, 1.08], target: [0.02, 0.82, 0.0] },
      dekat: { pos: [0.18, 1.12, 0.52], target: [-0.12, 0.82, 0.06] }
    },
    penggerak: ijazah,
    diam,
    baca,
    titikChip: () => ijazah.localToWorld(new THREE.Vector3(0.105, 0, 0.05)),
    titikPembaca: () => pembaca.localToWorld(new THREE.Vector3(0, 0.03, 0)),
    titikPerekam: poseDari([-0.02, 0.77, 0.2], 0, 0.6, 0),
    layar: (d) => layar.userData.tulis({ judul: s.t.perangkat.ijazah, ...d }),
    lampu: (w) => ring.userData.atur(w),
    async reaksi() {},
    async pulihkan() {},
    lampuSiaga: WARNA.siaga
  };
}

function bangunGudang(s) {
  const B = bahan();
  const grup = new THREE.Group();
  const lantai = new THREE.Mesh(new THREE.PlaneGeometry(16, 16), new THREE.MeshStandardMaterial({ map: beton("#8d8f8f", 4), roughness: 0.72 }));
  lantai.rotation.x = -Math.PI / 2;
  lantai.receiveShadow = true;
  grup.add(lantai);
  const garis = new THREE.Mesh(new THREE.PlaneGeometry(0.1, 16), new THREE.MeshStandardMaterial({ color: 0xe7c22c, roughness: 0.6 }));
  garis.rotation.x = -Math.PI / 2;
  garis.position.set(-1.1, 0.002, 0);
  grup.add(garis);
  const dinding = new THREE.Mesh(new THREE.PlaneGeometry(16, 6), new THREE.MeshStandardMaterial({ color: 0x5a6068, roughness: 0.9 }));
  dinding.position.set(0, 3, -3.5);
  grup.add(dinding);
  const tegak = new THREE.MeshStandardMaterial({ color: 0x2c5aa0, roughness: 0.5, metalness: 0.4 });
  const balok = new THREE.MeshStandardMaterial({ color: 0xe0701e, roughness: 0.45, metalness: 0.35 });
  for (const zRak of [-2.2]) {
    for (let i = 0; i < 4; i++) {
      const x = -2.4 + i * 1.6;
      for (const dz of [-0.45, 0.45]) {
        const t = kotak(0.07, 3, 0.07, tegak, 0.006);
        t.position.set(x, 1.5, zRak + dz);
        grup.add(t);
      }
      if (i < 3) {
        for (const y of [0.15, 1.05, 1.95, 2.85]) {
          for (const dz of [-0.45, 0.45]) {
            const b = kotak(1.53, 0.08, 0.05, balok, 0.006);
            b.position.set(x + 0.8, y, zRak + dz);
            grup.add(b);
          }
          if (y < 2.8) {
            const n = 3;
            for (let k = 0; k < n; k++) {
              if (Math.random() < 0.15) continue;
              const tinggi = 0.4 + Math.random() * 0.25;
              const kd = kardus(0.42, tinggi, 0.7);
              kd.position.set(x + 0.3 + k * 0.5, y + 0.04 + tinggi / 2, zRak);
              kd.rotation.y = (Math.random() - 0.5) * 0.08;
              grup.add(kd);
            }
          }
        }
      }
    }
  }
  const lampuGantung = (x, z) => {
    const kap = new THREE.Mesh(new THREE.ConeGeometry(0.22, 0.16, 24, 1, true), new THREE.MeshStandardMaterial({ color: 0x2b2f35, metalness: 0.6, roughness: 0.4, side: THREE.DoubleSide }));
    kap.position.set(x, 3.0, z);
    const bohlam = lampuLed(new THREE.CircleGeometry(0.17, 24), "#fff4dc");
    bohlam.rotation.x = Math.PI / 2;
    bohlam.position.set(x, 2.93, z);
    grup.add(kap, bohlam);
  };
  lampuGantung(-0.6, -0.4);
  lampuGantung(1.4, -1.6);
  lampuGantung(-2.4, -1.6);

  const meja = new THREE.Group();
  const atas = kotak(1.1, 0.04, 0.65, B.logam, 0.008);
  atas.position.y = 0.88;
  meja.add(atas);
  for (const [x, z] of [[-0.5, -0.28], [0.5, -0.28], [-0.5, 0.28], [0.5, 0.28]]) {
    const k = kotak(0.04, 0.86, 0.04, B.logamGelap, 0.008);
    k.position.set(x, 0.43, z);
    meja.add(k);
  }
  grup.add(meja);
  const kotakBarang = kardus(0.42, 0.26, 0.3);
  kotakBarang.position.set(-0.1, 1.03, 0.02);
  kotakBarang.rotation.y = 0.12;
  grup.add(kotakBarang);
  const pita = bidang(0.045, 0.3, pitaCukai(), { kasar: 0.35, logam: 0.2 });
  pita.rotation.x = -Math.PI / 2;
  pita.position.set(0.05, 0.131, 0);
  kotakBarang.add(pita);
  for (let i = 0; i < 2; i++) {
    const lain = kardus(0.42, 0.26, 0.3);
    lain.position.set(0.82 + i * 0.05, 0.13 + i * 0.26, -0.7);
    lain.rotation.y = -0.2 + i * 0.15;
    grup.add(lain);
  }

  const pemindai = new THREE.Group();
  const badan = kotak(0.075, 0.03, 0.17, new THREE.MeshStandardMaterial({ color: 0x2c3138, roughness: 0.55, metalness: 0.2 }), 0.012);
  pemindai.add(badan);
  const karet = kotak(0.079, 0.012, 0.05, new THREE.MeshStandardMaterial({ color: 0xe0701e, roughness: 0.7 }), 0.005);
  karet.position.set(0, -0.004, -0.075);
  pemindai.add(karet);
  const gagang = kotak(0.04, 0.11, 0.04, new THREE.MeshStandardMaterial({ color: 0x2c3138, roughness: 0.6 }), 0.012);
  gagang.position.set(0, -0.055, 0.04);
  gagang.rotation.x = -0.35;
  pemindai.add(gagang);
  const layar = layarPerangkat(0.058, 0.075, 384);
  layar.rotation.x = -Math.PI / 2;
  layar.position.set(0, 0.0152, 0.03);
  pemindai.add(layar);
  const lensa = lampuLed(new THREE.PlaneGeometry(0.05, 0.012), WARNA.siaga);
  lensa.rotation.x = Math.PI / 2;
  lensa.position.set(0, -0.0153, -0.06);
  pemindai.add(lensa);
  const indikator = lampuLed(new THREE.BoxGeometry(0.05, 0.004, 0.006), WARNA.siaga);
  indikator.position.set(0, 0.016, -0.02);
  pemindai.add(indikator);
  grup.add(pemindai);
  const diam = poseDari([0.36, 0.955, 0.2], 0, -0.4, 0);
  const baca = poseDari([-0.08, 1.235, 0.05], 0.22, 0.1, 0);
  pemindai.position.copy(diam.p);
  pemindai.quaternion.copy(diam.q);

  return {
    grup,
    kamera: {
      awal: { pos: [1.35, 1.65, 1.6], target: [-0.05, 1.0, -0.1] },
      periksa: { pos: [0.78, 1.62, 1.08], target: [-0.05, 1.12, 0.0] },
      dekat: { pos: [0.4, 1.42, 0.6], target: [-0.06, 1.16, 0.02] }
    },
    penggerak: pemindai,
    tagInduk: kotakBarang,
    tagTinggi: 0.34,
    diam,
    baca,
    titikChip: () => kotakBarang.localToWorld(new THREE.Vector3(0.05, 0.135, 0)),
    titikPembaca: () => pemindai.localToWorld(new THREE.Vector3(0, -0.016, -0.06)),
    titikPerekam: poseDari([0.12, 1.16, 0.18], 0, -0.3, 0),
    layar: (d) => layar.userData.tulis({ judul: s.t.perangkat.cukai, besar: 52, ...d }),
    lampu: (w) => {
      lensa.userData.atur(w);
      indikator.userData.atur(w);
    },
    async reaksi() {},
    async pulihkan() {},
    lampuSiaga: WARNA.siaga
  };
}

function bangunPintu(s) {
  const B = bahan();
  const grup = new THREE.Group();
  const lantai = new THREE.Mesh(new THREE.PlaneGeometry(12, 12), new THREE.MeshStandardMaterial({ map: lantaiUbin("#565c64", "#4a5058", 10, 6), roughness: 0.4, metalness: 0.1 }));
  lantai.rotation.x = -Math.PI / 2;
  lantai.receiveShadow = true;
  grup.add(lantai);
  const dindingMat = new THREE.MeshStandardMaterial({ color: 0x8d939a, roughness: 0.9 });
  const kiri = kotak(3, 3, 0.15, dindingMat, 0.002);
  kiri.position.set(-1.95, 1.5, 0);
  const kanan = kotak(3, 3, 0.15, dindingMat, 0.002);
  kanan.position.set(1.95, 1.5, 0);
  const atas = kotak(0.9, 0.85, 0.15, dindingMat, 0.002);
  atas.position.set(0, 2.575, 0);
  grup.add(kiri, kanan, atas);
  const kusen = new THREE.MeshStandardMaterial({ color: 0x3a4048, metalness: 0.6, roughness: 0.4 });
  for (const x of [-0.46, 0.46]) {
    const k = kotak(0.04, 2.15, 0.18, kusen, 0.006);
    k.position.set(x, 1.075, 0);
    grup.add(k);
  }
  const kAtas = kotak(0.96, 0.04, 0.18, kusen, 0.006);
  kAtas.position.set(0, 2.17, 0);
  grup.add(kAtas);

  const ruang = new THREE.Group();
  const lantaiDalam = new THREE.Mesh(new THREE.PlaneGeometry(3, 3), new THREE.MeshStandardMaterial({ color: 0x1b2129, roughness: 0.6 }));
  lantaiDalam.rotation.x = -Math.PI / 2;
  lantaiDalam.position.set(0, 0.003, -1.6);
  ruang.add(lantaiDalam);
  const dindingDalam = new THREE.Mesh(new THREE.PlaneGeometry(4, 3), new THREE.MeshStandardMaterial({ color: 0x10151c, roughness: 0.9 }));
  dindingDalam.position.set(0, 1.5, -3.0);
  ruang.add(dindingDalam);
  const ledServer = [];
  for (let i = 0; i < 4; i++) {
    const rak = kotak(0.6, 2.0, 0.8, new THREE.MeshStandardMaterial({ color: 0x15191f, roughness: 0.5, metalness: 0.5 }), 0.01);
    rak.position.set(-0.95 + i * 0.63, 1.0, -2.3);
    ruang.add(rak);
    for (let j = 0; j < 14; j++) {
      const unit = new THREE.Mesh(new THREE.PlaneGeometry(0.5, 0.09), new THREE.MeshStandardMaterial({ color: 0x232a33, roughness: 0.4, metalness: 0.6 }));
      unit.position.set(-0.95 + i * 0.63, 0.2 + j * 0.125, -1.899);
      ruang.add(unit);
      for (let k = 0; k < 3; k++) {
        const led = lampuLed(new THREE.PlaneGeometry(0.008, 0.008), Math.random() < 0.7 ? "#2fd67b" : "#4aa3ff");
        led.position.set(-0.95 + i * 0.63 + 0.17 + k * 0.025, 0.2 + j * 0.125, -1.897);
        led.userData.fase = Math.random() * 10;
        ledServer.push(led);
        ruang.add(led);
      }
    }
  }
  const biru = new THREE.PointLight(0x3a7bff, 6, 6, 2);
  biru.position.set(0, 2.4, -1.6);
  ruang.add(biru);
  grup.add(ruang);
  const hentiKedip = setiapBingkai((t) => {
    for (const l of ledServer) l.visible = Math.sin(t / 90 + l.userData.fase * 7) > -0.6;
  });

  const engsel = new THREE.Group();
  engsel.position.set(-0.44, 0, 0.0);
  const daun = kotak(0.88, 2.12, 0.05, new THREE.MeshStandardMaterial({ color: 0x8d949c, metalness: 0.55, roughness: 0.38 }), 0.008);
  daun.position.set(0.44, 1.06, 0);
  engsel.add(daun);
  const jendela = new THREE.Mesh(new THREE.PlaneGeometry(0.12, 0.6), B.kacaGelap);
  jendela.position.set(0.24, 1.45, 0.026);
  engsel.add(jendela);
  const pegangan = kotak(0.025, 0.3, 0.04, B.logam, 0.008);
  pegangan.position.set(0.8, 1.05, 0.05);
  engsel.add(pegangan);
  const papan = tekstur(512, 160, (g, w, h) => {
    g.fillStyle = "#1d3f73";
    g.fillRect(0, 0, w, h);
    g.fillStyle = "#fff";
    g.font = `700 46px ${SANS}`;
    g.textAlign = "center";
    g.fillText("RUANG SERVER", w / 2, 72);
    g.font = `500 30px ${SANS}`;
    g.fillText("B2 · KHUSUS PETUGAS", w / 2, 122);
  });
  const plakat = bidang(0.36, 0.11, papan, { kasar: 0.4 });
  plakat.position.set(0.44, 1.78, 0.026);
  engsel.add(plakat);
  grup.add(engsel);

  const pembaca = new THREE.Group();
  const pelat = kotak(0.085, 0.14, 0.025, B.hitam, 0.008);
  pembaca.add(pelat);
  const layar = layarPerangkat(0.068, 0.05, 384);
  layar.position.set(0, 0.035, 0.0131);
  pembaca.add(layar);
  const ring = lampuLed(new THREE.TorusGeometry(0.022, 0.0016, 6, 48), WARNA.siaga);
  ring.position.set(0, -0.025, 0.0131);
  pembaca.add(ring);
  pembaca.position.set(0.68, 1.22, 0.088);
  grup.add(pembaca);

  const kartu = new THREE.Group();
  const badan = kotak(0.0856, 0.054, 0.0009, new THREE.MeshStandardMaterial({ color: 0xf7f6fb, roughness: 0.4 }), 0.003, 2);
  const muka = bidang(0.0856, 0.054, kartuAkses(), { kasar: 0.35 });
  muka.position.z = 0.0006;
  kartu.add(badan, muka);
  grup.add(kartu);
  const diam = poseDari([0.86, 1.08, 0.52], -0.3, -0.5, 0.1);
  const baca = poseDari([0.68, 1.197, 0.112], 0, 0, 0);
  kartu.position.copy(diam.p);
  kartu.quaternion.copy(diam.q);

  return {
    grup,
    kamera: {
      awal: { pos: [2.0, 1.65, 3.1], target: [0.2, 1.1, 0] },
      periksa: { pos: [1.95, 1.58, 1.65], target: [0.55, 1.18, 0.12] },
      dekat: { pos: [0.92, 1.3, 0.62], target: [0.62, 1.2, 0.1] }
    },
    penggerak: kartu,
    diam,
    baca,
    titikChip: () => kartu.localToWorld(new THREE.Vector3(0.025, -0.01, 0.002)),
    titikPembaca: () => pembaca.localToWorld(new THREE.Vector3(0, -0.02, 0.02)),
    titikPerekam: poseDari([0.56, 1.02, 0.2], 0.2, -0.3, 0),
    layar: (d) => layar.userData.tulis({ judul: s.t.perangkat.akses, besar: 58, ...d }),
    lampu: (w) => ring.userData.atur(w),
    async reaksi(asli) {
      if (!asli) return;
      await animasi(1200, (k) => (engsel.rotation.y = -k * 1.25), mudah.keluar);
    },
    async pulihkan() {
      const a = engsel.rotation.y;
      if (Math.abs(a) > 0.01) await animasi(800, (k) => (engsel.rotation.y = a * (1 - k)));
    },
    buang: hentiKedip,
    lampuSiaga: WARNA.siaga
  };
}

const PEMBANGUN = { paspor: bangunAutogate, ijazah: bangunHrd, cukai: bangunGudang, akses: bangunPintu };
const SUASANA = {
  paspor: { latar: 0x1a222c, kabut: [0x1a222c, 7, 16], hemi: [0xdfe9ff, 0x404650, 0.55], matahari: [0xffffff, 1.7, [3, 6, 4]], pajanan: 0.75 },
  ijazah: { latar: 0x2a2520, kabut: [0x2a2520, 5, 12], hemi: [0xfff1dd, 0x4a3f33, 0.5], matahari: [0xffe6c4, 1.6, [-2, 4, 3]], pajanan: 0.85 },
  cukai: { latar: 0x1d2024, kabut: [0x1d2024, 6, 15], hemi: [0xfff6e6, 0x3a3d42, 0.5], matahari: [0xfff1d8, 1.7, [1, 6, 2]], pajanan: 0.9 },
  akses: { latar: 0x171b21, kabut: [0x171b21, 6, 14], hemi: [0xe6eeff, 0x3a3f46, 0.45], matahari: [0xffffff, 1.5, [2.5, 5, 4]], pajanan: 0.8 }
};

export async function buatDunia(kanvas, s) {
  await document.fonts.ready;
  try {
    await Promise.all([document.fonts.load(`600 40px ${SANS}`), document.fonts.load(`700 40px ${SANS}`), document.fonts.load(`500 20px ${MONO}`)]);
  } catch (e) {}
  const q = new URLSearchParams(location.search);
  const mati = new Set((q.get("tanpa") || "").split(","));
  const renderer = new THREE.WebGLRenderer({ canvas: kanvas, antialias: !mati.has("aa"), powerPreference: "high-performance" });
  renderer.setPixelRatio(modeRekam ? Number(q.get("skala") || 1) : Math.min(window.devicePixelRatio, 2));
  renderer.shadowMap.enabled = !mati.has("bayang");
  renderer.shadowMap.type = mati.has("lembut") ? THREE.PCFShadowMap : THREE.PCFSoftShadowMap;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  const scene = new THREE.Scene();
  const pmrem = new THREE.PMREMGenerator(renderer);
  scene.environment = pmrem.fromScene(new RoomEnvironment(renderer), 0.04).texture;
  const kamera = new THREE.PerspectiveCamera(36, 1, 0.01, 60);
  const kontrol = new OrbitControls(kamera, kanvas);
  kontrol.enableDamping = true;
  kontrol.dampingFactor = 0.08;
  kontrol.minDistance = 0.3;
  kontrol.maxDistance = 6;
  kontrol.maxPolarAngle = Math.PI * 0.49;
  const hemi = new THREE.HemisphereLight(0xffffff, 0x444444, 0.8);
  const matahari = new THREE.DirectionalLight(0xffffff, 2);
  matahari.castShadow = true;
  matahari.shadow.mapSize.set(2048, 2048);
  matahari.shadow.camera.left = -3;
  matahari.shadow.camera.right = 3;
  matahari.shadow.camera.top = 3;
  matahari.shadow.camera.bottom = -3;
  matahari.shadow.camera.near = 0.5;
  matahari.shadow.camera.far = 20;
  matahari.shadow.bias = -0.0004;
  matahari.shadow.normalBias = 0.02;
  scene.add(hemi, matahari, matahari.target);

  const composer = new EffectComposer(renderer);
  composer.addPass(new RenderPass(scene, kamera));
  const bloom = new UnrealBloomPass(new THREE.Vector2(256, 256), 0.7, 0.5, 1.6);
  if (!mati.has("bloom")) composer.addPass(bloom);
  composer.addPass(new OutputPass());

  const cahaya = cahayaBulat();
  const efek = new THREE.Group();
  scene.add(efek);

  let kasus = null;
  let aktifId = null;
  let tagAktif = null;
  let perekam = null;

  function ukur() {
    const w = kanvas.clientWidth, h = kanvas.clientHeight;
    renderer.setSize(w, h, false);
    composer.setSize(w, h);
    kamera.aspect = w / h;
    const sempit = w < 900;
    kamera.fov = sempit ? 46 : 36;
    if (!sempit) {
      const kiri = document.querySelector("aside.kiri"), kanan = document.querySelector("aside.kanan");
      const lk = kiri && kiri.offsetParent ? kiri.offsetWidth + 16 : 0;
      const lr = kanan && kanan.offsetParent ? kanan.offsetWidth + 16 : 0;
      kamera.setViewOffset(w, h, (lr - lk) / 2, 0, w, h);
    } else kamera.clearViewOffset();
    kamera.updateProjectionMatrix();
    if (window.__kotor) window.__kotor();
  }
  new ResizeObserver(ukur).observe(kanvas);
  ukur();

  function render() {
    kontrol.update();
    composer.render();
  }

  if (modeRekam) {
    const piksel = new Uint8Array(4);
    let kotor = true;
    window.__kotor = () => (kotor = true);
    window.__maju = (ms, n = 1) => {
      let gerak = false;
      for (let i = 0; i < n; i++) {
        gerak = gerak || adaAnimasi();
        majukan(ms);
        detak();
        gerak = gerak || adaAnimasi();
      }
      if (!gerak && !kotor) return false;
      kotor = false;
      render();
      const gl = renderer.getContext();
      gl.readPixels(0, 0, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, piksel);
      return true;
    };
  } else {
    const putar = () => {
      detak();
      render();
      requestAnimationFrame(putar);
    };
    requestAnimationFrame(putar);
  }

  function aturKamera(k) {
    kamera.position.set(...k.pos);
    kontrol.target.set(...k.target);
  }

  function terbang(k, durasi = 1400) {
    const p0 = kamera.position.clone(), t0 = kontrol.target.clone();
    const p1 = new THREE.Vector3(...k.pos), t1 = new THREE.Vector3(...k.target);
    kontrol.enabled = false;
    return animasi(durasi, (e) => {
      kamera.position.lerpVectors(p0, p1, e);
      kontrol.target.lerpVectors(t0, t1, e);
    }).then(() => (kontrol.enabled = true));
  }

  function bersihEfek() {
    while (efek.children.length) efek.remove(efek.children[0]);
  }

  function papan(lebar = 0.2, tinggi = 0.117, cw = 512, ch = 300) {
    const t = tekstur(cw, ch, () => {});
    const m = new THREE.Mesh(new THREE.PlaneGeometry(lebar, tinggi), new THREE.MeshBasicMaterial({ map: t, transparent: true, depthWrite: false, toneMapped: false, blending: THREE.AdditiveBlending, opacity: 0 }));
    m.renderOrder = 8;
    m.userData.tulis = (fn) => t.userData.gambar(fn);
    return m;
  }

  function hologramSertifikat(ok, id, posisi) {
    const m = papan();
    m.userData.tulis((g, w, h) => {
      const c = ok ? WARNA.ungu : WARNA.gagal;
      g.strokeStyle = c;
      g.lineWidth = 5;
      bulat(g, 6, 6, w - 12, h - 12, 18);
      g.stroke();
      g.fillStyle = "rgba(169,139,255,.12)";
      g.fill();
      g.fillStyle = c;
      g.font = `700 34px ${SANS}`;
      g.fillText(s.t.holo.sertifikat, 34, 62);
      g.fillStyle = "#e9e3ff";
      g.font = `500 26px ${MONO}`;
      g.fillText(id, 34, 112);
      g.font = `500 24px ${SANS}`;
      g.fillStyle = "rgba(233,227,255,.8)";
      g.fillText(s.t.holo.ttd, 34, 160);
      for (let i = 0; i < 9; i++) {
        g.fillStyle = `rgba(169,139,255,${0.25 + 0.06 * i})`;
        g.fillRect(34 + i * 30, 192, 22, 22);
      }
      g.fillStyle = ok ? WARNA.ok : WARNA.gagal;
      g.font = `700 30px ${SANS}`;
      g.fillText(ok ? s.t.holo.sah : s.t.holo.tidakSah, 34, 262);
    });
    m.position.copy(posisi);
    return m;
  }

  function hologramPuf(posisi) {
    const g = new THREE.Group();
    g.position.copy(posisi);
    const kolom = 32, baris = 24, sel = 0.005, jarak = 0.0062;
    const geo = new THREE.PlaneGeometry(sel, sel);
    const mat = new THREE.MeshBasicMaterial({ transparent: true, opacity: 0.95, depthWrite: false, toneMapped: false, blending: THREE.AdditiveBlending });
    const inst = new THREE.InstancedMesh(geo, mat, kolom * baris);
    const m = new THREE.Matrix4();
    let i = 0;
    for (let r = 0; r < baris; r++) for (let c = 0; c < kolom; c++) {
      m.makeTranslation((c - (kolom - 1) / 2) * jarak, (r - (baris - 1) / 2) * jarak, 0);
      inst.setMatrixAt(i++, m);
    }
    const warna = new THREE.Color();
    for (let k = 0; k < kolom * baris; k++) inst.setColorAt(k, warna.set(WARNA.oranye));
    g.add(inst);
    const bingkai = new THREE.LineSegments(new THREE.EdgesGeometry(new THREE.PlaneGeometry(kolom * jarak + 0.012, baris * jarak + 0.012)), new THREE.LineBasicMaterial({ color: new THREE.Color(WARNA.oranye).multiplyScalar(2), transparent: true, toneMapped: false }));
    g.add(bingkai);
    const label = papan(0.22, 0.027, 1040, 128);
    label.position.y = (baris * jarak) / 2 + 0.026;
    label.material.opacity = 1;
    g.add(label);
    g.userData = { inst, bingkai, label, n: kolom * baris };
    g.scale.setScalar(0.001);
    return g;
  }

  function tulisLabelPuf(holo, teks, warna) {
    holo.userData.label.userData.tulis((g, w, h) => {
      g.fillStyle = warna;
      g.font = `700 ${h * 0.5}px ${SANS}`;
      g.textBaseline = "middle";
      g.fillText(teks, 10, h / 2);
    });
  }

  function warnaiPuf(holo, fn) {
    const { inst, n } = holo.userData;
    const c = new THREE.Color();
    for (let k = 0; k < n; k++) inst.setColorAt(k, c.set(fn(k)));
    inst.instanceColor.needsUpdate = true;
  }

  function billboard(obj) {
    return setiapBingkai(() => obj.quaternion.copy(kamera.quaternion));
  }

  async function tampil(obj, durasi = 350) {
    const target = obj.material ? obj.material : null;
    await animasi(durasi, (k) => {
      if (target) target.opacity = k;
    });
  }

  async function paket(dari, ke, warna, teks, durasi = 1000, angkat = 0.06) {
    const g = new THREE.Group();
    const inti = new THREE.Mesh(new THREE.SphereGeometry(0.0055, 16, 12), new THREE.MeshBasicMaterial({ color: new THREE.Color(warna).multiplyScalar(4), toneMapped: false }));
    const pendar = new THREE.Sprite(new THREE.SpriteMaterial({ map: cahaya, color: warna, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
    pendar.scale.setScalar(0.045);
    g.add(inti, pendar);
    let lbl = null;
    if (teks) {
      lbl = labelSprite(teks, warna, 0.018);
      lbl.position.y = 0.028;
      g.add(lbl);
    }
    efek.add(g);
    const jejak = [];
    for (let i = 0; i < 12; i++) {
      const sp = new THREE.Sprite(new THREE.SpriteMaterial({ map: cahaya, color: warna, transparent: true, opacity: 0.5 * (1 - i / 12), blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
      sp.scale.setScalar(0.022 * (1 - i / 14));
      efek.add(sp);
      jejak.push(sp);
    }
    const riwayat = [];
    const tengah = dari.clone().lerp(ke, 0.5);
    tengah.y += angkat;
    const titik = new THREE.Vector3();
    await animasi(durasi, (k) => {
      const a = (1 - k) * (1 - k), b = 2 * (1 - k) * k, c = k * k;
      titik.set(a * dari.x + b * tengah.x + c * ke.x, a * dari.y + b * tengah.y + c * ke.y, a * dari.z + b * tengah.z + c * ke.z);
      g.position.copy(titik);
      riwayat.unshift(titik.clone());
      jejak.forEach((sp, i) => sp.position.copy(riwayat[Math.min(i * 2, riwayat.length - 1)]));
    }, mudah.halus);
    efek.remove(g);
    jejak.forEach((sp) => efek.remove(sp));
    const kilat = new THREE.Sprite(new THREE.SpriteMaterial({ map: cahaya, color: warna, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, toneMapped: false }));
    kilat.position.copy(ke);
    efek.add(kilat);
    await animasi(300, (k) => {
      kilat.scale.setScalar(0.03 + k * 0.08);
      kilat.material.opacity = 1 - k;
    });
    efek.remove(kilat);
  }

  async function gelombang(pusat, warna, kali = 3) {
    const tugas = [];
    for (let i = 0; i < kali; i++) {
      tugas.push((async () => {
        await tunggu(i * 220);
        const r = new THREE.Mesh(new THREE.RingGeometry(0.02, 0.023, 48), new THREE.MeshBasicMaterial({ color: new THREE.Color(warna).multiplyScalar(2.5), transparent: true, side: THREE.DoubleSide, depthWrite: false, toneMapped: false, blending: THREE.AdditiveBlending }));
        r.position.copy(pusat);
        r.quaternion.copy(kamera.quaternion);
        efek.add(r);
        await animasi(800, (k) => {
          r.scale.setScalar(1 + k * 3.2);
          r.material.opacity = 1 - k;
        }, mudah.keluar);
        efek.remove(r);
      })());
    }
    await Promise.all(tugas);
  }

  function atasChip(jarak = 0.12) {
    const c = kasus.titikChip();
    const arah = new THREE.Vector3().subVectors(kamera.position, c).normalize();
    const atas = new THREE.Vector3(0, 1, 0);
    return c.clone().addScaledVector(atas, jarak).addScaledVector(arah, 0.04);
  }

  function atasPembaca(jarak = 0.12) {
    return kasus.titikPembaca().clone().add(new THREE.Vector3(0, jarak, 0));
  }

  function lepasTag() {
    if (tagAktif) {
      tagAktif.parent && tagAktif.parent.remove(tagAktif);
      tagAktif = null;
    }
  }

  function pasangTag(teks, warna) {
    lepasTag();
    tagAktif = labelSprite(teks, warna, 0.026);
    tagAktif.position.set(0, kasus.tagTinggi || 0.05, 0);
    (kasus.tagInduk || kasus.penggerak).add(tagAktif);
  }

  function buatPerekam() {
    const B = bahan();
    const g = new THREE.Group();
    const b = kotak(0.06, 0.022, 0.09, B.hitam, 0.006);
    const ant = new THREE.Mesh(new THREE.CylinderGeometry(0.0018, 0.0018, 0.07, 8), B.logamGelap);
    ant.position.set(0.02, 0.045, -0.035);
    const led = lampuLed(new THREE.SphereGeometry(0.004, 10, 8), WARNA.gagal);
    led.position.set(-0.018, 0.012, 0.03);
    g.add(b, ant, led);
    const tag = labelSprite(s.t.tag.perekam, WARNA.ungu, 0.026);
    tag.position.y = 0.075;
    g.add(tag);
    const henti = setiapBingkai((t) => (led.visible = Math.sin(t / 120) > 0));
    g.userData.henti = henti;
    return g;
  }

  async function pilihKasus(id, langsung = false) {
    if (kasus) {
      scene.remove(kasus.grup);
      kasus.buang && kasus.buang();
    }
    bersihEfek();
    lepasTag();
    if (perekam) {
      perekam.userData.henti();
      perekam = null;
    }
    aktifId = id;
    kasus = PEMBANGUN[id](s);
    const su = SUASANA[id];
    scene.background = new THREE.Color(su.latar);
    scene.fog = new THREE.Fog(...su.kabut);
    hemi.color.set(su.hemi[0]);
    hemi.groundColor.set(su.hemi[1]);
    hemi.intensity = su.hemi[2];
    matahari.color.set(su.matahari[0]);
    matahari.intensity = su.matahari[1];
    matahari.position.set(...su.matahari[2]);
    renderer.toneMappingExposure = su.pajanan;
    kasus.grup.traverse((o) => {
      if (o.material && o.material.isMeshStandardMaterial && !o.material.isMeshPhysicalMaterial) o.material.envMapIntensity = o.material.userData.env ?? 0.45;
    });
    scene.add(kasus.grup);
    kasus.lampu(WARNA.siaga);
    kasus.layar({ status: s.t.layar.siaga, rinci: s.t.layar.siagaRinci, warna: WARNA.siaga });
    if (langsung) aturKamera(kasus.kamera.awal);
    else await terbang(kasus.kamera.awal, 900);
  }

  async function terbitkan() {
    await terbang(kasus.kamera.periksa, 1100);
    kasus.layar({ status: s.t.layar.pabrik, rinci: s.t.layar.pabrikRinci, warna: WARNA.oranye, ikon: "putar" });
    kasus.lampu(WARNA.oranye);
    const holo = hologramPuf(atasChip(0.13));
    efek.add(holo);
    const henti = billboard(holo);
    tulisLabelPuf(holo, s.t.holo.pufDaftar, WARNA.oranye);
    await animasi(450, (k) => holo.scale.setScalar(Math.max(0.001, k)), mudah.keluar);
    const bit = Array.from({ length: holo.userData.n }, () => Math.random() < 0.5);
    const mulai = sekarang();
    const hentiKedip = setiapBingkai((t) => {
      const p = Math.min(1, (t - mulai) / 1500);
      warnaiPuf(holo, (k) => (Math.random() < p ? (bit[k] ? WARNA.oranye : "#3a2410") : Math.random() < 0.5 ? WARNA.oranye : "#3a2410"));
    });
    await tunggu(1600);
    hentiKedip();
    tulisLabelPuf(holo, s.t.holo.kunciDibuat, WARNA.ok);
    warnaiPuf(holo, (k) => (bit[k] ? WARNA.ok : "#0d2a1a"));
    kasus.lampu(WARNA.ok);
    await tunggu(700);
    await animasi(350, (k) => holo.scale.setScalar(Math.max(0.001, 1 - k)));
    henti();
    efek.remove(holo);
    const sert = hologramSertifikat(true, s.idChip(), atasChip(0.12));
    efek.add(sert);
    const hentiS = billboard(sert);
    await tampil(sert, 300);
    await gelombang(kasus.titikChip(), WARNA.ungu, 2);
    await tunggu(500);
    await animasi(300, (k) => (sert.material.opacity = 1 - k));
    hentiS();
    efek.remove(sert);
    kasus.lampu(WARNA.siaga);
    kasus.layar({ status: s.t.layar.siaga, rinci: s.t.layar.siagaRinci, warna: WARNA.siaga });
  }

  async function mulaiPeriksa(mode) {
    bersihEfek();
    lepasTag();
    await kasus.pulihkan();
    if (perekam) {
      perekam.userData.henti();
      kasus.grup.remove(perekam);
      perekam = null;
    }
    kasus.lampu(WARNA.siaga);
    kasus.layar({ status: s.t.layar.siaga, rinci: s.t.layar.siagaRinci, warna: WARNA.siaga });
    if (pose(kasus.penggerak).p.distanceTo(kasus.diam.p) > 0.001) await gerakKe(kasus.penggerak, kasus.diam, 600);
    const terbangKe = terbang(kasus.kamera.periksa, 1000);
    if (mode === "klon") pasangTag(s.t.tag.klon, WARNA.oranye);
    if (mode === "klon_daftar") pasangTag(s.t.tag.tiruan, WARNA.gagal);
    if (mode === "ulang") {
      perekam = buatPerekam();
      perekam.position.copy(kasus.titikPerekam.p);
      perekam.quaternion.copy(kasus.titikPerekam.q);
      perekam.scale.setScalar(0.001);
      kasus.grup.add(perekam);
      animasi(400, (k) => perekam.scale.setScalar(Math.max(0.001, k)), mudah.pegas);
    }
    await terbangKe;
    kasus.layar({ status: s.t.layar.dekatkan, rinci: "", warna: WARNA.siaga });
    await gerakBusur(kasus.penggerak, kasus.baca, 1100, 0.05);
    kasus.layar({ status: s.t.layar.membaca, rinci: "", warna: WARNA.siaga, ikon: "putar" });
  }

  async function langkahSertifikat(ok) {
    const h = hologramSertifikat(ok, s.idChip(), atasPembaca(0.14));
    efek.add(h);
    const henti = billboard(h);
    await tampil(h, 300);
    await tunggu(700);
    await animasi(300, (k) => (h.material.opacity = 1 - k));
    henti();
    efek.remove(h);
  }

  async function langkahNyala(hasil) {
    kasus.layar({ status: s.t.layar.nyala, rinci: "", warna: WARNA.siaga, ikon: "putar" });
    await gelombang(kasus.titikPembaca(), WARNA.toska, 3);
    const holo = hologramPuf(atasChip(0.13));
    efek.add(holo);
    const henti = billboard(holo);
    await animasi(400, (k) => holo.scale.setScalar(Math.max(0.001, k)), mudah.keluar);
    const sendiri = hasil.mode === "klon_daftar";
    const bit = Array.from({ length: holo.userData.n }, () => Math.random() < 0.5);
    if (hasil.mode === "klon" && !hasil.nyalaOk) {
      for (let p = 1; p <= 5; p++) {
        tulisLabelPuf(holo, `${s.t.holo.percobaan} ${p}/5`, WARNA.oranye);
        const mulai = sekarang();
        const hk = setiapBingkai(() => warnaiPuf(holo, () => (Math.random() < 0.5 ? WARNA.oranye : "#3a2410")));
        await tunggu(260);
        hk();
        warnaiPuf(holo, () => (Math.random() < 0.5 ? WARNA.gagal : "#2a0b0b"));
        await tunggu(140);
      }
      tulisLabelPuf(holo, s.t.holo.kunciGagal, WARNA.gagal);
      holo.userData.bingkai.material.color.set(WARNA.gagal).multiplyScalar(2);
      kasus.layar({ status: s.t.layar.pufGagal, rinci: s.t.layar.pufGagalRinci, warna: WARNA.gagal, ikon: "gagal" });
      kasus.lampu(WARNA.gagal);
      await tunggu(900);
    } else {
      tulisLabelPuf(holo, sendiri ? s.t.holo.pufSendiri : s.t.holo.pufBaca, sendiri ? WARNA.gagal : WARNA.oranye);
      const mulai = sekarang();
      const hk = setiapBingkai((t) => {
        const p = Math.min(1, (t - mulai) / 900);
        warnaiPuf(holo, (k) => (Math.random() < p ? (bit[k] ? WARNA.oranye : "#3a2410") : Math.random() < 0.5 ? WARNA.oranye : "#3a2410"));
      });
      await tunggu(950);
      hk();
      if (sendiri) {
        tulisLabelPuf(holo, s.t.holo.kunciLain, WARNA.gagal);
        warnaiPuf(holo, (k) => (bit[k] ? "#ff7a59" : "#2a0f0b"));
      } else {
        tulisLabelPuf(holo, s.t.holo.kunciPulih, WARNA.ok);
        warnaiPuf(holo, (k) => (bit[k] ? WARNA.ok : "#0d2a1a"));
        holo.userData.bingkai.material.color.set(WARNA.ok).multiplyScalar(2);
      }
      await tunggu(650);
    }
    await animasi(300, (k) => holo.scale.setScalar(Math.max(0.001, 1 - k)));
    henti();
    efek.remove(holo);
  }

  async function langkahTantangan(byte) {
    kasus.layar({ status: s.t.layar.tantangan, rinci: `ML-KEM-768 · ${byte} B`, warna: WARNA.siaga, ikon: "putar" });
    await paket(kasus.titikPembaca(), kasus.titikChip(), WARNA.toska, `${byte} B`, 1000);
  }

  async function langkahBukti(ulang) {
    const chip = kasus.titikChip(), pembaca = kasus.titikPembaca();
    if (ulang && perekam) {
      const p = perekam.localToWorld(new THREE.Vector3(0, 0.02, 0));
      await paket(chip, p, WARNA.ok, "32 B", 650);
      await tunggu(150);
      await paket(p, pembaca, WARNA.gagal, s.t.tag.buktiLama, 850);
    } else {
      await paket(chip, pembaca, WARNA.ok, "32 B", 1000);
    }
  }

  async function langkahCocok(asli) {
    kasus.layar({ status: s.t.layar.cocok, rinci: "SHA3-256", warna: WARNA.siaga, ikon: "putar" });
    await tunggu(500);
  }

  async function putusan(asli) {
    const w = asli ? WARNA.ok : WARNA.gagal;
    kasus.lampu(w);
    kasus.layar({ status: asli ? s.t.layar.asli[aktifId] : s.t.layar.palsu[aktifId], rinci: asli ? s.t.layar.asliRinci : s.t.layar.palsuRinci, warna: w, ikon: asli ? "ok" : "gagal" });
    const tugas = [kasus.reaksi(asli), gelombang(kasus.titikPembaca(), w, 2)];
    if (!asli) {
      tugas.push((async () => {
        for (let i = 0; i < 3; i++) {
          kasus.lampu("#3a0b0b");
          await tunggu(160);
          kasus.lampu(w);
          await tunggu(220);
        }
      })());
    }
    if (asli && kasus.kamera.dekat && (aktifId === "paspor" || aktifId === "akses")) tugas.push(terbang(kasus.kamera.awal, 1400));
    await Promise.all(tugas);
  }

  async function kembali() {
    lepasTag();
    await Promise.all([gerakBusur(kasus.penggerak, kasus.diam, 800, 0.04), kasus.pulihkan()]);
    if (perekam) {
      perekam.userData.henti();
      kasus.grup.remove(perekam);
      perekam = null;
    }
    kasus.lampu(WARNA.siaga);
    kasus.layar({ status: s.t.layar.siaga, rinci: s.t.layar.siagaRinci, warna: WARNA.siaga });
  }

  function segarkanLayar() {
    if (kasus) kasus.layar({ status: s.t.layar.siaga, rinci: s.t.layar.siagaRinci, warna: WARNA.siaga });
  }

  return { pilihKasus, terbitkan, mulaiPeriksa, langkahSertifikat, langkahNyala, langkahTantangan, langkahBukti, langkahCocok, putusan, kembali, terbang, segarkanLayar, kamera: () => kasus.kamera };
}
