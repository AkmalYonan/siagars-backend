import http from 'k6/http';
import { sleep } from 'k6';

const baseURL = (__ENV.SIAGARS_STAGING_URL || '').replace(/\/$/, '');
if (!/^https:\/\/[^/]+$/.test(baseURL) || baseURL === 'https://siagars-backend.vercel.app') {
  throw new Error('Set SIAGARS_STAGING_URL ke origin staging HTTPS.');
}
const province = __ENV.SIAGARS_PROVINCE_CODE || '31';
const endpoints = [
  ['hospitals', '/hospitals?dataset=real&limit=20'],
  ['recommendations', `/recommendations?dataset=real&services=icu&province_code=${province}`],
  ['summary', '/insights/summary'],
  ['coverage', '/insights/coverage'],
  ['service-gaps', '/insights/service-gaps?service=icu'],
  ['freshness', '/insights/freshness'],
];
http.setResponseCallback(http.expectedStatuses(200));
const headers = { Accept: 'application/json' };
if (__ENV.VERCEL_AUTOMATION_BYPASS_SECRET) {
  headers['x-vercel-protection-bypass'] = __ENV.VERCEL_AUTOMATION_BYPASS_SECRET;
}
const thresholds = { http_req_failed: ['rate==0'] };
for (const [name] of endpoints) {
  thresholds[`http_req_duration{name:${name}}`] = ['p(95)<=500'];
}

export const options = {
  vus: 5,
  duration: '1m',
  summaryTrendStats: ['count', 'p(50)', 'p(95)'],
  thresholds,
};

function request(path, name) {
  return http.get(`${baseURL}/api/v1${path}`, { headers, redirects: 0, tags: { name } });
}

export function setup() {
  for (const [name, path] of endpoints) {
    if (request(path, `warmup-${name}`).status !== 200) {
      throw new Error(`Warm-up gagal untuk ${name}`);
    }
  }
}

export default function () {
  for (const [name, path] of endpoints) {
    request(path, name);
  }
  sleep(1);
}
