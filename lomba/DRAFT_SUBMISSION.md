# Kerangka PDF First Draft - SiagaRS

Bahan untuk dokumen PDF First Draft Submission WIA (MIA 2026). Ini kerangka isi per halaman, bukan PDF jadi. Pindahkan ke PowerPoint/Google Slides/Canva lalu ekspor ke PDF.

Sumber aturan: Guidebook WIA bagian J dan PPT Technical Meeting (slide "First Draft Submission"). Sumber isi versi berjalan: `backend/PRD.md`, `backend/README.md`, `backend/docs/DECISIONS.md`, `backend/docs/CHANGELOG.md`, dan `backend/docs/openapi.yaml` (API 0.3.0 di staging, 0.2.1 di production; diperbarui 26 Oktober 2026).

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
├── /cek-bpjs            Cek kepesertaan BPJS simulasi (peserta sintetis)
├── /tentang-data        Cakupan, provenance, kesegaran, keterbatasan
├── /bandingkan          2-3 RS (prioritas Should)
└── /operator
    ├── /operator/login
    ├── /operator/rujukan
    ├── /operator/layanan
    └── /operator/riwayat-bpjs  Riwayat cek BPJS (tersamar, lihat data asli dengan password)
```

**[ISI: gambar diagram sitemap di slide. Tandai halaman yang sudah ada vs masih rencana. Status halaman frontend tidak tercatat di dokumen backend; frontend ter-deploy di `https://frontend-siagars.vercel.app`, nama rute final ikuti frontend.]**

## Halaman 4 - Alur pengguna

Tiga peran dari Scope_MVP bagian 02 dan 06:

| Peran | Alur |
|---|---|
| Keluarga pasien | cari lokasi, pilih layanan, hasil peta/daftar, detail sumber dan status, hubungi RS atau buka navigasi |
| Pelacak kode demo | masukkan kode sintetis, status saat ini, riwayat waktu, langkah berikutnya (seluruh halaman berlabel simulasi) |
| Pengecek BPJS (simulasi) | masukkan NIK, nama, tanggal lahir peserta demo; lihat status kepesertaan tersamar berlabel simulasi |
| Operator demo | login, daftar rujukan milik RS, ubah status sesuai transisi, simpan; pengguna refresh dan melihat event baru. Operator juga mengubah status layanan/bed pada RS yang ditugaskan dan melihat riwayat cek BPJS |

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
| Should | bandingkan 2-3 RS; sort kecocokan/jarak; filter bed; berbagi profil; autocomplete nama RS |
| Tambahan 0.3.0 | cek kepesertaan BPJS simulasi + riwayat operator (backend siap di staging) |
| Could | favorit lokal, geolokasi, PWA, dark mode, statistik persebaran |
| Di luar V0.1 | diagnosis, rekam medis, booking, pembayaran, chat dokter, klaim BPJS, cek BPJS Kesehatan sungguhan, ambulans live, rujukan produksi |

**[ISI: beri tanda tiap fitur: sudah jalan / sedang dikerjakan / rencana, berdasarkan status frontend nyata saat draft dibekukan. Jangan menandai "sudah jalan" tanpa menjalankannya.]**

## Halaman 7 - Bagian backend dan API

Isi yang didukung kode dan README backend:

- Stack: Go 1.27.1, net/http ServeMux, MongoDB Atlas lewat go.mongodb.org/mongo-driver/v2 v2.9.2, bcrypt cost 12.
- Arsitektur: handler HTTP, service (aturan bisnis), repository (MongoDB), domain (state machine, freshness, Haversine), adapter SATUSEHAT. Prefix `/api/v1`, JSON snake_case, timestamp RFC3339 UTC.
- Kontrak saat ini (0.3.0) memiliki 22 operasi API; production masih 0.2.1 (19 operasi, tanpa cek BPJS). Sumber kontrak adalah `docs/openapi.yaml`; Scalar API Reference tersedia di `/docs` (publik: `https://siagars-backend.vercel.app/docs`):

| Method | Path | Fungsi |
|---|---|---|
| GET | /health, /ready | liveness; readiness DB |
| GET | /services, /regions | master layanan; provinsi + kota/kabupaten |
| GET | /hospitals, /hospitals/{id} | pencarian/filter/pagination; detail (UUID atau kode SATUSEHAT) |
| GET | /hospitals/suggest | autocomplete nama RS |
| GET | /hospitals/compare | bandingkan 2-3 RS |
| POST | /referrals/lookup | tracking sintetis lewat body |
| POST | /bpjs/check | cek kepesertaan BPJS simulasi (0.3.0) |
| POST | /auth/login, /auth/logout | session operator |
| GET | /auth/me, /auth/csrf | profil+RS izin; token CSRF |
| GET | /admin/hospitals, /admin/hospitals/{id} | RS dalam scope operator |
| GET | /admin/referrals | rujukan RS operator |
| PATCH | /admin/referrals/{id}/status | ubah status (version, CSRF, scope RS) |
| PATCH | /admin/hospitals/{id}/services/{slug} | ubah status layanan |
| PATCH | /admin/hospitals/{id}/beds/{category} | ubah bed |
| GET | /admin/bpjs-checks | riwayat cek BPJS (tersamar) (0.3.0) |
| POST | /admin/bpjs-checks/{id}/reveal | buka data asli dengan password ulang, diaudit (0.3.0) |

