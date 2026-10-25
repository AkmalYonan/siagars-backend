# Serah Terima Production Backend SiagaRS v0.2.1 untuk Frontend

Untuk: M. Tauriq Arya dan tim frontend. Dari: Akmal Bariq Yonanda (backend). Tanggal: 25 Oktober 2026. Versi API production: **0.2.1**.

Production kini sama dengan staging 0.2.x, jadi FE bisa menembak production **tanpa secret bypass dan tanpa proxy khusus**. Rujukan lain: [`HANDOVER_v0.2.md`](HANDOVER_v0.2.md) (aturan tampilan, bentuk data, daftar periksa), [`CHANGELOG.md`](CHANGELOG.md), [`API_FRONTEND.md`](API_FRONTEND.md), [`openapi.yaml`](openapi.yaml). Bila dokumen berbeda dengan OpenAPI, ikuti OpenAPI.

## 1. Alamat

| | |
|---|---|
| Base URL | `https://siagars-backend.vercel.app` |
| API | `https://siagars-backend.vercel.app/api/v1` |
| Dokumentasi interaktif (Scalar) | `https://siagars-backend.vercel.app/docs` |
| Kontrak JSON | `https://siagars-backend.vercel.app/docs/openapi.json` |

Staging (`develop`) tetap ada untuk mencoba perubahan berikutnya; production hanya berubah saat `main` dirilis.

## 2. Menghubungkan FE (Next.js)

Pakai rewrite biasa. Browser hanya bicara ke origin FE, jadi tidak perlu CORS (backend memang tidak membuka CORS).

```ts
// next.config.ts
async rewrites() {
  return [{ source: '/api/v1/:path*', destination: `${process.env.API_BASE_URL}/api/v1/:path*` }];
}
```

```text
# .env.local (atau env Vercel milik FE)
API_BASE_URL=https://siagars-backend.vercel.app
```

Cek cepat: `GET /api/v1/health` -> `{"data":{"status":"ok"}}`, lalu `GET /api/v1/hospitals?limit=3`.

Route handler proxy dengan header bypass di `API_FRONTEND.md` bagian 2a **hanya untuk staging**; jangan dipakai untuk production.

## 3. Data di production

- 516 RS nyata Jabodetabek dari SATUSEHAT sandbox (`dataset=real`, default), 30 RS simulasi (`dataset=simulation`), keduanya (`dataset=all`, total 546).
- SATUSEHAT sandbox tidak menyediakan ICU/NICU/PICU/bed/rawat inap/BPJS. Karena itu RS nyata diberi **data contoh (dummy)**: `facility_summary.data_origin == "simulation"`, `bpjs_data_origin == "simulation"`, dan tiap item `services`/`beds` berlabel `data_origin: "simulation"`. **Wajib tampilkan label "Simulasi"**; jangan sajikan sebagai fakta RS.
- Sebagian RS sengaja tanpa bed (`null`) agar jalur "belum diketahui" teruji. `null`/`unknown` = belum diketahui, bukan "tidak ada". Bed `0` adalah angka nyata.
- `freshness` dihitung dari `source_updated_at`. Data contoh ditempel pada 25 Okt 2026, jadi akan tampil `stale` setelah 24 jam. Itu perilaku benar, bukan bug; backend dapat menyegarkannya bila diminta.
- Compare dan detail menerima UUID **atau** kode SATUSEHAT (`external_id`), campuran boleh: `/hospitals/compare?ids=1004673817,<uuid>`.
- Tracking demo publik sintetis: `POST /referrals/lookup` dengan `{"tracking_code":"SR-DEMO-001"}` (field-nya `tracking_code`).

## 4. Endpoint publik yang siap dipakai

`GET /health`, `/ready`, `/services`, `/regions`, `/hospitals` (filter, `services` bersifat AND, `limit` maks 50, parameter tak dikenal 422), `/hospitals/suggest?q=`, `/hospitals/{id|kode}`, `/hospitals/compare?ids=`, `POST /referrals/lookup`. Detailnya ada di `/docs`.

## 5. Login operator di production (perlu diketahui)

- Cookie sesi `siagars_session` (HttpOnly, Secure, SameSite=Lax) dan header `X-CSRF-Token` untuk setiap mutasi berlaku seperti di `HANDOVER_v0.2.md` bagian 3.
- Backend mencocokkan header `Origin` browser dengan `PUBLIC_ORIGIN`. Di production nilainya saat ini `https://siagars-backend.vercel.app`, jadi login dari FE di origin lain (mis. `http://localhost:3000` atau domain FE) akan ditolak 403 sampai backend mengubah `PUBLIC_ORIGIN` ke origin FE. **Beri tahu Akmal origin FE final** (satu nilai, persis, tanpa slash akhir).
- Akun operator production dibagikan oleh Akmal lewat kanal privat, tidak ada di git atau dokumen ini.
- Endpoint publik (bagian 4) tidak butuh login atau Origin khusus.

## 6. Daftar periksa FE

- [ ] `API_BASE_URL` menunjuk production, tanpa header bypass di kode apa pun.
- [ ] Label "Simulasi" tampil bila `facility_summary.data_origin` atau `bpjs_data_origin` bernilai `simulation`.
- [ ] `unknown`/`null` tampil sebagai "Belum diketahui", bukan merah/"tidak tersedia".
- [ ] Compare memakai `external_id ?? id`.
- [ ] Filter wilayah memakai `code` dari `GET /regions` apa adanya.
- [ ] Origin FE diberikan ke Akmal sebelum menguji login operator.

## 7. Hasil uji production (25 Okt 2026)

Dijalankan terhadap `https://siagars-backend.vercel.app`: 32 pemeriksaan lulus, mencakup health/ready, 6 layanan, regions, 516 RS nyata dengan label simulasi, detail via UUID dan kode, compare (campuran, duplikat 422, hilang 404), validasi parameter, filter AND, jarak, dataset, suggest, lookup (demo 200, acak 404), admin tanpa sesi 401, login Origin salah 403, tanpa CORS, header keamanan, `/docs` dan OpenAPI 0.2.1 dengan 19 path. Belum diuji di production: login operator berhasil dan mutasi admin (akun belum dibuat).
