import { expect, test } from '@playwright/test';
import { API, Hospital, json, uiEnabled } from './helpers';

// Alur 1: cari -> detail. Berjalan terhadap backend yang sudah di-seed (RS simulasi, dataset=simulation).
test.describe('cari -> detail (API)', () => {
  test('filter services memakai AND, lalu detail konsisten dengan daftar', async ({ request }) => {
    const list = await json<{ data: Hospital[]; meta: Record<string, unknown> }>(
      await request.get(`${API}/hospitals?dataset=simulation&services=nicu,icu&limit=5`),
    );
    expect(list.meta).toMatchObject({ page: 1, limit: 5, data_scope: 'pilot', dataset: 'simulation' });
    expect(typeof list.meta.as_of).toBe('string');
    expect(Array.isArray(list.data)).toBe(true);
    test.skip(list.data.length === 0, 'seed simulasi belum dimuat: jalankan seed pada database uji');

    for (const h of list.data) {
      const open = (slug: string) => h.services.some((s) => s.slug === slug && ['available', 'limited'].includes(s.status));
      expect(open('nicu') && open('icu'), `${h.name} harus punya nicu DAN icu`).toBe(true);
      expect(h.data_origin).toBe('simulation');
    }

    const target = list.data[0];
    const detail = await json<{ data: Hospital; meta: { as_of: string } }>(await request.get(`${API}/hospitals/${target.id}`));
    expect(detail.data.id).toBe(target.id);
    expect(detail.data.name).toBe(target.name);
    for (const item of [...detail.data.services, ...detail.data.beds]) {
      // provenance per item (PRD 3): origin + freshness selalu ada, version >= 1
      expect(item.data_origin).toBeTruthy();
      expect(['fresh', 'aging', 'stale', 'unknown']).toContain(item.freshness);
      expect(item.version).toBeGreaterThanOrEqual(1);
    }
  });

  test('bed null tetap null (bukan 0) dan RS tak dikenal -> 404', async ({ request }) => {
    const list = await json<{ data: Hospital[] }>(await request.get(`${API}/hospitals?dataset=all&limit=50`));
    for (const b of list.data.flatMap((h) => h.beds)) {
      for (const v of [b.total, b.available]) expect(v === null || (Number.isInteger(v) && v >= 0)).toBe(true);
    }
    const missing = await request.get(`${API}/hospitals/10000000-0000-4000-8000-0000000000ff`);
    expect(missing.status()).toBe(404);
    expect((await missing.json()).error.code).toBe('not_found');
  });

  test('parameter tak dikenal -> 422', async ({ request }) => {
    expect((await request.get(`${API}/hospitals?bogus=1`)).status()).toBe(422);
  });

  test.describe('UI', () => {
    test.skip(!uiEnabled, 'TODO UI: frontend belum ada. Hapus skip ini saat halaman pencarian tersedia (SIAGARS_UI=1).');

    test('pengguna mencari nama, membuka hasil, melihat detail dengan label sumber dan kesegaran', async ({ page }) => {
      // TODO UI: sesuaikan selector (getByRole) dengan komponen final.
      // await page.goto('/');
      // await page.getByRole('searchbox', { name: /cari/i }).fill('Siaga');
      // await page.getByRole('link', { name: /RS Simulasi Siaga 01/ }).click();
      // await expect(page.getByRole('heading', { name: /RS Simulasi Siaga 01/ })).toBeVisible();
      // await expect(page.getByText(/simulasi/i).first()).toBeVisible();   // label simulation terlihat
      // await expect(page.getByText(/diperbarui|kesegaran/i).first()).toBeVisible();
    });
  });
});
