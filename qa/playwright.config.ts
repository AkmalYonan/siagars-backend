import { defineConfig } from '@playwright/test';

const baseURL = process.env.SIAGARS_BASE_URL ?? 'http://localhost:8080';
const target = new URL(baseURL);
if (process.env.E2E_ALLOW_MUTATION !== '1' ||
    !process.env.OPERATOR_EMAIL || !process.env.OPERATOR_PASSWORD) {
  throw new Error('Suite API memerlukan E2E_ALLOW_MUTATION=1 dan kredensial operator untuk fixture uji.');
}
if (process.env.CI && (!process.env.SIAGARS_BASE_URL || !process.env.SIAGARS_PUBLIC_ORIGIN)) {
  throw new Error('CI memerlukan SIAGARS_BASE_URL staging dan SIAGARS_PUBLIC_ORIGIN.');
}
if (target.hostname === 'siagars-backend.vercel.app') {
  throw new Error('Suite API memutasi fixture. Gunakan database uji atau staging.');
}

const bypass = process.env.VERCEL_AUTOMATION_BYPASS_SECRET;
export default defineConfig({
  testDir: './tests/e2e',
  workers: 1,
  forbidOnly: true,
  retries: 0,
  reporter: 'list',
  timeout: 30_000,
  use: {
    baseURL,
    extraHTTPHeaders: {
      Accept: 'application/json',
      ...(bypass ? { 'x-vercel-protection-bypass': bypass } : {}),
    },
  },
});
