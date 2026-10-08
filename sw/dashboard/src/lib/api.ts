import type {
	AttackResult,
	CheckResult,
	EnrollBody,
	EnrollResult,
	HistoryReply,
	PowerUpResult,
	StatusReport
} from './types';

export class ApiError extends Error {
	readonly status: number;
	readonly code: string;
	readonly chipCode: string;

	constructor(status: number, code: string, message: string, chipCode = '') {
		super(message);
		this.status = status;
		this.code = code;
		this.chipCode = chipCode;
	}
}

interface ErrorEnvelope {
	galat: { kode: string; pesan: string; kode_chip?: string };
}

function isErrorEnvelope(value: unknown): value is ErrorEnvelope {
	if (typeof value !== 'object' || value === null || !('galat' in value)) return false;
	const inner = (value as { galat: unknown }).galat;
	return typeof inner === 'object' && inner !== null && 'kode' in inner && 'pesan' in inner;
}

async function request<T>(method: 'GET' | 'POST', path: string, body?: unknown): Promise<T> {
	let response: Response;
	try {
		response = await fetch(path, {
			method,
			cache: 'no-store',
			headers: method === 'POST' ? { 'Content-Type': 'application/json' } : undefined,
			body: method === 'POST' ? JSON.stringify(body ?? {}) : undefined
		});
	} catch {
		throw new ApiError(0, 'TIDAK_TERJANGKAU', 'Pemeriksa tidak terjangkau.');
	}
	const text = await response.text();
	let data: unknown = null;
	if (text) {
		try {
			data = JSON.parse(text);
		} catch {
			data = null;
		}
	}
	if (!response.ok) {
		if (isErrorEnvelope(data)) {
			throw new ApiError(response.status, data.galat.kode, data.galat.pesan, data.galat.kode_chip ?? '');
		}
		throw new ApiError(response.status, 'GALAT_HTTP', `HTTP ${response.status}`);
	}
	if (data === null) {
		throw new ApiError(response.status, 'JAWABAN_KOSONG', 'Jawaban pemeriksa kosong.');
	}
	return data as T;
}

export const api = {
	status: () => request<StatusReport>('GET', '/api/status'),
	history: (limit: number) => request<HistoryReply>('GET', `/api/riwayat?batas=${limit}`),
	enroll: (body: EnrollBody) => request<EnrollResult>('POST', '/api/daftar', body),
	powerUp: () => request<PowerUpResult>('POST', '/api/nyalakan', {}),
	check: (konteks: string) => request<CheckResult>('POST', '/api/periksa', { konteks }),
	readKey: () => request<AttackResult>('POST', '/api/serang/baca-kunci', {}),
	switchChip: (chip: string, selfEnroll: boolean) =>
		request<StatusReport>('POST', '/api/simulasi/chip', { chip, daftar_sendiri: selfEnroll })
};
