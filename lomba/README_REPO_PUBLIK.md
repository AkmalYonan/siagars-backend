# Template README untuk repository publik

Panduan: aturan panitia (Guidebook bagian J, PPT Technical Meeting) mewajibkan README.md di repository publik (GitHub/GitLab) berisi empat hal: judul + deskripsi singkat 1-2 kalimat, tech stack, cara menjalankan (clone, install, run), dan progres saat ini. Template di bawah garis memenuhi keempatnya. Salin isi di bawah garis ke README.md repo yang akan dikumpulkan, lalu ganti semua tanda **[ISI]**.

Aturan isi:
- Tanpa secret: jangan memasukkan isi `.env`, `atlas-credentials.env`, MONGODB_URI, password, client secret SATUSEHAT, atau token apa pun. Hanya nama variabel dan nilai placeholder.
- Periksa sebelum push: `.gitignore` backend sudah mengecualikan `.env`, `.env.*` dan `*.env` (kecuali `.env.example`). Pastikan riwayat commit juga bersih, bukan hanya berkas terbaru.
- Tulis hanya yang benar-benar berjalan. Perintah yang belum diuji harus diberi catatan.
- Backend dan frontend adalah dua proyek terpisah (Go dan Next.js). Putuskan apakah dikumpulkan sebagai dua repo atau satu; panitia hanya meminta satu link repo publik. **[ISI: keputusan tim]**
- Bagian cara menjalankan di bawah diambil dari backend/README.md. Bagian frontend sengaja kosong karena tidak ada di dokumen backend.

---

# SiagaRS

**Temukan rumah sakit yang tepat, bukan sekadar yang terdekat.**

SiagaRS membantu keluarga pasien mencari rumah sakit berdasarkan kebutuhan fasilitas, melihat sumber dan umur setiap informasi, serta melacak progres rujukan simulasi. Versi 0.1 adalah prototipe lomba Web In Action (MIA 2026) oleh Tim MalasNgoding; seluruh data rujukan dan operator bersifat sintetis dan tidak ada data pasien nyata.

> Bukan layanan medis. SiagaRS tidak memberikan diagnosis atau triase, tidak menyimpan rekam medis, dan tidak memproses rujukan medis resmi. Ketersediaan bed pada prototipe tidak menjamin penerimaan pasien; konfirmasi selalu lewat kontak rumah sakit.

## Tim

Radja Admiral (ketua), M. Tauriq Arya (frontend/integrasi), Akmal Bariq Yonanda (backend). **[ISI: sesuaikan dengan nama lengkap pada pendaftaran]**

## Tech stack

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.27.1, net/http ServeMux |
| Database | MongoDB Atlas (replica set) lewat go.mongodb.org/mongo-driver/v2 v2.9.2 |
| Auth | session cookie HttpOnly + CSRF, password bcrypt cost 12 |
| Dokumentasi API | OpenAPI 3.0.3 (`backend/docs/openapi.yaml`), Swagger UI di `/docs` |
| Integrasi opsional | SATUSEHAT Master Sarana Index Sandbox, read-only, nonaktif secara default |
| Frontend | Next.js **[ISI: versi, UI library, peta; tidak tercatat di dokumen backend]** |

Versi dependency Go dikunci di `go.mod`/`go.sum`.

## Cara menjalankan

### Backend

Prasyarat: Go 1.27.1 dan cluster MongoDB Atlas (replica set; mongod standalone tidak didukung karena transaksi multi-dokumen). IP mesin harus diizinkan di Network Access Atlas.

```powershell
git clone [ISI: URL repo]
cd [ISI: folder backend]

# 1. Salin .env.example menjadi .env (berkas ini tidak masuk git), lalu isi:
#    MONGODB_URI=mongodb+srv://<user>:<password>@<cluster>/...   (placeholder, isi milik sendiri)
#    MONGODB_DATABASE=siagars
#    APP_ENV=development
#    PUBLIC_ORIGIN=http://localhost:3000
#    DEMO_MODE=true
#    SATUSEHAT_ENABLED=false

go mod download
go run . migrate     # terapkan migration bernomor
go run . seed        # seed sintetis idempotent (30 RS fiktif)
go run .             # API di http://localhost:8080
```

