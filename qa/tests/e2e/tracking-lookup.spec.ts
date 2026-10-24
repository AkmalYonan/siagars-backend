import { expect, test } from '@playwright/test';
import { DEMO_CODE, json, lookup, uiEnabled } from './helpers';

// Alur 2: lookup SR-DEMO-001. Jaga jumlah request: lookup dibatasi 10/menit/IP, jadi 429 tidak diuji di sini.
test.describe('lookup tracking (API)', () => {
  test('SR-DEMO-001 mengembalikan DTO publik tanpa data internal', async ({ request }) => {
    const res = await lookup(request);
    const body = await json<{ data: Record<string, any> }>(res);
    expect(res.headers()['cache-control']).toBe('no-store');
    expect(Object.keys(body.data).sort()).toEqual(['destination', 'history', 'next_step', 'public_note', 'public_status', 'updated_at']);
    expect(['submitted', 'verified', 'searching', 'confirmed', 'completed', 'rejected']).toContain(body.data.public_status);
    expect(body.data.history.length).toBeGreaterThan(0);
    for (const h of body.data.history) expect(Object.keys(h).sort()).toEqual(['created_at', 'from_status', 'public_note', 'to_status']);
    const raw = JSON.stringify(body).toLowerCase();
    for (const leak of ['owner_hospital_id', 'token', 'hash', 'actor', 'operator', 'internal', DEMO_CODE]) {
      expect(raw, `bocor: ${leak}`).not.toContain(leak.toLowerCase());
    }
  });

  test('kode tak dikenal -> 404 generik', async ({ request }) => {
    const res = await lookup(request, 'SR-TIDAK-ADA-999');
    expect(res.status()).toBe(404);
    expect((await res.json()).error.code).toBe('not_found');
  });

  test.describe('UI', () => {
    test.skip(!uiEnabled, 'TODO UI: frontend belum ada. Aktifkan saat halaman lacak rujukan tersedia.');

    test('memasukkan SR-DEMO-001 menampilkan status, langkah berikutnya, dan label skenario sintetis', async ({ page }) => {
      // TODO UI:
      // await page.goto('/lacak');
      // await page.getByLabel(/kode/i).fill('SR-DEMO-001');
      // await page.getByRole('button', { name: /lacak|cari/i }).click();
      // await expect(page.getByText(/simulasi|sintetis/i).first()).toBeVisible();
      // await expect(page.getByRole('list', { name: /riwayat/i })).toBeVisible();
    });
  });
});
