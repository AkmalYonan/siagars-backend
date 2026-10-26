import { expect, test } from '@playwright/test';
import { allowMutation, API, json, uiEnabled } from './helpers';

// Alur 5: cek BPJS simulasi (API 0.3.0). Peserta sintetis dari seed (NIK fiktif berawalan 9999), bukan BPJS Kesehatan.
// Rate limit 10/menit/IP dihitung sebelum validasi, jadi jumlah request dijaga kecil dan 429 tidak diuji di sini.
// Cek yang valid DICATAT ke riwayat operator (tersamar + terenkripsi, TTL 30 hari), sehingga hanya jalan dengan E2E_ALLOW_MUTATION=1.
const BUDI = { nik: '9999010101900001', name: 'Budi Santoso', birth_date: '1990-01-01' };

function check(request: import('@playwright/test').APIRequestContext, data: Record<string, string>) {
  return request.post(`${API}/bpjs/check`, { data });
}

test.describe('cek BPJS simulasi (API)', () => {
  test('validasi 422 tidak menggemakan input, riwayat admin tanpa session -> 401', async ({ request }) => {
    const res = await check(request, { nik: '12345', name: 'Budi Santoso', birth_date: '1990-01-01' });
    test.skip([404, 501].includes(res.status()), 'backend belum 0.3.0 (production masih 0.2.1)');
    expect(res.status()).toBe(422);
    const body = await res.json();
    expect(body.error.code).toBe('validation_error');
    expect(JSON.stringify(body)).not.toContain('12345');
    expect((await request.get(`${API}/admin/bpjs-checks`)).status()).toBe(401);
  });

  test('peserta demo found tersamar, salah satu field -> not_found yang sama', async ({ request }) => {
    test.skip(!allowMutation, 'Set E2E_ALLOW_MUTATION=1 (hanya pada database uji): cek BPJS dicatat ke riwayat operator');

    const res = await check(request, BUDI);
    test.skip([404, 501].includes(res.status()), 'backend belum 0.3.0 (production masih 0.2.1)');
    expect(res.headers()['cache-control']).toBe('no-store');
    const found = (await json<{ data: any }>(res)).data;
    expect(found).toMatchObject({ result: 'found', data_origin: 'simulation' });
    expect(found.participant).toMatchObject({ nik_masked: '9999********0001', name_masked: 'BU** SA*****', status: 'active' });
    const raw = JSON.stringify(found);
    for (const leak of [BUDI.nik, BUDI.name, BUDI.birth_date]) expect(raw, `bocor: ${leak}`).not.toContain(leak);

    const miss = (await json<{ data: any }>(await check(request, { ...BUDI, birth_date: '1990-01-02' }))).data;
    expect(miss).toMatchObject({ result: 'not_found', participant: null, data_origin: 'simulation' });
  });

  test.describe('UI', () => {
    test.skip(!uiEnabled, 'TODO UI: aktifkan saat halaman cek BPJS frontend tersedia.');

    test('pengguna mengisi data demo dan melihat hasil tersamar berlabel simulasi', async ({ page }) => {
      // TODO UI: sesuaikan rute dan selector dengan komponen final.
      // await page.goto('/cek-bpjs');
      // await expect(page.getByText(/simulasi/i).first()).toBeVisible();
      // await expect(page.getByText(/dicatat|30 hari/i).first()).toBeVisible();
      // await page.getByLabel(/NIK/i).fill(BUDI.nik);
      // await page.getByLabel(/nama/i).fill(BUDI.name);
      // await page.getByLabel(/tanggal lahir/i).fill(BUDI.birth_date);
      // await page.getByRole('button', { name: /cek/i }).click();
      // await expect(page.getByText('9999********0001')).toBeVisible();
      // await expect(page).not.toHaveURL(new RegExp(BUDI.nik));
    });
  });
});
