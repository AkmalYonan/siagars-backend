# Verifikasi performa v1.0.0

Gunakan Vercel Preview untuk kandidat yang akan dirilis, dengan Atlas staging dan data yang mewakili beban production. Jalankan dari direktori `backend` setelah migrasi dan seed staging. Set `SIAGARS_STAGING_URL` ke origin preview, `SIAGARS_PROVINCE_CODE` ke provinsi dengan data nyata, dan `VERCEL_AUTOMATION_BYPASS_SECRET` melalui environment bila preview dilindungi.

```sh
k6 run tests/load/public-api.js
```

Skrip menghangatkan enam endpoint sebelum pengukuran, menjalankan 5 VU selama 1 menit, dan mencatat p50/p95 per endpoint. Targetnya seluruh request HTTP 200 dan p95 ≤ 500 ms per endpoint. Catat commit kandidat, wilayah runner dan Atlas, jumlah RS, p50, p95, serta tingkat error. Ulangi dengan kondisi yang sama bila ada perubahan indeks.

## Baseline Preview develop v0.5.0

Pengukuran 11 November 2026 memakai k6 2.3.0 dari Jakarta terhadap 516 RS pada dataset nyata. Semua 390 request HTTP berhasil, termasuk warm-up. Vercel melayani request dari edge `sin1` dengan fungsi di `iad1`, berdasarkan `x-vercel-id`.

| Endpoint | p50 | p95 |
|---|---:|---:|
| `/hospitals` | 1.450 ms | 3.600 ms |
| `/recommendations` | 2.020 ms | 2.530 ms |
| `/insights/summary` | 31,30 ms | 107,24 ms |
| `/insights/coverage` | 31,16 ms | 94,98 ms |
| `/insights/service-gaps` | 30,30 ms | 56,93 ms |
| `/insights/freshness` | 33,16 ms | 130,03 ms |

Kandidat menetapkan region fungsi `sin1` sesuai PRD. Konfirmasi region dari header deployment kandidat dan ulangi beban di staging. Jika pencarian atau rekomendasi masih melewati 500 ms, bandingkan waktu query Atlas dan periksa rencana eksekusi sebelum memilih perubahan indeks. [Konfigurasi region Vercel](https://vercel.com/docs/functions/configuring-functions/region)
