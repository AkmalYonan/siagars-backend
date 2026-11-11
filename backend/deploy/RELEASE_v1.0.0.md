# Gate staging dan rilis backend v1.0.0

## Staging

- Terapkan migrasi ke database staging, lalu jalankan seed demo. Buat operator uji dengan scope RS sintetis `10000000-0000-4000-8000-000000000001`. Suite mengembalikan status ICU fixture dan menyimpan versi serta audit update. Riwayat BPJS yang ditulis memakai peserta sintetis.
- Pada GitHub environment `staging`, isi variabel `SIAGARS_STAGING_URL` dan `SIAGARS_STAGING_PUBLIC_ORIGIN`. Simpan `E2E_OPERATOR_EMAIL`, `E2E_OPERATOR_PASSWORD`, dan bila diperlukan `VERCEL_AUTOMATION_BYPASS_SECRET` sebagai secret. Workflow `staging-api` berjalan saat deployment Preview dari `develop` sukses. Gunakan `STAGING_DEPLOYMENT_ENVIRONMENT` bila nama environment dari integrasi berbeda.
- Workflow `backend-ci` menjalankan integrasi pada replica set MongoDB sementara. Untuk pengujian lokal, isi `TEST_MONGODB_URI` dengan koneksi ke server uji yang mendukung transaksi. Jalankan Playwright dari `qa` dengan `SIAGARS_BASE_URL`, `SIAGARS_PUBLIC_ORIGIN`, `OPERATOR_EMAIL`, `OPERATOR_PASSWORD`, dan `E2E_ALLOW_MUTATION=1`.
- Verifikasi peer Vercel yang terlihat oleh Go dan isi `TRUSTED_PROXIES` hanya dengan IP/CIDR peer tepercaya yang terkonfirmasi. Cocokkan kuota dari klien berbeda dan lintas instance. [Dokumentasi header Vercel](https://vercel.com/docs/headers/request-headers)
- Catat keputusan pemilik untuk akses `/docs`, daftar jaringan Atlas, dan rotasi kredensial Atlas/SATUSEHAT/operator yang disetujui. Catat status dan tanggal rotasi tanpa nilai kredensial.
- Review berkas kurasi dengan PM. Jalankan `go run ./cmd/manage curate <file>` dari `backend`, lalu `--apply` di staging setelah review. Verifikasi audit, provenance, dan hitungan Summary/Coverage setelah cache lima menit berakhir.
- Lampirkan hasil staging API, integrasi MongoDB, [load test](PERF.md), hasil E2E UI dari FE, dan bukti kurasi pada keputusan Go/No-Go PM.

## Production

1. Setelah Go/No-Go, buat snapshot Atlas dan pastikan prosedur pemulihan tersedia. Jalankan migrasi yang tertunda dari kandidat terhadap database production sebelum deployment menerima traffic. Promosikan kandidat ke `main`.
2. Terapkan batch kurasi yang disetujui. Verifikasi `/api/v1/ready`, versi di `/docs/openapi.json` bila tersedia, pencarian/detail, rekomendasi, insights, lookup demo, serta login dan baca scope operator. Verifikasi `/api/v1/insights/summary?dataset=real` menunjukkan `curated_count` minimal 30 setelah cache berakhir.
3. Jika smoke gagal, tahan tag, pulihkan deployment sebelumnya, dan evaluasi perubahan data dari audit serta snapshot sebelum melakukan pemulihan data.
4. Setelah smoke lulus, buat tag `v1.0.0` pada commit production dan koordinasikan tag dengan FE.
