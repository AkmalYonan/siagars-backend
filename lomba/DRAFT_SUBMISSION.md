# Kerangka PDF First Draft - SiagaRS

Bahan untuk dokumen PDF First Draft Submission WIA (MIA 2026). Ini kerangka isi per halaman, bukan PDF jadi. Pindahkan ke PowerPoint/Google Slides/Canva lalu ekspor ke PDF.

Sumber aturan: Guidebook WIA bagian J dan PPT Technical Meeting (slide "First Draft Submission"). Sumber isi: backend/PRD.md, backend/README.md, backend/docs/DECISIONS.md, siagars-v0.1/sources/Scope_MVP.md dan Teknis_Sistem.md.

## Ketentuan berkas (dari dokumen panitia)

| Item | Ketentuan |
|---|---|
| Format | PDF dari PowerPoint/Google Slides/Canva |
| Batas | maksimal 10 halaman dan 10 MB |
| Nama berkas | `WIA_FSDMIA2026_NamaTim` (versi PPT Technical Meeting; Guidebook menulis `WIA_FDSMIA2025_NamaTim`, lihat CHECKLIST_LOMBA.md) |
| Isi wajib | konsep singkat (latar belakang, tujuan, masalah); sitemap; wireframe/mockup sederhana tampilan utama; penjelasan fitur utama yang direncanakan; screenshot minimal 2 halaman yang sudah dikerjakan (wajib landing page + 1 halaman lain) |
| Pengumpulan | lewat website MIA 2026, bersama link repo publik (lihat README_REPO_PUBLIK.md) |
| Dampak penilaian | menurut Q&A panitia no. 4, draft tidak memengaruhi nilai akhir dan tidak mengeliminasi; tujuannya feedback mentor |

Anggaran halaman (total 10):

| Hal. | Isi |
|---|---|
| 1 | Cover |
| 2 | Konsep: latar belakang, tujuan, masalah |
| 3 | Sitemap |
| 4 | Alur pengguna (3 peran) |
| 5 | Wireframe/mockup tampilan utama |
| 6 | Fitur utama yang direncanakan |
| 7 | Bagian backend dan API |
| 8 | Data, sumber dan kesegaran (provenance) |
| 9 | Screenshot halaman yang sudah dikerjakan |
| 10 | Progres, keterbatasan dan rencana sampai final |

Legenda tanda: **[ISI]** = diisi tim; **[BELUM TERBUKTI]** = jangan ditulis sebagai fakta sebelum ada bukti; **[PLACEHOLDER]** = ganti dengan gambar nyata.

---

## Halaman 1 - Cover

- Nama produk: SiagaRS
- Nama tim: MalasNgoding (nama berkas memakai `NamaTim` sesuai format panitia **[ISI: tulis persis seperti di pendaftaran]**)
- Anggota (nama lengkap tanpa singkatan, sesuai pendaftaran): Radja Admiral (ketua), M. Tauriq Arya, Akmal Bariq Yonanda **[ISI: pastikan ejaan nama lengkap sama dengan formulir pendaftaran]**
- Tagline dari Scope_MVP: "Temukan rumah sakit yang tepat, bukan sekadar yang terdekat."
- Cabang: Web In Action (WIA), MIA 2026
- Subtema: **[ISI: belum ditetapkan di dokumen.]** Subtema WIA yang terbaca di PPT Technical Meeting antara lain Public Service Tracking (status/progres layanan publik) dan Data-Driven Decision Support. Fitur tracking rujukan dan transparansi sumber data SiagaRS dekat dengan keduanya, tetapi pilihan resmi harus diputuskan tim. Q&A no. 31 menyarankan fokus pada satu subtema.

## Halaman 2 - Konsep singkat

Isi dari Scope_MVP bagian 01, apa adanya:

- Ringkasan: SiagaRS membantu keluarga pasien menemukan rumah sakit berdasarkan kebutuhan fasilitas, memeriksa sumber dan umur informasi, menghubungi rumah sakit, dan memantau progres rujukan simulasi.
- Hipotesis masalah (tulis sebagai hipotesis, bukan temuan riset): informasi rumah sakit tersebar; lokasi terdekat belum tentu sesuai fasilitas; umur informasi tidak jelas; progres rujukan sulit dipantau. Dokumen sumber menyatakan ini berasal dari brainstorming dan perlu divalidasi lewat wawancara. **[BELUM TERBUKTI: tidak ada hasil wawancara pengguna di dokumen. Jangan menulis statistik atau kutipan pengguna.]**
- Tujuan: pencarian berbasis kebutuhan, informasi yang dapat ditelusuri, dan timeline status yang jelas.
- Target produk: pengguna menemukan profil RS yang sesuai dalam kurang dari 60 detik pada skenario uji. **[BELUM TERBUKTI: ini target; uji usability belum dilakukan.]**
- Batas versi: bukan diagnosis, triase, rekam medis, booking, pembayaran, klaim BPJS ataupun rujukan medis produksi. Tidak ada data pasien nyata.

