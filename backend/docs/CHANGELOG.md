# Changelog API SiagaRS

Untuk tim frontend. Versi mengikuti `info.version` di [`openapi.yaml`](openapi.yaml) dan tampil di `/docs`. Selama masih 0.x, perubahan bersifat **aditif** kecuali dicatat sebagai "Perubahan perilaku".

## 0.3.1 (26 Okt 2026) - Stabilisasi (BE-01, BE-03, BE-05, BE-06, BE-08)

### Perubahan perilaku
| Area | Sebelum (0.3.0) | Sekarang (0.3.1) |
|---|---|---|
| `POST /admin/bpjs-checks/{id}/reveal` | Membuka NIK/nama asli setelah re-autentikasi | **Dihapus** (404). Keputusan D-03: NIK/nama asli tidak disimpan dalam bentuk apa pun, termasuk terenkripsi. |
| Riwayat cek BPJS | Tersamar + `raw_enc` (AES-GCM) + `nik_hmac` | Hanya tersamar + `nik_hmac` (untuk `repeat_count`). Migrasi `006_bpjs_no_raw` menghapus `raw_enc` dari entri lama dan validator menolaknya. |
| Kode wilayah | Seed `31.71`, MSI `3171` hidup berdampingan (nama kota ganda di `/regions`) | Satu format Kemendagri tanpa titik (`31`, `3171`). Migrasi `007_region_codes`; filter dengan kode bertitik kini **422**. |

### Kontrak (aditif / pelonggaran)
- `GET /admin/referrals`: tiap item punya `hospital_name` (nama RS pemilik). Skema baru `AdminReferral`. FE bisa menghapus lookup nama RS per baris.
- `POST /auth/login`: `csrf_token` kini **opsional** di respons. Bila tidak ada, panggil `GET /auth/csrf` sebelum mutasi. (Backend masih mengirimkannya.)
- `bpjs_data_origin` (Hospital) dan `facility_summary.data_origin` didokumentasikan sebagai `DataOrigin | null`.
- Skema `Error` seragam: `{ error: { code, message, details? }, request_id }`; `code` kini enum yang lengkap (lihat OpenAPI). `details` opsional (`[{field, issue}]`). Gangguan init serverless (503) kini juga memakai skema ini, bukan `{"error":"service_unavailable"}`.

### Internal (di luar kontrak publik)
- `POST /api/v1/internal/jobs/refresh-simulation` (header `X-Job-Secret`) dan `GET` yang sama dengan `Authorization: Bearer <JOB_SECRET>` untuk Vercel Cron (harian 22.00 UTC = 05.00 WIB, `vercel.json`). Menjalankan logika `dummy-facilities` lalu menstempel ulang `source_updated_at` **hanya** item `data_origin=simulation`, sehingga data simulasi `fresh` tiap pagi. Data `curated`/SATUSEHAT tidak pernah ditulis; `verified_at` tidak pernah diisi. Audit `job.refresh_simulation` (aktor `system:cron`). Tanpa `JOB_SECRET` endpoint selalu 401. CLI setara: `go run ./cmd/manage refresh-simulation`.

### Yang harus dilakukan FE
- Ubah copy `/cek-bpjs`: *"Kami tidak menyimpan NIK lengkap Anda. Riwayat pengecekan disimpan dalam bentuk tersamar selama 30 hari."* Hapus UI "Lihat data asli".
- Pakai `hospital_name` di daftar rujukan operator (FE-05).
- Pastikan kode wilayah selalu diambil dari `/regions` (jangan menulis kode bertitik).
- Perbarui `lib/contracts/types.ts`: `LoginResult.csrf_token?`, `Error.error.details?`, `AdminReferral`.

### Operasional
- Env baru `JOB_SECRET` (≥ 32 karakter) di backend; di Vercel set juga `CRON_SECRET` dengan nilai yang sama agar Vercel Cron mengirim bearer token. `BPJS_HISTORY_KEY` kini hanya untuk HMAC.
- Urutan rilis: backup/snapshot Atlas → `go run ./cmd/manage migrate` (005, 006, 007) → `seed` → smoke test.

## 0.3.0 (25 Okt 2026) - branch `develop` (staging)

### Baru: cek BPJS (simulasi) dan riwayat operator
| Endpoint | Fungsi |
|---|---|
| `POST /bpjs/check` | Publik. Mencocokkan NIK + nama + tanggal lahir ke **peserta sintetis**; bukan BPJS Kesehatan. Respons tersamar, `data_origin: "simulation"`, rate limit 10/menit/IP. |
| `GET /admin/bpjs-checks` | Riwayat pemeriksaan (tersamar) untuk semua operator login. |
| `POST /admin/bpjs-checks/{id}/reveal` | Membuka data asli satu entri setelah operator memasukkan ulang passwordnya; diaudit. |

Panduan dan data demo: [`archive/HANDOVER_BE-TO-FE_BPJS.md`](archive/HANDOVER_BE-TO-FE_BPJS.md). Total 22 operasi; perubahan aditif.

### Yang harus dilakukan FE
- Ubah teks persetujuan `/cek-bpjs`: input **dicatat** untuk riwayat operator (tersamar + terenkripsi, dihapus otomatis setelah 30 hari). Usulan kalimat ada di handover.
- Tambahkan halaman riwayat operator dan dialog password untuk "Lihat data asli".
- Backend: migration 005 (koleksi `bpjs_participants`, `bpjs_check_history` dengan index TTL 30 hari) dan env baru `BPJS_HISTORY_KEY` (opsional; tanpa kunci hanya data tersamar yang disimpan dan reveal membalas 409).

## 0.2.1 (25 Okt 2026) - branch `develop` (staging)

