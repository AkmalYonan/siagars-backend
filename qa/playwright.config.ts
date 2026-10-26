import { defineConfig } from '@playwright/test';

// Semua nilai environment-specific datang dari env; tidak ada credential di repo.
//   SIAGARS_BASE_URL      origin aplikasi (reverse proxy same-origin atau Go langsung), default http://localhost:8080
//                         production https://siagars-backend.vercel.app masih API 0.2.1: test cek BPJS (0.3.0) otomatis di-skip
//   SIAGARS_PUBLIC_ORIGIN nilai header Origin untuk mutasi; harus sama dengan PUBLIC_ORIGIN backend (default = base URL)
//   OPERATOR_EMAIL / OPERATOR_PASSWORD   akun operator demo lokal (opsional; test operator di-skip tanpa ini)
//   E2E_ALLOW_MUTATION=1  izinkan test yang mengubah data (status rujukan, riwayat cek BPJS) pada database yang sedang dipakai backend
//   SIAGARS_UI=1          aktifkan test UI setelah frontend tersedia (saat ini semua test UI di-skip)
const baseURL = process.env.SIAGARS_BASE_URL ?? 'http://localhost:8080';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  timeout: 30_000,
  use: {
    baseURL,
    extraHTTPHeaders: { Accept: 'application/json' },
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'api', testMatch: /.*\.spec\.ts/ },
    // TODO UI: tambahkan project browser saat frontend tersedia, mis. viewport 360/768/1440:
    // { name: 'ui-360', use: { ...devices['Pixel 5'], viewport: { width: 360, height: 740 } } },
    // { name: 'ui-768', use: { viewport: { width: 768, height: 1024 } } },
    // { name: 'ui-1440', use: { viewport: { width: 1440, height: 900 } } },
  ],
});