## Halaman 3 - Sitemap

Struktur halaman rencana dari Scope_MVP bagian 08:

```
/                        Beranda (manfaat, cakupan pilot, CTA)
├── /rumah-sakit         Pencarian, filter, peta/daftar, pagination
│   └── /rumah-sakit/[id]  Profil, fasilitas, bed, sumber, telepon/rute
├── /lacak-rujukan       Form kode demo + timeline sintetis
├── /tentang-data        Cakupan, provenance, kesegaran, keterbatasan
├── /bandingkan          2-3 RS (prioritas Should)
└── /operator
    ├── /operator/login
    ├── /operator/rujukan
    └── /operator/layanan
```

**[ISI: gambar diagram sitemap di slide. Tandai halaman yang sudah ada vs masih rencana. Status halaman frontend tidak tercatat di dokumen backend.]**

## Halaman 4 - Alur pengguna

Tiga peran dari Scope_MVP bagian 02 dan 06:

| Peran | Alur |
|---|---|
| Keluarga pasien | cari lokasi, pilih layanan, hasil peta/daftar, detail sumber dan status, hubungi RS atau buka navigasi |
| Pelacak kode demo | masukkan kode sintetis, status saat ini, riwayat waktu, langkah berikutnya (seluruh halaman berlabel simulasi) |
| Operator demo | login, daftar rujukan milik RS, ubah status sesuai transisi, simpan; pengguna refresh dan melihat event baru. Operator juga mengubah status layanan/bed pada RS yang ditugaskan |

Cerita demo (Scope_MVP bagian 07): "Rina" mencari RS dengan NICU dan BPJS di wilayah pilot, membedakan data yang diketahui, stale dan simulasi, memeriksa kontak, lalu melacak kode demo. **[ISI: gambar user journey; Rina adalah tokoh fiktif untuk skenario, tulis demikian.]**

## Halaman 5 - Wireframe/mockup

**[PLACEHOLDER: tempel wireframe/mockup tampilan utama: beranda, hasil pencarian (peta+daftar), detail RS, lacak rujukan, dashboard operator.]**

Panduan konten (dari Scope_MVP bagian 09): pencarian publik tanpa login; mobile jadi prioritas; status memakai teks/ikon selain warna; waktu WIB; peta selalu punya alternatif daftar; hasil kosong menawarkan reset filter; request gagal menawarkan retry; sumber dan waktu di dekat informasi yang dirujuk.

Catatan: Q&A panitia no. 24 dan 36 menyatakan desain Figma high-fidelity tidak wajib untuk WIA, jadi wireframe sederhana cukup.

## Halaman 6 - Fitur utama yang direncanakan

Prioritas dari Scope_MVP bagian 03:

| Prioritas | Fitur |
|---|---|
| Must | landing; pencarian nama/kota/provinsi; filter layanan/BPJS; peta+daftar; detail; sumber+waktu; tracking sintetis; operator minimum; UI state lengkap |
| Should | bandingkan 2-3 RS; sort kecocokan/jarak; filter bed; berbagi profil |
| Could | favorit lokal, geolokasi, PWA, dark mode, statistik persebaran |
| Di luar V0.1 | diagnosis, rekam medis, NIK, booking, pembayaran, chat dokter, klaim BPJS, ambulans live, rujukan produksi |

**[ISI: beri tanda tiap fitur: sudah jalan / sedang dikerjakan / rencana, berdasarkan status frontend nyata saat draft dibekukan. Jangan menandai "sudah jalan" tanpa menjalankannya.]**

## Halaman 7 - Bagian backend dan API

Isi yang didukung kode dan README backend:

- Stack: Go 1.27.1, net/http ServeMux, MongoDB Atlas lewat go.mongodb.org/mongo-driver/v2 v2.9.2, bcrypt cost 12.
- Arsitektur: handler HTTP, service (aturan bisnis), repository (MongoDB), domain (state machine, freshness, Haversine), adapter SATUSEHAT. Prefix `/api/v1`, JSON snake_case, timestamp RFC3339 UTC.
- Endpoint (14 endpoint pada tabel PRD bagian 3; sumber kontrak `docs/openapi.yaml`, Swagger UI di `/docs` saat API berjalan):

| Method | Path | Fungsi |
|---|---|---|
| GET | /health, /ready | liveness; readiness DB |
| GET | /services | master layanan |
| GET | /hospitals, /hospitals/{id} | pencarian/filter/pagination; detail |
| POST | /referrals/lookup | tracking sintetis lewat body |
| POST | /auth/login, /auth/logout | session operator |
| GET | /auth/me, /auth/csrf | profil+RS izin; token CSRF |
| GET | /admin/referrals | rujukan RS operator |
| PATCH | /admin/referrals/{id}/status | ubah status (version, CSRF, scope RS) |
| PATCH | /admin/hospitals/{id}/services/{slug} | ubah status layanan |
| PATCH | /admin/hospitals/{id}/beds/{category} | ubah bed |