### Baru: data contoh fasilitas untuk RS nyata
SATUSEHAT sandbox tidak menyediakan ICU/NICU/PICU/bed/rawat inap/BPJS, jadi RS nyata sebelumnya berisi `unknown`/`null` semua. Agar FE bisa membangun dan menguji UI dengan nama RS nyata, backend menempelkan **data contoh deterministik** (perintah `dummy-facilities`, hanya ke database dev/staging):
- Layanan `icu`, `nicu`, `picu`, `igd-24h`, `ambulance`, `rawat-inap`; bed `general`/`icu`/`nicu`/`picu` (sebagian RS sengaja tanpa bed -> `null`); dan `bpjs`.
- Peluang ketersediaan mengikuti kelas RS (A/B lebih lengkap), tetapi angkanya **bukan fakta**.
- Semua item: `data_origin: "simulation"`, `source_url: null`, `verified_at: null`. `data_origin` RS tetap `satusehat_sandbox`.
- Tidak menimpa data yang sudah ada (kurasi atau pembaruan operator). Mengulang perintah tidak mengubah apa pun.

### Field baru (aditif)
| Field | Isi |
|---|---|
| `bpjs_data_origin` (Hospital) | Asal nilai `bpjs`: `simulation`, `curated`, atau `null` bila `bpjs` null. |
| `facility_summary.data_origin` | `simulation` bila ada angka contoh di ringkasan; asal data bersumber bila tidak; `null` bila tidak ada data. |

### Konfigurasi
- Backend menerima login dari beberapa origin FE (`PUBLIC_ORIGIN` berdaftar). Production: `https://frontend-siagars.vercel.app` dan `http://localhost:3000`; staging: `http://localhost:3000` dan domain FE yang sama. FE cukup mengganti `API_BASE_URL`. Tidak ada perubahan kontrak API.

### Yang harus dilakukan FE
- Tampilkan label **"Simulasi"** di badge/ringkasan bila `facility_summary.data_origin == "simulation"` dan di BPJS bila `bpjs_data_origin == "simulation"`. Jangan menyajikannya sebagai fakta RS.
- `freshness` data contoh dihitung dari `source_updated_at` seperti biasa, jadi berubah menjadi `stale` setelah 24 jam sejak data ditempel. Itu perilaku yang benar, bukan bug.
- Filter `services=icu` kini mengembalikan RS nyata berstatus contoh `available`/`limited`. Hasil ini tetap simulasi.
- Begitu backend mengkurasi data bersumber, nilai contoh diganti (field sama, `data_origin: "curated"`). Tidak perlu mengubah kode.

## 0.2.0 (25 Okt 2026) - branch `develop` (staging)

### Baru
| Endpoint | Fungsi |
|---|---|
| `GET /regions` | Provinsi + kota/kabupaten; pakai `code` apa adanya untuk `province_code`/`city_code`. |
| `GET /hospitals/suggest?q=&limit=&dataset=` | Autocomplete nama RS (maks 20 hasil). |
| `GET /admin/hospitals` | RS dalam scope operator (butuh login), lengkap dengan `version` layanan/bed. |
| `GET /admin/hospitals/{id}` | Detail satu RS dalam scope operator; di luar scope = 403. |

### Field baru pada objek `Hospital` (list, detail, compare, tracking)
| Field | Isi |
|---|---|
| `external_id`, `source_namespace` | Kode dari sumber (SATUSEHAT `kode_satusehat`); `null` untuk simulasi. |
| `facility_class`, `facility_subtype` | `{code, name}` atau `null` (kelas RS A/B/C/D, jenis RS). |
| `operational` | `true/false/null`. |
| `bpjs_source_url`, `bpjs_source_updated_at` | Sumber dan tanggal data BPJS; `null` bila belum dikurasi. |
| `facility_summary` | `{icu, nicu, picu, rawat_inap, bpjs, inpatient_beds{total,available,freshness}, last_source_updated_at}`. Pakai ini untuk badge/filter di UI. |

### Perubahan perilaku
- `GET /hospitals/{id}` dan `GET /hospitals/compare` menerima **UUID atau `external_id`**. Dua ID yang merujuk RS sama (UUID dan kodenya) ditolak `422`. ID yang bukan UUID dan bukan kode valid kini `404` (sebelumnya `422` pada compare).
- Parameter `q` berawalan `$` kini teks biasa (bug lama diperbaiki).
- Semua respons menambah header `X-Frame-Options: DENY` dan `Referrer-Policy: no-referrer`.
- Layanan baru `rawat-inap` muncul di `GET /services` dan di `services` RS simulasi. Bed rawat inap tetap kategori `general`.

### Yang harus diperhatikan FE
- **RS nyata dari SATUSEHAT belum punya data ICU/NICU/PICU/bed/BPJS.** `facility_summary` berisi `unknown`/`null`. Tampilkan sebagai "belum diketahui", bukan "tidak tersedia". Data lengkap untuk pengembangan UI ada di `dataset=simulation` (atau `all`).
- `facility_class.name` dan `facility_subtype.name` terisi di environment yang sudah sinkron ulang; bila `null`, tampilkan `code`.
- Tidak ada field yang dihapus atau diganti nama; kode FE 0.1.0 tetap jalan.

### Environment
- Staging (`develop`): URL Vercel Preview branch `develop`, database `siagars_dev`. Akun uji operator dibagikan terpisah oleh pemilik backend (bukan di repo).
- Production (`main`) tetap 0.1.0 sampai dirilis.

## 0.1.0 (24 Okt 2026)
Baseline: 15 operasi (health, layanan, daftar/detail/compare RS, lookup tracking, auth operator, admin referral/layanan/bed).
