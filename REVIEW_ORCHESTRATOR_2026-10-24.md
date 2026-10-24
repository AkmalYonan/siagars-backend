# Review hasil Orchestrator SiagaRS

Tanggal: 24 Oktober 2026. Reviewer: Codex.

Penilaian: pekerjaan menyediakan fondasi backend untuk integrasi frontend, tetapi masih memerlukan perbaikan sebelum dinyatakan siap deployment. Lima temuan di bawah berprioritas P2 (perbaikan fungsional yang perlu dijadwalkan).

Review dilakukan atas berkas yang tersedia, README, AGENTS, kontrak OpenAPI, dan status Orca. Folder ini bukan repository Git, sehingga atribusi perubahan terhadap baseline/commit sebelumnya tidak dapat diverifikasi melalui diff. Kode aplikasi tidak diubah selama review.

## Temuan

### 1. P2 — Kata kunci berawalan `$` ditafsirkan sebagai ekspresi MongoDB

Lokasi: `backend/internal/repository/search.go:63`.

Argumen pencarian kedua pada `$indexOfCP` adalah `strings.ToLower(s.Q)` langsung di dalam `$expr`. MongoDB membaca string berawalan `$` sebagai referensi field. Contohnya, `q=$name` mencari nilai field `name`, bukan teks literal `$name`; `q=$` atau referensi variabel yang tidak valid dapat membuat agregasi gagal. Validasi query saat ini menerima karakter tersebut. Jalur database akibatnya tidak selalu setara dengan `service.Filter`.

