export type Lang = 'id' | 'en';

const id = {
	pageTitle: 'GEMBOK Pemeriksa',
	appSubtitle: 'Pemeriksa keaslian chip',
	langLabel: 'Bahasa',
	themeLabel: 'Tema',
	themeDark: 'Gelap',
	themeLight: 'Terang',
	connected: 'Terhubung',
	connecting: 'Menghubungkan',
	offline: 'Terputus',
	offlineTitle: 'Pemeriksa tidak terjangkau',
	offlineBody: 'Pastikan gembok-verifier berjalan dan alamatnya benar. Dasbor mencoba lagi setiap beberapa detik.',
	retry: 'Coba lagi',

	heroIdle: 'Siap memeriksa',
	heroIdleBody: 'Pemeriksa mengunci sebuah rahasia dengan kunci publik chip. Hanya chip asli yang bisa membukanya dan mengirim bukti yang cocok.',
	heroNotEnrolled: 'Chip belum didaftarkan',
	heroNotEnrolledBody: 'Pendaftaran membaca kunci publik chip dan menerbitkan sertifikat untuknya. Cukup sekali saat chip dibuat.',
	heroOffline: 'Menunggu pemeriksa',
	heroOfflineBody: 'Dasbor tidak terhubung ke layanan gembok-verifier.',
	heroChecking: 'Memeriksa',
	heroCheckingBody: 'Tantangan ML-KEM dikirim, menunggu bukti dari chip.',
	heroSwitched: '{chip} terpasang',
	heroSwitchedBody: 'Tekan Periksa chip untuk memeriksa chip yang baru dipasang.',
	verdictASLI: 'ASLI',
	verdictPALSU: 'PALSU',
	sealIdle: 'Gembok tertutup, menunggu pemeriksaan',
	sealChecking: 'Gembok sedang diuji',
	sealGenuine: 'Gembok terbuka: chip asli',
	sealFake: 'Gembok tetap tertutup: chip palsu',
	checkButton: 'Periksa chip',
	checkingButton: 'Memeriksa',
	contextLabel: 'Konteks bukti (opsional)',
	contextPlaceholder: 'misalnya gerbang-A',
	contextHelp: 'Teks ini ikut diikat ke bukti, paling banyak 255 byte.',
	contextTooLong: 'Konteks lebih dari 255 byte.',
	metricChipTime: 'Waktu chip',
	metricTotalTime: 'Waktu total',
	metricCycles: 'Siklus clock',
	simulatedNote: 'Siklus dari simulasi (angka tetap), bukan hasil ukur di papan.',
	measuredNote: 'Siklus dibaca dari register CYCLES chip pada 50 MHz.',
	autoPowerUp: 'Chip dinyalakan otomatis dengan data bantu terdaftar.',
	autoPowerUpRetried: 'Chip dinyalakan otomatis dengan data bantu terdaftar. Kunci PUF pulih pada percobaan ke-{n}.',
	powerUpExhausted: 'Percobaan memulihkan kunci PUF: {n}, semuanya ditolak.',
	rateWaited: 'Menunggu pembatas laju chip {n} kali.',
	tagLabel: 'Tag dari chip',
	reasonLabel: 'Alasan',
	notRun: 'tidak dijalankan',
	contextUsed: 'Konteks bukti: "{c}".',
	chipCodeNote: 'Kode dari chip: {code}.',
	reason_BUKTI_COCOK: 'Bukti dari chip cocok dengan kunci publik di sertifikat.',
	reason_BUKTI_TIDAK_COCOK: 'Bukti tidak cocok. Chip ini tidak memegang kunci rahasia pasangan sertifikat.',
	reason_DATA_BANTU_DITOLAK: 'Chip menolak data bantu terdaftar. PUF chip ini bukan PUF chip yang didaftarkan.',
	reason_SERTIFIKAT_TIDAK_SAH: 'Sertifikat tidak sah, jadi chip tidak ditanya sama sekali.',
	reason_CHIP_TANPA_KUNCI: 'Chip tidak memegang kunci PUF.',
	reason_CHIP_MENOLAK: 'Chip menolak perintah.',

	readerTitle: 'Chip di pembaca',
	readerBody: 'Chip tiruan membawa sertifikat dan data bantu chip asli, tetapi PUF-nya berbeda.',
	chipGenuine: 'Chip asli',
	chipClone: 'Chip tiruan',
	chipGenuineHint: 'Chip yang didaftarkan',
	chipCloneHint: 'PUF lain, sertifikat curian',
	selfEnroll: 'Tiruan mendaftarkan PUF-nya sendiri',
	selfEnrollHint: 'Chip tiruan lalu bisa menjawab, tetapi buktinya tetap tidak cocok dengan sertifikat.',
	switching: 'Mengganti chip',
	boardMode: 'Backend papan: chip fisik terpasang langsung. Penggantian chip hanya ada di simulasi.',

	attackTitle: 'Coba baca kunci',
	attackBody: 'Host menjalankan satu bukti, lalu membaca memori chip di alamat tempat benih, kunci bersama, dan kunci dekapsulasi diproses.',
	attackButton: 'Baca memori rahasia',
	attackRunning: 'Membaca memori',
	attackSummaryZero: '{n} byte rahasia terbaca, semuanya nol.',
	attackSummaryLeak: '{n} byte rahasia tidak nol.',
	attackPublic: 'Wilayah publik H dan TAG tetap terbaca ({n} byte bukan nol), jadi jalur baca bekerja.',
	attackProofRan: 'Bukti sebelum membaca: {verdict}.',
	attackNoProof: 'Belum ada bukti sebelum membaca karena chip belum didaftarkan.',
	attackWhy: 'Brankas menghapus slot rahasia sebelum BUSY turun, dan kunci dekapsulasi hanya hidup di memori polinomial internal yang tidak punya alamat di bus.',
	region_D: 'Benih d',
	region_Z: 'Benih z',
	region_M: "Pesan m'",
	region_K: 'Kunci bersama K',
	region_SIGMA: 'Benih derau sigma',
	region_KBAR: 'Kunci penolakan implisit',
	region_DK: 'Kunci dekapsulasi dk_pke',
	region_H: 'Hash kunci publik',
	region_TAG: 'Bukti terakhir',
	secretTag: 'rahasia',
	publicTag: 'publik',
	bytes: '{n} byte',
	showAll: 'Tampilkan semua {n} baris',
	showLess: 'Ringkas',
	moreLines: '{n} baris lagi',

	statusTitle: 'Status chip',
	statusBackend: 'Backend',
	backendSim: 'Simulasi perangkat lunak',
	backendMmio: 'Papan DE10-Nano (mmio)',
	statusParameter: 'Parameter',
	statusPUF: 'Kunci PUF',
	pufReady: 'Siap di brankas',
	pufNotReady: 'Belum siap',
	statusPUFMode: 'Sumber PUF',
	pufMode0: 'Osilator cincin',
	pufMode1: 'Model simulasi',
	pufMode2: 'Kunci pengembangan tetap (tidak aman)',
	pufModeUnknown: 'Mode tidak dikenal ({n})',
	debugBuild: 'debug',
	statusOscillators: 'Osilator PUF',
	oscillatorsValue: '{n} osilator, topeng {m} byte',
	windowLabel: 'jendela',
	cyclesUnit: 'siklus',
	warningTitle: 'Peringatan keamanan PUF',
	warn_PUF_KUNCI_PENGEMBANGAN: 'Bitstream memakai kunci pengembangan tetap (PUF_MODE 2). Kunci ini publik, jadi hasil ASLI tidak membuktikan keaslian chip.',
	warn_PUF_MODEL_SIMULASI: 'Bitstream memakai model PUF simulasi (PUF_MODE 1). Kunci dihitung dari rumus yang diketahui umum, bukan dari fisik chip.',
	warn_PUF_MODE_TIDAK_DIKENAL: 'CAPS melaporkan mode PUF yang tidak dikenal. Anggap kunci chip tidak rahasia.',
	warn_PUF_BANGUNAN_DEBUG: 'Bangunan debug membuka PUF_MEASURE dan hitungan osilator mentah, jadi kunci bisa dibaca dari luar chip.',
	statusEnrolled: 'Pendaftaran',
	notEnrolled: 'Belum terdaftar',
	certInvalid: 'Sertifikat tidak sah',
	statusProofs: 'Bukti sejak reset',
	statusIssuer: 'Tanda tangan penerbit',
	issuerUsage: '{used} dari {max} terpakai',
	statusLimiter: 'Pembatas laju',
	cooling: 'Aktif, {time} lagi',
	limiterIdle: 'Siap',

	setupTitle: 'Pendaftaran dan penyalaan',
	setupBody: 'Daftarkan sekali saat chip dibuat. Nyalakan setiap kali chip mendapat daya.',
	chipIdLabel: 'ID chip (opsional)',
	chipIdPlaceholder: 'dibuat dari kunci publik bila kosong',
	chipIdInvalid: 'ID chip hanya boleh huruf, angka, titik, garis bawah, dan tanda hubung, diawali huruf atau angka.',
	advancedLabel: 'Pengaturan lanjutan',
	threshLabel: 'Ambang PUF (opsional)',
	threshPlaceholder: 'bawaan {n}',
	threshHint: 'Pakai hasil karakterisasi papan. Bila kosong, pemeriksa memakai {n} dari --ambang.',
	threshInvalid: 'Ambang harus bilangan bulat 1 sampai 65535.',
	enrollButton: 'Daftarkan chip',
	reenrollButton: 'Daftarkan ulang',
	reenrollConfirm: 'Pendaftaran ulang memakai satu dari {left} tanda tangan penerbit yang tersisa.',
	confirmYes: 'Ya, daftarkan ulang',
	confirmNo: 'Batal',
	enrolling: 'Mendaftarkan',
	enrolledOk: 'Chip didaftarkan sebagai {id} dengan ambang PUF {t}. Sertifikat ditandatangani dengan LMS.',
	powerButton: 'Nyalakan chip',
	powering: 'Menyalakan',
	powerOk: 'Kunci PUF dipulihkan pada percobaan {n} dari {max} dan cocok dengan data bantu.',
	powerRefused: 'Chip menolak data bantu ({code}). Percobaan: {n} dari {max}.',

	historyTitle: 'Riwayat pemeriksaan',
	historyEmpty: 'Belum ada pemeriksaan. Tekan Periksa chip untuk memulai.',
	noContext: 'tanpa konteks',

	err_TIDAK_TERJANGKAU: 'Pemeriksa tidak terjangkau.',
	err_BELUM_TERDAFTAR: 'Chip belum didaftarkan. Daftarkan chip dulu.',
	err_SUDAH_TERDAFTAR: 'Chip sudah didaftarkan.',
	err_CHIP_MENOLAK: 'Chip menolak perintah ({code}).',
	err_CHIP_TIDAK_MENJAWAB: 'Chip tidak menjawab dalam batas waktu.',
	err_PEMBATAS_LAJU: 'Pembatas laju chip masih aktif. Coba lagi sebentar.',
	err_TANDA_TANGAN_HABIS: 'Semua 1024 tanda tangan penerbit sudah terpakai.',
	err_PENERBIT_TIDAK_ADA: 'Kunci penerbit tidak tersedia.',
	err_BUKAN_SIMULASI: 'Penggantian chip hanya ada di backend simulasi.',
	err_TOPENG_TIDAK_SESUAI: 'Data bantu terdaftar dibuat untuk chip dengan jumlah osilator lain, jadi tidak dikirim. Daftarkan ulang chip ini.',
	err_GALAT_INTERNAL: 'Pemeriksa mengalami galat. Lihat log layanan.',

	footer: 'Tantangan memakai ML-KEM (FIPS 203). Sertifikat ditandatangani dengan LMS (RFC 8554).'
};

