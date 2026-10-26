import { APIRequestContext, APIResponse, expect } from '@playwright/test';

export const API = '/api/v1';
export const DEMO_CODE = process.env.DEMO_TRACKING_CODE ?? 'SR-DEMO-001';
export const PUBLIC_ORIGIN = process.env.SIAGARS_PUBLIC_ORIGIN ?? process.env.SIAGARS_BASE_URL ?? 'http://localhost:8080';
export const operator = {
  email: process.env.OPERATOR_EMAIL ?? '',
  password: process.env.OPERATOR_PASSWORD ?? '',
};
export const hasOperator = operator.email !== '' && operator.password !== '';
export const allowMutation = process.env.E2E_ALLOW_MUTATION === '1';
export const uiEnabled = process.env.SIAGARS_UI === '1';

export const STATUSES = ['submitted', 'verified', 'searching', 'confirmed', 'completed', 'rejected'] as const;
export type Status = (typeof STATUSES)[number];
// Mesin status PRD bagian 4 (confirmed -> searching memerlukan catatan alasan).
export const NEXT: Record<Status, Status[]> = {
  submitted: ['verified'],
  verified: ['searching', 'rejected'],
  searching: ['confirmed', 'rejected'],
  confirmed: ['completed', 'searching'],
  completed: [],
  rejected: [],
};

export interface Hospital {
  id: string;
  name: string;
  latitude: number | null;
  longitude: number | null;
  data_origin: string;
  freshness: string;
  bpjs_data_origin?: string | null;
  facility_summary?: { data_origin: string | null; [key: string]: unknown };
  services: { slug: string; status: string; data_origin: string; freshness: string; version: number }[];
  beds: { category: string; total: number | null; available: number | null; data_origin: string; freshness: string; version: number }[];
}

export async function json<T = any>(res: APIResponse, status = 200): Promise<T> {
  expect(res.status(), await res.text()).toBe(status);
  return res.json() as Promise<T>;
}

export function lookup(request: APIRequestContext, code = DEMO_CODE) {
  return request.post(`${API}/referrals/lookup`, { data: { tracking_code: code } });
}

export async function login(request: APIRequestContext): Promise<string> {
  const res = await request.post(`${API}/auth/login`, {
    headers: { Origin: PUBLIC_ORIGIN },
    data: { email: operator.email, password: operator.password },
  });
  const body = await json<{ data: { csrf_token: string } }>(res);
  expect(body.data.csrf_token).toHaveLength(64);
  return body.data.csrf_token;
}