Cek: `GET http://localhost:8080/api/v1/health` (liveness) dan `/api/v1/ready` (DB siap). Dokumentasi interaktif: http://localhost:8080/docs.

Akun operator demo tidak disediakan dan tidak ada password bawaan. Buat sendiri setelah seed (email dan password milik sendiri, minimal 12 karakter) dengan `go run ./cmd/manage operator` dan variabel `OPERATOR_EMAIL`, `OPERATOR_PASSWORD`, `OPERATOR_HOSPITAL_IDS`; lihat backend/README.md.

Perintah `migrate`/`seed` tidak mereset atau menghapus database.

### Frontend

**[ISI: clone, install, run frontend. Di development, Next.js meneruskan `/api/v1/*` ke `http://localhost:8080` lewat rewrite; Origin browser harus sama dengan PUBLIC_ORIGIN.]**

### Sinkronisasi data Sandbox SATUSEHAT (opsional)

Hanya jika tim punya credential Sandbox yang sah. Tidak diperlukan untuk menjalankan demo; kosongkan semua variabel SATUSEHAT jika tidak punya. Rincian dan batasannya ada di backend/README.md, bagian "Sinkronisasi SATUSEHAT MSI Sandbox".

## Pengujian

```powershell
gofmt -l cmd internal migrations seed tests
go build ./...
go vet ./...
go test ./...
```

Tes integrasi berbasis database bersifat opt-in lewat `TEST_MONGODB_URI` dan membuat database sementara sendiri; tes tidak menyentuh database `siagars`.

## Progres saat ini

Diisi sesuai bukti pada hari draft dibekukan. Status backend per 24 Oktober 2026 (dari backend/README.md):

- [x] REST API Go + MongoDB Atlas: pencarian RS (filter layanan AND, BPJS, wilayah, sort nama/jarak/kecocokan, pagination), detail, master layanan
- [x] Provenance per item (asal data, waktu sumber, waktu sinkronisasi, verifikasi) dan kesegaran fresh/aging/stale/unknown
- [x] Tracking rujukan sintetis dengan state machine dan riwayat publik
- [x] Auth operator (session, CSRF, scope RS) dan update status rujukan/layanan/bed dengan optimistic locking
- [x] Migration, seed idempotent, dokumentasi OpenAPI
- [x] Adapter SATUSEHAT MSI Sandbox read-only (teruji dengan mock dan live Sandbox Jabodetabek)
- [x] Query pushdown MongoDB, index pencarian, compare 2-3 RS, dan benchmark pencarian
- [x] Deployment backend publik di Vercel: `https://siagars-backend.vercel.app`
- [ ] Frontend Next.js: **[ISI: landing, pencarian, detail, lacak rujukan, operator; centang hanya yang sudah berjalan]**
- [ ] Uji end-to-end frontend dengan backend (belum ada bukti)
- [ ] Kurasi data RS nyata, termasuk layanan/bed/BPJS (belum ada)
- [x] Benchmark kinerja pencarian backend
- [ ] Docker Compose dan race detector (belum teruji)
- [ ] Login operator, mutasi, dan suite `qa/` terhadap deployment produksi

## Sumber data dan batasan

- Seed berisi 30 RS fiktif ("RS Simulasi Siaga 01..30") yang selalu berlabel `simulation`.
- Data dari SATUSEHAT MSI Sandbox berlabel `satusehat_sandbox`; itu data Sandbox, bukan data produksi, dan belum memuat layanan, bed, atau BPJS.
- Nilai yang tidak diketahui ditampilkan sebagai tidak diketahui (null), bukan nol.
- Tidak ada integrasi produksi SATUSEHAT, FHIR, maupun rujukan resmi.

## Dokumen

- `backend/PRD.md` - kebutuhan dan kontrak backend
- `backend/docs/openapi.yaml` - kontrak API
- `backend/docs/DECISIONS.md` - keputusan implementasi

## Lisensi

**[ISI: belum ditentukan di dokumen. Hak cipta karya tetap milik peserta menurut Guidebook; pilih lisensi atau tulis "semua hak dilindungi".]**
