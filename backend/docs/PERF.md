# Verifikasi performa v1.0.0

Gunakan Vercel Preview untuk kandidat yang akan dirilis, dengan Atlas staging dan data yang mewakili beban production. Jalankan dari direktori `backend` setelah migrasi dan seed staging. Set `SIAGARS_STAGING_URL` ke origin preview, `SIAGARS_PROVINCE_CODE` ke provinsi dengan data nyata, dan `VERCEL_AUTOMATION_BYPASS_SECRET` melalui environment bila preview dilindungi.

```sh
k6 run tests/load/public-api.js
```

Skrip menghangatkan enam endpoint sebelum pengukuran, menjalankan 5 VU selama 1 menit, dan mencatat p50/p95 per endpoint. Targetnya seluruh request HTTP 200 dan p95 ≤ 500 ms per endpoint. Catat commit kandidat, wilayah runner dan Atlas, jumlah RS, p50, p95, serta tingkat error. Ulangi dengan kondisi yang sama bila ada perubahan indeks.

Hasil staging v1.0.0 masih menunggu kandidat terpasang dan akses runner k6.