Perbaikan: bungkus kata kunci dengan `{$literal: ...}`. Tambahkan uji ekuivalensi database untuk `$`, `$name`, `$$NOW`, dan kata kunci biasa. Temuan berdasarkan pembacaan kode dan semantik resmi MongoDB; request terhadap Atlas tidak direproduksi pada sesi review ini. Rujukan: [MongoDB $literal](https://www.mongodb.com/docs/manual/reference/operator/aggregation/literal/).

### 2. P2 — Compose tidak meneruskan konfigurasi trusted proxy

Lokasi: `backend/compose.yaml:9`; pemakai konfigurasi: `backend/internal/config/config.go` dan `backend/internal/http/server.go`.

Backend sudah membaca `TRUSTED_PROXIES`, tetapi daftar `environment` layanan `api` tidak memuat variabel itu. Menyetelnya di host sesuai petunjuk deployment tidak meneruskannya ke container. Di belakang reverse proxy, backend tetap memakai IP peer/proxy untuk limiter. Pengguna dapat berbagi kuota lookup 10 request/menit dan kuota login berdasarkan IP proxy.

Perbaikan: teruskan `TRUSTED_PROXIES` secara eksplisit dan dokumentasikan cara menentukan alamat peer proxy yang terlihat dari container. Uji dengan dua IP klien melalui proxy; jangan mengasumsikan alamat loopback host sama dengan peer di jaringan Docker. Docker tidak tersedia untuk reproduksi runtime.

### 3. P2 — Sumber fakta BPJS hasil kurasi dibuang

Lokasi: `backend/seed/curate.go:296`, `backend/seed/curate_apply.go:69`.

Validator mewajibkan `source_url` dan `source_updated_at`, tetapi `CuratedHospital` tidak membawa provenance untuk BPJS. Apply hanya menulis field `bpjs`. Provenance tingkat RS tetap milik MSI, padahal BPJS tidak berasal dari MSI. Setelah import, sumber dan waktu fakta BPJS tidak dapat ditelusuri dari data tersimpan maupun DTO.

Perbaikan: simpan provenance BPJS secara khusus, sesuaikan schema/migration dan kontrak DTO bila diekspos, lalu uji bahwa fakta BPJS beserta sumbernya bertahan setelah apply dan pengulangan apply. Jangan mengganti provenance profil MSI dengan sumber BPJS.

### 4. P2 — Perintah kurasi dengan argumen file belum dapat dijalankan

Lokasi: `backend/cmd/manage/main.go:11`, `backend/main.go:39`.

Kedua entrypoint membatasi jumlah argumen sehingga `curate <file> [--apply]` ditolak sebelum mencapai parser `RunCurate`. Reproduksi aktual dari folder backend:

```text
go run ./cmd/manage curate data/curation/example.placeholder.json
Perintah gagal: usage: go run ./cmd/manage migrate|seed|operator|sync-satusehat|curate
exit status 1
```

Workaround lewat `CURATE_FILE` sudah disebutkan di README, tetapi contoh CLI utama belum berfungsi. Perbaikan: izinkan argumen tambahan khusus untuk `curate`, pertahankan validasi perintah lainnya, dan uji entrypoint publik, bukan hanya fungsi parser.

### 5. P2 — Mengaktifkan suite UI dapat menghasilkan kelulusan tanpa pengujian

Lokasi contoh: `qa/tests/e2e/map-fallback.spec.ts:23`; pola serupa ada pada tiga berkas skenario lainnya.

`SIAGARS_UI=1` menghapus kondisi skip, sedangkan isi tes UI masih berupa komentar tanpa navigasi atau assertion. Dengan browser tersedia, skenario tersebut bisa hijau tanpa menguji aplikasi. Ini berbeda dengan keterbatasan yang wajar berupa frontend belum tersedia: masalahnya adalah status kelulusan yang dapat menyesatkan setelah flag diaktifkan.

Perbaikan: gunakan `test.fixme`/skip tanpa kondisi sampai implementasi UI lengkap, atau implementasikan navigasi, interaksi, dan assertion sebelum menyediakan flag aktivasi.

## Validasi independen

| Pemeriksaan | Hasil |
|---|---|
| `go build ./...` | Lulus |
| `go vet ./...` | Lulus |
| `go test -count=1 ./...` | Lulus; suite integrasi DB skip |
| `gofmt -l cmd internal migrations seed tests main.go` | Tidak ada keluaran |
| `go test -v ./tests/integration` | Seluruh tes DB skip karena `TEST_MONGODB_URI` tidak tersedia; benchmark juga opt-in |
| `go test -race ./internal/service` | Tidak berjalan: CGO nonaktif |
| `npm run typecheck` di `qa` | Lulus |
| `npm run test:list` di `qa` | 13 skenario terdaftar; ini bukan hasil eksekusi E2E |
| Docker/proxy deployment | Tidak dijalankan; Docker tidak tersedia |

Konflik route `/hospitals/compare` sempat menyebabkan panic saat worker masih mengedit. Worker memperbaikinya, dan pengujian ulang lulus. Masalah itu tidak dihitung sebagai temuan terbuka.

Tidak ada pembacaan credential, mutasi database, sinkronisasi SATUSEHAT, atau deployment yang dilakukan oleh reviewer. Bukti integrasi Atlas/benchmark dari worker pada DECISIONS merupakan laporan worker, bukan hasil pengujian ulang reviewer.

## Cakupan hasil dan pekerjaan tersisa

| Paket | Hasil yang tersedia | Batas penyelesaian |
|---|---|---|
| B | Pushdown query MongoDB, migration 003 untuk index, trusted proxy, dokumentasi benchmark | Penyatuan kode wilayah baru berupa usulan; seed bertitik dan MSI tanpa titik masih berbeda |
| C | Pipeline kurasi JSON/CSV, validator, apply idempotent, template dan panduan | Tidak ada 10 profil RS nyata terkurasi; contoh masih placeholder |
| D | Tes kontrak, scaffold Playwright API/UI, checklist QA | E2E frontend, aksesibilitas dan responsif belum dibuktikan |
| E | Checklist lomba, kerangka draft, README publik dan outline pitching | PDF final dan screenshot aplikasi belum tersedia; PRD masih menyebut baseline PostgreSQL |
| F | Dockerfile, Compose, contoh proxy dan runbook | Container/proxy belum tervalidasi dengan Docker |
| G | Endpoint compare 2–3 RS serta tes sort/filter bed; OpenAPI dan fixture diperbarui | Halaman compare dan tombol bagikan belum dibuat |

Frontend belum tersedia di workspace ini. Hasil B–G sebaiknya diterima sebagai fondasi backend dan dokumen pendukung dengan pekerjaan lanjutan yang dicatat di atas, bukan sebagai penyelesaian seluruh fitur UI atau kesiapan produksi.

## Status Orchestrator saat review

Enam dispatch dalam run `run_df19fd4b2d0b` telah berstatus `completed`; lima terminal telah `released` dan terminal F `retained` dengan alasan `user_takeover`. Tampilan koordinator masih menunjukkan proses berjalan dan pesan menunggu F pada pemeriksaan terakhir. Jadi hasil worker sudah dapat direview, tetapi penutupan akhir koordinator belum terkonfirmasi. Reviewer tidak mengambil alih inbox atau mengubah lifecycle run milik koordinator.
