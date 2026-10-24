import { expect, test } from '@playwright/test';
import { allowMutation, API, hasOperator, json, login, lookup, NEXT, PUBLIC_ORIGIN, Status, uiEnabled } from './helpers';

// Alur 3: login operator -> update status -> lookup berubah.
// Test kedua mengubah data (status rujukan SR-DEMO-001 maju satu langkah), jadi hanya jalan dengan E2E_ALLOW_MUTATION=1
// terhadap database uji. Credential hanya dari OPERATOR_EMAIL / OPERATOR_PASSWORD.
test.describe('operator update status (API)', () => {
  test.skip(!hasOperator, 'OPERATOR_EMAIL/OPERATOR_PASSWORD belum di-set (akun dibuat lokal lewat manage; tidak ada akun bawaan)');

  test('tanpa session -> 401 dan CSRF salah -> 403', async ({ request }) => {
    expect((await request.get(`${API}/admin/referrals`)).status()).toBe(401);
    const csrf = await login(request);
    const res = await request.patch(`${API}/admin/referrals/20000000-0000-4000-8000-000000000001/status`, {
      headers: { Origin: PUBLIC_ORIGIN, 'X-CSRF-Token': 'x'.repeat(64) },
      data: { status: 'verified', version: 1 },
    });
    expect(res.status()).toBe(403);
    expect((await res.json()).error.code).toBe('invalid_csrf');
    await request.post(`${API}/auth/logout`, { headers: { Origin: PUBLIC_ORIGIN, 'X-CSRF-Token': csrf }, data: {} });
  });

  test('update status oleh operator terlihat pada lookup publik', async ({ request }) => {
    test.skip(!allowMutation, 'Set E2E_ALLOW_MUTATION=1 (hanya pada database uji) untuk menjalankan test yang mengubah data');

    const before = (await json<{ data: any }>(await lookup(request))).data;
    const csrf = await login(request);
    const me = await json<{ data: { hospital_ids: string[] } }>(await request.get(`${API}/auth/me`));
    expect(me.data.hospital_ids.length).toBeGreaterThan(0);

    // Kode tracking tidak ada di DTO admin; cocokkan rujukan lewat status + updated_at yang sama dengan lookup.
    const admin = await json<{ data: any[] }>(await request.get(`${API}/admin/referrals`));
    const ref = admin.data.find((r) => r.status === before.public_status && r.updated_at === before.updated_at);
    test.skip(!ref, 'rujukan SR-DEMO-001 tidak dimiliki operator ini');
    const options = NEXT[ref.status as Status];
    test.skip(options.length === 0, `status ${ref.status} sudah terminal; reseed database uji`);
    const to = options[0];

    const note = `Uji E2E ${new Date().toISOString()}`;
    const headers = { Origin: PUBLIC_ORIGIN, 'X-CSRF-Token': csrf };
    const patched = await json<{ data: any }>(
      await request.patch(`${API}/admin/referrals/${ref.id}/status`, { headers, data: { status: to, public_note: note, version: ref.version } }),
    );
    expect(patched.data.status).toBe(to);
    expect(patched.data.version).toBe(ref.version + 1);

    // Versi basi -> 409, tidak ada riwayat ganda.
    const stale = await request.patch(`${API}/admin/referrals/${ref.id}/status`, { headers, data: { status: to, public_note: note, version: ref.version } });
    expect(stale.status()).toBe(409);

    const after = (await json<{ data: any }>(await lookup(request))).data;
    expect(after.public_status).toBe(to);
    expect(after.public_note).toBe(note);
    expect(after.history.length).toBe(before.history.length + 1);
    expect(after.history.at(-1)).toMatchObject({ from_status: before.public_status, to_status: to, public_note: note });

    const out = await request.post(`${API}/auth/logout`, { headers, data: {} });
    expect(out.status()).toBe(200);
    expect((await request.get(`${API}/auth/me`)).status()).toBe(401);
  });

  test.describe('UI', () => {
    test.skip(!uiEnabled, 'TODO UI: frontend belum ada. Aktifkan saat halaman login operator dan dashboard tersedia.');

    test('operator login, mengubah status, lalu halaman lacak menampilkan event terbaru', async ({ page }) => {
      // TODO UI: isi dari process.env.OPERATOR_EMAIL / OPERATOR_PASSWORD (jangan hardcode).
      // await page.goto('/operator/login');
      // await page.getByLabel(/email/i).fill(process.env.OPERATOR_EMAIL!);
      // await page.getByLabel(/password|kata sandi/i).fill(process.env.OPERATOR_PASSWORD!);
      // await page.getByRole('button', { name: /masuk/i }).click();
      // ... ubah status, buka /lacak, pastikan event terbaru tampil.
    });
  });
});
