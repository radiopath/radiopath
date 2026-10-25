import createClient from 'openapi-fetch';
import type { components, paths } from './api.gen';

export type Schemas = components['schemas'];
export type Me = Schemas['Me'];
export type Site = Schemas['Site'];
export type SiteInput = Schemas['SiteInput'];
export type Antenna = Schemas['Antenna'];
export type ManualAntennaInput = Schemas['ManualAntennaInput'];
export type Link = Schemas['Link'];
export type LinkInput = Schemas['LinkInput'];
export type LinkDetail = Schemas['LinkDetail'];
export type Share = Schemas['Share'];
export type Coverage = Schemas['Coverage'];
export type CoverageInput = Schemas['CoverageInput'];
export type CoverageDetail = Schemas['CoverageDetail'];
export type CoverageView = Schemas['CoverageView'];
export type LegendBin = Schemas['LegendBin'];
export type Token = Schemas['Token'];
export type HFPredictionInput = Schemas['HFPredictionInput'];
export type HFPoint = Schemas['HFPoint'];
export type HFMap = Schemas['HFMap'];

export const api = createClient<paths>({ baseUrl: '' });

/** ApiError carries the problem detail and, for rejected input, the message of every invalid field. */
export class ApiError extends Error {
	constructor(
		public status: number,
		message: string,
		public errors: string[] = []
	) {
		super(message);
	}
}

type Result<T> = { data?: T; error?: unknown; response: Response };

// Pages anyone may open; a 401 there is an answer to show, not a missing login.
const publicPage = /^\/(login|register|forgot|reset|verify|s)(\/|$)/;

/** Awaits an openapi-fetch call and returns its data; throws ApiError otherwise. A 401 on an app page goes to the login and back. */
export async function call<T>(p: Promise<Result<T>>): Promise<T> {
	const { data, error, response } = await p;
	if (response.ok) return data as T;
	if (response.status === 401 && !publicPage.test(location.pathname)) {
		location.assign('/login?next=' + encodeURIComponent(location.pathname + location.search));
	}
	const problem = (error ?? {}) as { detail?: string; errors?: { message?: string }[] };
	const errors = (problem.errors ?? []).map((e) => e.message ?? '').filter(Boolean);
	throw new ApiError(response.status, problem.detail || response.statusText, errors);
}

/** The config query: the features the login pages offer. */
export const configQuery = () => ({ queryKey: ['config'], queryFn: () => call(api.GET('/api/config')), staleTime: Infinity });

/** The logged-in user, or null for guests. */
export const meQuery = () => ({
	queryKey: ['me'],
	queryFn: async () => {
		const { data, response } = await api.GET('/api/me');
		if (response.status === 401) return null;
		if (!response.ok) throw new ApiError(response.status, response.statusText);
		return data ?? null;
	}
});

/** Returns next only if it is a path on this site, otherwise /, so the login cannot redirect off site. */
export function safeNext(next: string | null): string {
	// Resolve the way the browser will: it drops tabs and newlines and reads \ as /, so "/\t/host" means "//host".
	if (!next?.startsWith('/')) return '/';
	try {
		const u = new URL(next, location.origin);
		return u.origin === location.origin ? u.pathname + u.search + u.hash : '/';
	} catch {
		return '/';
	}
}

/** Parses a decimal number typed by the user; NaN for anything else, so the caller can say which field is wrong. */
export function parseNum(v: string | number): number {
	if (typeof v === 'number') return v;
	return v.trim() === '' ? NaN : Number(v.trim());
}
