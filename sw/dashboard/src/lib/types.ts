export type Verdict = 'ASLI' | 'PALSU';

export type Reason =
	| 'BUKTI_COCOK'
	| 'BUKTI_TIDAK_COCOK'
	| 'SERTIFIKAT_TIDAK_SAH'
	| 'DATA_BANTU_DITOLAK'
	| 'CHIP_TANPA_KUNCI'
	| 'CHIP_MENOLAK';

export interface CheckResult {
	id: number;
	waktu: string;
	hasil: Verdict;
	alasan: Reason;
	keterangan: string;
	id_chip: string;
	parameter: string;
	konteks: string;
	siklus_chip: number;
	waktu_chip_us: number;
	waktu_total_us: number;
	siklus_simulasi: boolean;
	nyala_otomatis: boolean;
	percobaan_nyala: number;
	tunggu_pembatas: number;
	kode_chip?: string;
	tag_chip?: string;
	backend: string;
	chip_simulasi?: string;
}

export interface ChipReport {
	id: string;
	versi: string;
	mode_puf: number;
	nama_mode_puf: string;
	bangunan_debug: boolean;
	jumlah_suara: number;
	jendela_log2: number;
	jumlah_osilator: number;
	panjang_topeng: number;
	batas_waktu_puf_ms: number;
	sudah_boot: boolean;
	sibuk: boolean;
	puf_siap: boolean;
	pendinginan: boolean;
	sisa_pendinginan_siklus: number;
	jumlah_bukti: number;
	galat_terakhir: string;
	siklus_terakhir: number;
	ambang_puf: number;
}

export interface CertificateSummary {
	id_chip: string;
	parameter: string;
	diterbitkan: string;
	sidik_ek: string;
	indeks_tanda_tangan: number;
	sah: boolean;
	masalah?: string;
}

export interface IssuerSummary {
	tersedia: boolean;
	algoritma: string;
	id: string;
	tanda_tangan_terpakai: number;
	tanda_tangan_maksimum: number;
}

export interface SimulationReport {
	chip: string;
	daftar_sendiri: boolean;
	pilihan: string[];
}

export type WarningCode =
	| 'PUF_KUNCI_PENGEMBANGAN'
	| 'PUF_MODEL_SIMULASI'
	| 'PUF_MODE_TIDAK_DIKENAL'
	| 'PUF_BANGUNAN_DEBUG';

export interface Warning {
	kode: WarningCode;
	pesan: string;
}

export interface StatusReport {
	backend: 'sim' | 'mmio';
	siklus_simulasi: boolean;
	clock_hz: number;
	parameter_bawaan: { k: number; nama: string };
	ambang_bawaan: number;
	coba_nyala: number;
	peringatan: Warning[];
	chip: ChipReport;
	terdaftar: boolean;
	sertifikat: CertificateSummary | null;
	penerbit: IssuerSummary;
	simulasi: SimulationReport | null;
	waktu: string;
}

export interface HelperView {
	ambang: number;
	panjang_topeng: number;
	topeng: string;
	cek: string;
	jumlah_pasangan: number;
}

export interface Certificate {
	versi: number;
	id_chip: string;
	parameter: string;
	ek: string;
	penerbit: string;
	diterbitkan: string;
	algoritma_tanda_tangan: string;
	tanda_tangan: string;
}

export interface EnrollResult {
	sertifikat: Certificate;
	data_bantu: HelperView;
	sidik_ek: string;
	siklus_puf: number;
	siklus_daftar: number;
	waktu_daftar_chip_us: number;
	siklus_simulasi: boolean;
	tanda_tangan_terpakai: number;
	tanda_tangan_maksimum: number;
	waktu_total_us: number;
}

export interface PowerUpResult {
	berhasil: boolean;
	puf_siap: boolean;
	kode_chip: string;
	keterangan: string;
	percobaan: number;
	percobaan_maksimum: number;
	siklus: number;
	waktu_chip_us: number;
	siklus_simulasi: boolean;
}

export interface MemoryRegion {
	nama: string;
	alamat: string;
	panjang: number;
	rahasia: boolean;
	semua_nol: boolean;
	byte_bukan_nol: number;
	isi_hex: string;
}

export interface AttackResult {
	waktu: string;
	bukti_dijalankan: boolean;
	bukti: CheckResult | null;
	catatan?: string;
	wilayah: MemoryRegion[];
	jumlah_byte_rahasia: number;
	byte_rahasia_bukan_nol: number;
	rahasia_semua_nol: boolean;
	jumlah_byte_publik: number;
	byte_publik_bukan_nol: number;
	penjelasan: string;
}

export interface HistoryReply {
	riwayat: CheckResult[];
	jumlah: number;
}

export interface EnrollBody {
	id_chip?: string;
	k?: number;
	ambang?: number;
	ulang?: boolean;
}

export type SealMode = 'idle' | 'checking' | 'genuine' | 'fake' | 'off';

export type Connection = 'connecting' | 'online' | 'offline';
