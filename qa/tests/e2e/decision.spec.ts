import { expect, test } from '@playwright/test';
import { API, Hospital, json } from './helpers';

test('rekomendasi RS simulasi dapat dibuka sebagai detail', async ({ request }) => {
  const query = 'dataset=simulation&services=icu&province_code=31';
  const result = await json<{ data: { hospital: { id: string }; score: number }[] }>(
    await request.get(`${API}/recommendations?${query}`),
  );
  expect(result.data.length).toBeGreaterThan(0);
  const { hospital, score } = result.data[0];
  expect(score).toBeGreaterThanOrEqual(0);
  expect(score).toBeLessThanOrEqual(100);
  const detail = await json<{ data: Hospital }>(await request.get(`${API}/hospitals/${hospital.id}`));
  expect(detail.data.id).toBe(hospital.id);
});

test('paginasi mengembalikan RS berbeda dan tiga RS dapat dibandingkan', async ({ request }) => {
  const path = `${API}/hospitals?dataset=simulation&limit=3&sort=name`;
  const first = await json<{ data: Hospital[] }>(await request.get(`${path}&page=1`));
  const second = await json<{ data: Hospital[] }>(await request.get(`${path}&page=2`));
  expect(first.data).toHaveLength(3);
  expect(second.data).toHaveLength(3);
  const ids = first.data.map((hospital) => hospital.id);
  expect(second.data.every((hospital) => !ids.includes(hospital.id))).toBe(true);
  const compare = await json<{ data: Hospital[] }>(
    await request.get(`${API}/hospitals/compare?ids=${ids.join(',')}`),
  );
  expect(compare.data.map((hospital) => hospital.id)).toEqual(ids);
});

test('keempat endpoint Insights melayani dataset simulasi', async ({ request }) => {
  const base = `${API}/insights`;
  const summary = await json<{ data: { hospital_count: number; curated_count: number } }>(
    await request.get(`${base}/summary?dataset=simulation`),
  );
  const coverage = await json<{ data: { hospital_count: number; curated_count: number }[] }>(
    await request.get(`${base}/coverage?dataset=simulation`),
  );
  expect(summary.data.hospital_count).toBeGreaterThan(0);
  expect(coverage.data.reduce((total, city) => total + city.curated_count, 0)).toBe(summary.data.curated_count);
  expect(coverage.data.reduce((total, city) => total + city.hospital_count, 0)).toBe(summary.data.hospital_count);
  for (const path of ['service-gaps?dataset=simulation&service=icu', 'freshness?dataset=simulation']) {
    const res = await request.get(`${base}/${path}`);
    const result = await json<{ data: unknown[]; meta: { dataset: string } }>(res);
    expect(res.headers()['cache-control']).toBe('public, max-age=300');
    expect(result.meta.dataset).toBe('simulation');
    expect(result.data.length).toBeGreaterThan(0);
  }
});