- Aturan yang sudah ada di kode: filter `services` memakai AND; limit default 20 maksimum 50; parameter tak dikenal 422; `dataset=real|simulation|all`; state machine rujukan submitted, verified, searching, confirmed, completed (rejected terminal); optimistic locking version dan konflik 409; session cookie HttpOnly dengan CSRF dan validasi Origin; rate limit login, lookup, dan cek BPJS; header `X-Frame-Options` dan `Referrer-Policy`.
- Keamanan data: tidak ada data pasien atau rekam medis; kode tracking dan token session disimpan sebagai hash; credential tidak ada di repo. Cek BPJS simulasi mencatat input pengguna untuk riwayat operator: tersamar, data asli terenkripsi AES-256-GCM, dihapus otomatis setelah 30 hari; tulis demikian, jangan menulis "tidak menyimpan NIK".
- Catatan untuk juri: Q&A panitia no. 34 menyatakan backend menjadi poin plus selain frontend. Itu pernyataan panitia, bukan jaminan nilai.

**[PLACEHOLDER: diagram arsitektur: Browser, Next.js di Vercel (rewrite `/api/v1`), Go API serverless di Vercel, MongoDB Atlas, adapter SATUSEHAT Sandbox (manual, read-only).]**

## Halaman 8 - Data, sumber dan kesegaran

- Empat atribut per informasi: `data_origin` (curated, simulation, satusehat_sandbox, satusehat_production), `source_updated_at`, `synced_at`, `verified_at`. Layanan dan bed punya metadata sendiri.
- Kesegaran (kebijakan demo, bukan standar medis): fresh <1 jam; aging 1-24 jam; stale >24 jam; unknown bila timestamp tidak ada atau tidak valid.
- Null berbeda dari nol: `available=null` berarti belum diketahui, `available=0` berarti menurut sumber tidak ada bed kosong; BPJS null bukan false.
- Seed sintetis: 30 RS fiktif bernama RS Simulasi Siaga 01..30, 6 layanan (termasuk `rawat-inap`), 4 kategori bed; seluruhnya berlabel simulation. Seed juga memuat 5 peserta BPJS sintetis (NIK fiktif berawalan `9999`). Alamat dan koordinat sintetis. Q&A panitia no. 5 dan 40 mengizinkan data dummy tetapi menyarankan data nyata atau mendekati nyata.
- Data nyata: adapter Master Sarana Index SATUSEHAT **Sandbox** (read-only) berhasil diuji live pada 23 Oktober 2026 untuk Jabodetabek: 14 kab/kota, 516 RS dibuat, 0 skip/gagal; run kedua 516 unchanged. Tulis sebagai "data Sandbox", bukan data produksi. MSI tidak menyediakan layanan/bed/BPJS. Agar UI bisa dibangun dengan nama RS nyata, RS MSI diberi **data contoh** deterministik berlabel `simulation` (`facility_summary.data_origin`, `bpjs_data_origin`); itu bukan fakta RS dan wajib berlabel "Simulasi" di slide. Data RS juga memuat kelas/jenis RS (`facility_class`, `facility_subtype`) dari MSI. Pipeline kurasi `curate` (sumber + tanggal wajib) sudah ada. **[BELUM TERBUKTI: kurasi 30-50 RS dengan minimal 10 profil lengkap bersumber belum ada.]**
- Jangan menyatakan "terhubung ke SATUSEHAT produksi", "rujukan resmi", atau "cek BPJS Kesehatan": tidak ada di kode. Tidak ada API publik BPJS untuk cek kepesertaan; fitur ini simulasi.

## Halaman 9 - Screenshot halaman yang sudah dikerjakan

Wajib minimal 2 halaman: landing page + 1 halaman lain.

| No. | Halaman | Status |
|---|---|---|
| 1 | Landing (`/`) | **[PLACEHOLDER: screenshot nyata]** |
| 2 | Pilih salah satu: hasil pencarian / detail RS / lacak rujukan / operator | **[PLACEHOLDER: screenshot nyata]** |
| 3 (opsional) | Scalar API Reference `/docs` untuk bagian API | **[PLACEHOLDER: screenshot nyata dari `https://siagars-backend.vercel.app/docs`]** |

Aturan: ambil screenshot dari aplikasi yang benar-benar berjalan; data simulasi harus terlihat berlabel; jangan menampilkan secret, email operator nyata, atau URL query berisi kode. Backend production di Vercel lolos 32 pemeriksaan publik dan 15 pemeriksaan login/mutasi operator dari origin FE (25 Oktober 2026); screenshot frontend tetap harus diambil dari `https://frontend-siagars.vercel.app` atau FE lokal yang benar-benar berjalan.

## Halaman 10 - Progres dan rencana

Progres yang bisa dibuktikan per 26 Oktober 2026: API Go + MongoDB Atlas (22 operasi, versi 0.3.0 di staging), migration 001-005, seed idempotent, adapter MSI Sandbox (516 RS), data contoh fasilitas berlabel simulasi, `facility_summary`, wilayah dan autocomplete, compare RS (UUID atau kode SATUSEHAT), query pushdown, benchmark pencarian, pipeline kurasi, cek BPJS simulasi + riwayat operator terenkripsi, dokumentasi OpenAPI (Scalar), deployment Vercel production dan staging, serta uji login/mutasi operator di production dari origin FE. Gofmt, build, vet, unit test, contract test, dan suite integrasi Atlas tercatat lulus.

Belum terbukti dan harus tertulis jujur: E2E UI frontend-backend, suite `qa/` terhadap production, rilis cek BPJS ke production, transisi status rujukan di production, Docker, race detector, kurasi RS nyata bersumber, uji usability <60 detik, dan SATUSEHAT/BPJS produksi. Status frontend **[ISI]**.

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