- Aturan yang sudah ada di kode: filter `services` memakai AND; limit default 20 maksimum 50; parameter tak dikenal 422; `dataset=real|simulation|all`; state machine rujukan submitted, verified, searching, confirmed, completed (rejected terminal); optimistic locking version dan konflik 409; session cookie HttpOnly dengan CSRF dan validasi Origin; rate limit login dan lookup.
- Keamanan data: tidak ada data pasien; kode tracking dan token session disimpan sebagai hash; credential tidak ada di repo.
- Catatan untuk juri: Q&A panitia no. 34 menyatakan backend menjadi poin plus selain frontend. Itu pernyataan panitia, bukan jaminan nilai.

**[PLACEHOLDER: diagram arsitektur: Browser, reverse proxy, Next.js, Go API, MongoDB Atlas, adapter SATUSEHAT Sandbox (manual, read-only).]**

## Halaman 8 - Data, sumber dan kesegaran

- Empat atribut per informasi: `data_origin` (curated, simulation, satusehat_sandbox, satusehat_production), `source_updated_at`, `synced_at`, `verified_at`. Layanan dan bed punya metadata sendiri.
- Kesegaran (kebijakan demo, bukan standar medis): fresh <1 jam; aging 1-24 jam; stale >24 jam; unknown bila timestamp tidak ada atau tidak valid.
- Null berbeda dari nol: `available=null` berarti belum diketahui, `available=0` berarti menurut sumber tidak ada bed kosong; BPJS null bukan false.
- Seed sintetis: 30 RS fiktif bernama RS Simulasi Siaga 01..30, 5 layanan, 4 kategori bed; seluruhnya berlabel simulation. Alamat dan koordinat sintetis. Q&A panitia no. 5 dan 40 mengizinkan data dummy tetapi menyarankan data nyata atau mendekati nyata.
- Data nyata: adapter Master Sarana Index SATUSEHAT **Sandbox** (read-only) berhasil diuji live pada 23 Oktober 2026 untuk Jabodetabek: 14 kab/kota, 516 RS dibuat, 0 skip/gagal; run kedua 516 unchanged. Tulis sebagai "data Sandbox", bukan data produksi. RS dari MSI belum punya layanan/bed/BPJS (null/kosong), sehingga filter layanan pada data nyata belum menghasilkan apa pun. **[BELUM TERBUKTI: kurasi 30-50 RS dengan minimal 10 profil lengkap belum ada.]**
- Jangan menyatakan "terhubung ke SATUSEHAT produksi" atau "rujukan resmi": tidak ada di kode.

## Halaman 9 - Screenshot halaman yang sudah dikerjakan

Wajib minimal 2 halaman: landing page + 1 halaman lain.

| No. | Halaman | Status |
|---|---|---|
| 1 | Landing (`/`) | **[PLACEHOLDER: screenshot nyata]** |
| 2 | Pilih salah satu: hasil pencarian / detail RS / lacak rujukan / operator | **[PLACEHOLDER: screenshot nyata]** |
| 3 (opsional) | Swagger UI `/docs` untuk bagian API | **[PLACEHOLDER: screenshot nyata dari API lokal]** |

Aturan: ambil screenshot dari aplikasi yang benar-benar berjalan; data simulasi harus terlihat berlabel; jangan menampilkan secret, email operator nyata, atau URL query berisi kode. Backend baru terbukti lewat smoke test HTTP lokal (README); bukti integrasi frontend ke backend belum ada.

## Halaman 10 - Progres dan rencana

Progres yang bisa dibuktikan (README backend, 23 Oktober 2026): baseline API Go + MongoDB Atlas, migration 001-002, seed idempotent, adapter MSI Sandbox, dokumentasi OpenAPI; gofmt, build, vet dan unit test lulus; suite integrasi lulus pada database tes sementara di Atlas.

Belum terbukti dan harus tertulis jujur: E2E frontend-backend, benchmark kinerja, Docker, race detector, kurasi RS nyata, uji usability <60 detik, deployment publik. Status frontend **[ISI]**.

Rencana dari Scope_MVP bagian 10: QA responsif/aksesibilitas 30 Oktober; freeze draft 31 Oktober; First Draft 1 November; review mentor 3-6 November; feedback 7 November; revisi 8-10 November; final 11 November.

---

## Daftar periksa sebelum ekspor PDF

- [ ] Total halaman <= 10 dan ukuran <= 10 MB (periksa ukuran berkas setelah ekspor)
- [ ] Nama berkas mengikuti format yang dikonfirmasi panitia
- [ ] Screenshot minimal 2 halaman nyata (landing + 1 lain)
- [ ] Semua tanda [ISI], [PLACEHOLDER], [BELUM TERBUKTI] sudah diganti atau dihapus
- [ ] Tidak ada angka atau klaim yang tidak ada di dokumen/kode
- [ ] Tidak ada credential, data pasien, atau data RS karangan yang diklaim nyata
- [ ] PDF membuka dengan benar di peramban dan tautan repo dapat diklik
