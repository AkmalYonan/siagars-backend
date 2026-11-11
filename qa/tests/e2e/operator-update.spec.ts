import { expect, test } from '@playwright/test';
import { API, Hospital, json, login, PUBLIC_ORIGIN } from './helpers';

test('operator mengubah layanan dan hasil terlihat di API publik', async ({ request }) => {
  const hospitalId = '10000000-0000-4000-8000-000000000001';
  const path = `${API}/admin/hospitals/${hospitalId}/services/icu`;
  const detailPath = `${API}/hospitals/${hospitalId}`;
  expect((await request.get(`${API}/admin/hospitals/${hospitalId}`)).status()).toBe(401);

  const csrf = await login(request);
  const headers = { Origin: PUBLIC_ORIGIN, 'X-CSRF-Token': csrf };
  try {
    const before = await json<{ data: Hospital }>(await request.get(detailPath));
    expect(before.data.data_origin).toBe('simulation');
    const service = before.data.services.find((entry) => entry.slug === 'icu');
    if (!service || service.data_origin !== 'simulation') {
      throw new Error('Fixture ICU simulasi wajib tersedia');
    }
    const status = service.status === 'available' ? 'limited' : 'available';
    const data = { status, version: service.version };
    const invalidCsrf = await request.patch(path, {
      headers: { ...headers, 'X-CSRF-Token': 'x'.repeat(64) }, data,
    });
    expect(invalidCsrf.status()).toBe(403);

    const changed = await request.patch(path, { headers, data });
    try {
      expect(changed.status()).toBe(200);
      const stale = await request.patch(path, { headers, data });
      expect(stale.status()).toBe(409);
      const after = await json<{ data: Hospital }>(await request.get(detailPath));
      expect(after.data.services.find((entry) => entry.slug === 'icu')).toMatchObject({
        status, version: service.version + 1, data_origin: 'simulation',
      });
    } finally {
      if (changed.ok()) {
        const restored = await request.patch(path, {
          headers, data: { status: service.status, version: service.version + 1 },
        });
        expect(restored.status(), 'Pemulihan status fixture').toBe(200);
      }
    }
  } finally {
    expect((await request.post(`${API}/auth/logout`, { headers, data: {} })).status()).toBe(200);
  }
  expect((await request.get(`${API}/auth/me`)).status()).toBe(401);
});
