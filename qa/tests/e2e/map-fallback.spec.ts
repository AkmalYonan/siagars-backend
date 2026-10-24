import { expect, test } from '@playwright/test';
import { API, Hospital, json, uiEnabled } from './helpers';

// Alur 4: peta gagal -> daftar tetap. Peta memakai halaman aktif yang sama dengan daftar (PRD 3), tanpa endpoint bbox,
// jadi daftar tidak boleh bergantung pada tile atau koordinat.
test.describe('peta gagal -> daftar tetap (API)', () => {
  test('daftar tetap lengkap tanpa lat/lng dan koordinat tak dikenal berpasangan null', async ({ request }) => {
    const list = await json<{ data: Hospital[]; meta: { total: number } }>(await request.get(`${API}/hospitals?dataset=all&limit=50`));
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

  test.describe('UI', () => {
    test.skip(!uiEnabled, 'TODO UI: frontend belum ada. Aktifkan saat komponen peta tersedia.');

    test('tile peta diblokir -> pesan galat peta tampil dan daftar RS tetap bisa dipakai', async ({ page }) => {
      // TODO UI: sesuaikan host tile dengan penyedia peta yang dipilih frontend.
      // await page.route(/tile|maps|osm/i, (route) => route.abort());
      // await page.goto('/');
      // await expect(page.getByRole('alert')).toContainText(/peta/i);
      // await expect(page.getByRole('list', { name: /rumah sakit/i }).getByRole('listitem').first()).toBeVisible();
    });
  });
});
