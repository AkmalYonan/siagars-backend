import { expect, test } from '@playwright/test';
import { API, json } from './helpers';

test('asisten mengurai pencarian NICU dan BPJS', async ({ request }) => {
  const response = await request.post(`${API}/assistant/chat`, {
    data: { message: 'Cari RS di Kota Bekasi yang punya NICU dan menerima BPJS' },
  });
  const { data, meta } = await json(response);
  expect(response.headers()['cache-control']).toBe('no-store');
  expect(meta.model).toBe('rule-nlp-v1');
  expect(data.is_emergency).toBe(false);
  expect(data.parsed_query).toMatchObject({
    services: ['nicu'],
    city_code: '3275',
    province_code: '32',
    bpjs: 'wajib',
    dataset: 'real',
  });
  for (const match of data.recommendations) {
    expect(match.hospital.bpjs).toBe(true);
  }
  for (const alternative of data.alternatives) {
    expect(['bpjs_not_accepted', 'bpjs_unknown']).toContain(alternative.reason_code);
    expect(alternative.note).toBeTruthy();
  }
});

test('asisten mendahulukan peringatan darurat', async ({ request }) => {
  const { data } = await json(await request.post(`${API}/assistant/chat`, {
    data: { message: 'Cari RS di Kota Bekasi, pasien tidak sadar' },
  }));
  expect(data.is_emergency).toBe(true);
  expect(data.emergency_keywords).toEqual(['tidak sadar']);
  expect(data.reply).toContain('119');
  expect(data.parsed_query).toBeNull();
  expect(data.recommendations).toEqual([]);
  expect(data.alternatives).toEqual([]);
});
