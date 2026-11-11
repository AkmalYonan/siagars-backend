import { expect, test } from '@playwright/test';
import { API, Hospital, json } from './helpers';

// API daftar tetap bisa dipakai tanpa koordinat atau tile peta.
test.describe('peta gagal -> daftar tetap (API)', () => {
  test('daftar tetap lengkap tanpa lat/lng dan koordinat tak dikenal berpasangan null', async ({ request }) => {
    const list = await json<{ data: Hospital[]; meta: { total: number } }>(await request.get(`${API}/hospitals?dataset=all&limit=50`));
    expect(list.data.length).toBeGreaterThan(0);
    expect(list.meta.total).toBeGreaterThanOrEqual(list.data.length);
    for (const h of list.data) {
      expect(h.latitude === null, `${h.name}: lat/lng harus berpasangan`).toBe(h.longitude === null);
      expect(h.name).toBeTruthy();
    }
  });

  test('sort=distance tanpa lat/lng ditolak 422 (frontend fallback ke sort=name saat geolokasi/peta gagal)', async ({ request }) => {
    expect((await request.get(`${API}/hospitals?sort=distance`)).status()).toBe(422);
    expect((await request.get(`${API}/hospitals?sort=name`)).status()).toBe(200);
    expect((await request.get(`${API}/hospitals?lat=-6.2`)).status()).toBe(422);
  });
});