export type MessageKey = keyof typeof id;

const en: Record<MessageKey, string> = {
	pageTitle: 'GEMBOK Verifier',
	appSubtitle: 'Chip authenticity checker',
	langLabel: 'Language',
	themeLabel: 'Theme',
	themeDark: 'Dark',
	themeLight: 'Light',
	connected: 'Connected',
	connecting: 'Connecting',
	offline: 'Disconnected',
	offlineTitle: 'Verifier unreachable',
	offlineBody: 'Make sure gembok-verifier is running and the address is right. The dashboard retries every few seconds.',
	retry: 'Try again',

	heroIdle: 'Ready to check',
	heroIdleBody: "The verifier locks a secret with the chip's public key. Only the genuine chip can open it and send back a matching proof.",
	heroNotEnrolled: 'Chip not enrolled',
	heroNotEnrolledBody: "Enrollment reads the chip's public key and issues a certificate for it. It is done once, when the chip is made.",
	heroOffline: 'Waiting for the verifier',
	heroOfflineBody: 'The dashboard is not connected to the gembok-verifier service.',
	heroChecking: 'Checking',
	heroCheckingBody: "ML-KEM challenge sent, waiting for the chip's proof.",
	heroSwitched: '{chip} inserted',
	heroSwitchedBody: 'Press Check chip to check the chip that was just inserted.',
	verdictASLI: 'GENUINE',
	verdictPALSU: 'COUNTERFEIT',
	sealIdle: 'Padlock closed, waiting for a check',
	sealChecking: 'Padlock under test',
	sealGenuine: 'Padlock open: genuine chip',
	sealFake: 'Padlock stays shut: counterfeit chip',
	checkButton: 'Check chip',
	checkingButton: 'Checking',
	contextLabel: 'Proof context (optional)',
	contextPlaceholder: 'for example gate-A',
	contextHelp: 'This text is bound into the proof, up to 255 bytes.',
	contextTooLong: 'Context is longer than 255 bytes.',
	metricChipTime: 'Chip time',
	metricTotalTime: 'Total time',
	metricCycles: 'Clock cycles',
	simulatedNote: 'Cycles come from the simulation (fixed numbers), not a measurement on the board.',
	measuredNote: "Cycles read from the chip's CYCLES register at 50 MHz.",
	autoPowerUp: 'The chip was powered up automatically with the enrolled helper data.',
	autoPowerUpRetried: 'The chip was powered up automatically with the enrolled helper data. Its PUF key came back on attempt {n}.',
	powerUpExhausted: 'Attempts to restore the PUF key: {n}, all rejected.',
	rateWaited: "Waited for the chip's rate limiter {n} times.",
	tagLabel: 'Tag from the chip',
	reasonLabel: 'Reason',
	notRun: 'not run',
	contextUsed: 'Proof context: "{c}".',
	chipCodeNote: 'Code from the chip: {code}.',
	reason_BUKTI_COCOK: "The chip's proof matches the public key in the certificate.",
	reason_BUKTI_TIDAK_COCOK: 'The proof does not match. This chip does not hold the private key behind the certificate.',
	reason_DATA_BANTU_DITOLAK: "The chip rejected the enrolled helper data. Its PUF is not the enrolled chip's PUF.",
	reason_SERTIFIKAT_TIDAK_SAH: 'The certificate is invalid, so the chip was not asked at all.',
	reason_CHIP_TANPA_KUNCI: 'The chip holds no PUF key.',
	reason_CHIP_MENOLAK: 'The chip refused the command.',

	readerTitle: 'Chip in the reader',
	readerBody: "The cloned chip carries the genuine chip's certificate and helper data, but its PUF is different.",
	chipGenuine: 'Genuine chip',
	chipClone: 'Cloned chip',
	chipGenuineHint: 'The enrolled chip',
	chipCloneHint: 'Different PUF, stolen certificate',
	selfEnroll: 'Clone enrolls its own PUF',
	selfEnrollHint: 'The clone can then answer, but its proof still does not match the certificate.',
	switching: 'Swapping chip',
	boardMode: 'Board backend: the physical chip is attached directly. Swapping chips exists only in simulation.',

	attackTitle: 'Try to read the key',
	attackBody: 'The host runs one proof, then reads chip memory at the addresses where the seeds, the shared key and the decapsulation key are processed.',
	attackButton: 'Read secret memory',
	attackRunning: 'Reading memory',
	attackSummaryZero: '{n} secret bytes read, all of them zero.',
	attackSummaryLeak: '{n} secret bytes are not zero.',
	attackPublic: 'The public H and TAG regions are still readable ({n} non-zero bytes), so the read path works.',
	attackProofRan: 'Proof before reading: {verdict}.',
	attackNoProof: 'No proof before reading because the chip is not enrolled.',
	attackWhy: 'The vault clears the secret slots before BUSY drops, and the decapsulation key lives only in internal polynomial memory that has no bus address.',
	region_D: 'Seed d',
	region_Z: 'Seed z',
	region_M: "Message m'",
	region_K: 'Shared key K',
	region_SIGMA: 'Noise seed sigma',
	region_KBAR: 'Implicit rejection key',
	region_DK: 'Decapsulation key dk_pke',
	region_H: 'Public key hash',
	region_TAG: 'Latest proof',
	secretTag: 'secret',
	publicTag: 'public',
	bytes: '{n} bytes',
	showAll: 'Show all {n} lines',
	showLess: 'Collapse',
	moreLines: '{n} more lines',

	statusTitle: 'Chip status',
	statusBackend: 'Backend',
	backendSim: 'Software simulation',
	backendMmio: 'DE10-Nano board (mmio)',
	statusParameter: 'Parameter',
	statusPUF: 'PUF key',
	pufReady: 'Ready in the vault',
	pufNotReady: 'Not ready',
	statusPUFMode: 'PUF source',
	pufMode0: 'Ring oscillators',
	pufMode1: 'Simulation model',
	pufMode2: 'Fixed development key (insecure)',
	pufModeUnknown: 'Unknown mode ({n})',
	debugBuild: 'debug',
	statusOscillators: 'PUF oscillators',
	oscillatorsValue: '{n} oscillators, {m}-byte mask',
	windowLabel: 'window',
	cyclesUnit: 'cycles',
	warningTitle: 'PUF security warning',
	warn_PUF_KUNCI_PENGEMBANGAN: 'The bitstream uses the fixed development key (PUF_MODE 2). The key is public, so a GENUINE result does not prove the chip is genuine.',
	warn_PUF_MODEL_SIMULASI: 'The bitstream uses the simulated PUF model (PUF_MODE 1). The key comes from a publicly known formula, not from the physics of the chip.',
	warn_PUF_MODE_TIDAK_DIKENAL: 'CAPS reports an unknown PUF mode. Treat the chip key as not secret.',
	warn_PUF_BANGUNAN_DEBUG: 'The debug build exposes PUF_MEASURE and raw oscillator counts, so the key can be read from outside the chip.',
	statusEnrolled: 'Enrollment',
	notEnrolled: 'Not enrolled',
	certInvalid: 'Certificate invalid',
	statusProofs: 'Proofs since reset',
	statusIssuer: 'Issuer signatures',
	issuerUsage: '{used} of {max} used',
	statusLimiter: 'Rate limiter',
	cooling: 'Active, {time} left',
	limiterIdle: 'Ready',

	setupTitle: 'Enrollment and power-up',
	setupBody: 'Enroll once when the chip is made. Power up each time the chip gets power.',
	chipIdLabel: 'Chip ID (optional)',
	chipIdPlaceholder: 'derived from the public key when empty',
	chipIdInvalid: 'A chip ID may only use letters, digits, dots, underscores and hyphens, starting with a letter or digit.',
	advancedLabel: 'Advanced settings',
	threshLabel: 'PUF threshold (optional)',
	threshPlaceholder: 'default {n}',
	threshHint: 'Use the result of the board characterization. When empty, the verifier uses {n} from --ambang.',
	threshInvalid: 'The threshold must be a whole number from 1 to 65535.',
	enrollButton: 'Enroll chip',
	reenrollButton: 'Re-enroll',
	reenrollConfirm: 'Re-enrolling uses one of the {left} remaining issuer signatures.',
	confirmYes: 'Yes, re-enroll',
	confirmNo: 'Cancel',
	enrolling: 'Enrolling',
	enrolledOk: 'Chip enrolled as {id} with PUF threshold {t}. The certificate is signed with LMS.',
	powerButton: 'Power up chip',
	powering: 'Powering up',
	powerOk: 'PUF key restored on attempt {n} of {max} and matched the helper data.',
	powerRefused: 'The chip rejected the helper data ({code}). Attempts: {n} of {max}.',

	historyTitle: 'Check history',
	historyEmpty: 'No checks yet. Press Check chip to start.',
	noContext: 'no context',

	err_TIDAK_TERJANGKAU: 'The verifier is unreachable.',
	err_BELUM_TERDAFTAR: 'The chip is not enrolled. Enroll it first.',
	err_SUDAH_TERDAFTAR: 'The chip is already enrolled.',
	err_CHIP_MENOLAK: 'The chip refused the command ({code}).',
	err_CHIP_TIDAK_MENJAWAB: 'The chip did not answer in time.',
	err_PEMBATAS_LAJU: "The chip's rate limiter is still active. Try again in a moment.",
	err_TANDA_TANGAN_HABIS: 'All 1024 issuer signatures are used up.',
	err_PENERBIT_TIDAK_ADA: 'The issuer key is not available.',
	err_BUKAN_SIMULASI: 'Swapping chips exists only in the simulation backend.',
	err_TOPENG_TIDAK_SESUAI: 'The enrolled helper data was made for a chip with a different oscillator count, so it was not sent. Re-enroll this chip.',
	err_GALAT_INTERNAL: 'The verifier hit an error. Check the service log.',

	footer: 'Challenges use ML-KEM (FIPS 203). Certificates are signed with LMS (RFC 8554).'
};

const dictionaries: Record<Lang, Record<MessageKey, string>> = { id, en };

function readStored(key: string): string | null {
	try {
		return globalThis.localStorage?.getItem(key) ?? null;
	} catch {
		return null;
	}
}

function writeStored(key: string, value: string) {
	try {
		globalThis.localStorage?.setItem(key, value);
	} catch {
		return;
	}
}

export type Theme = 'dark' | 'light';

class Preferences {
	lang = $state<Lang>('id');
	theme = $state<Theme>('dark');

	load() {
		const lang = readStored('gembok.bahasa');
		if (lang === 'id' || lang === 'en') this.lang = lang;
		const theme = readStored('gembok.tema');
		if (theme === 'dark' || theme === 'light') this.theme = theme;
	}

	setLang(lang: Lang) {
		this.lang = lang;
		writeStored('gembok.bahasa', lang);
	}

	setTheme(theme: Theme) {
		this.theme = theme;
		writeStored('gembok.tema', theme);
	}

	t = (key: MessageKey, vars?: Record<string, string | number>): string => {
		let text = dictionaries[this.lang][key];
		if (vars) {
			for (const [name, value] of Object.entries(vars)) {
				text = text.replaceAll(`{${name}}`, String(value));
			}
		}
		return text;
	};

	has = (key: string): key is MessageKey => key in id;

	number = (value: number, digits = 0): string =>
		new Intl.NumberFormat(this.lang === 'id' ? 'id-ID' : 'en-US', {
			minimumFractionDigits: digits,
			maximumFractionDigits: digits
		}).format(value);

	duration = (micros: number): string => {
		if (micros < 1000) return `${this.number(micros, micros < 10 ? 2 : 0)} µs`;
		if (micros < 1_000_000) return `${this.number(micros / 1000, 2)} ms`;
		return `${this.number(micros / 1_000_000, 2)} s`;
	};

	clock = (iso: string): string => {
		const date = new Date(iso);
		if (Number.isNaN(date.getTime())) return iso;
		return date.toLocaleTimeString(this.lang === 'id' ? 'id-ID' : 'en-GB', { hour12: false });
	};
}

export const prefs = new Preferences();
