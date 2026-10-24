# QA Checklist - SiagaRS V0.1

Dipakai untuk QA manual dan sebagai acuan test UI Playwright setelah frontend ada. Baseline PRD 23 Oktober 2026. Semua data adalah **skenario sintetis**; jangan memasukkan data pasien nyata saat pengujian.

Cara pakai: salin tabel ke catatan uji, isi `Lulus / Gagal / N/A`, catat versi build, browser dan tanggal. Kriteria "Lulus" harus teramati, bukan diasumsikan.

## 0. Prasyarat

- [ ] Backend berjalan dengan database **uji** (bukan database `siagars` utama) dan seed simulasi dimuat.
- [ ] Akun operator demo dibuat lokal; credential hanya dari environment (`OPERATOR_EMAIL`, `OPERATOR_PASSWORD`), tidak dicatat di dokumen ini.
- [ ] `go test ./tests/contract/...` lulus (uji kontrak tanpa Atlas).
- [ ] `npx playwright test --list` menampilkan spesifikasi; test UI masih `skip` sampai `SIAGARS_UI=1`.

## 1. Aksesibilitas (WCAG 2.1 AA)

| # | Kriteria | Cara uji | Hasil |
|---|---|---|---|
| A1 | Kontras teks normal >= 4.5:1, teks besar dan komponen UI >= 3:1 (1.4.3, 1.4.11) | Periksa warna badge status/kesegaran, tombol, tepi input dengan alat kontras | |
| A2 | Informasi tidak hanya lewat warna (1.4.1): `fresh/aging/stale/unknown`, `available/limited/unavailable/unknown` punya teks/ikon | Tampilan grayscale atau simulasi buta warna | |
| A3 | Semua kontrol punya nama aksesibel: input berlabel `<label>`, tombol ikon punya `aria-label` (1.3.1, 4.1.2) | Accessibility tree DevTools / `getByRole` | |
| A4 | Struktur heading berurutan (satu `h1` per halaman), landmark `main`, `nav`, `header` | Screen reader rotor / aXe | |
| A5 | Galat form terkait field (`aria-describedby`/`aria-invalid`), pesan dibacakan (3.3.1, 3.3.3); kesalahan 422 menunjuk field | Kirim form salah, dengarkan dengan NVDA/VoiceOver | |
| A6 | Perubahan dinamis diumumkan: hasil pencarian, galat, status update memakai `role="status"`/`aria-live="polite"` atau `role="alert"` (4.1.3) | Screen reader | |
| A7 | Zoom 200% dan teks 400% reflow tanpa scroll horizontal pada 320 CSS px (1.4.4, 1.4.10) | Zoom browser | |
| A8 | Target sentuh >= 24x24 CSS px (2.5.8), disarankan 44x44 pada mobile | Inspeksi ukuran | |
| A9 | Peta bukan satu-satunya jalur: semua informasi dan aksi peta tersedia di daftar (1.1.1, 2.1.1); peta punya alternatif teks | Nonaktifkan peta | |
| A10 | Tidak ada konten berkedip > 3x/detik; animasi menghormati `prefers-reduced-motion` (2.3.1, 2.3.3) | Aktifkan reduced motion di OS | |
| A11 | Bahasa halaman `lang="id"`; judul halaman unik per rute (3.1.1, 2.4.2) | Inspeksi `<html>`, tab title | |
| A12 | Alat otomatis: aXe/Lighthouse tanpa pelanggaran `critical/serious` pada halaman cari, detail, lacak, login, dashboard operator | Jalankan alat; catat sisa pelanggaran | |

## 2. Responsif (360 / 768 / 1440 px)

Ulangi pada setiap lebar: **360x740**, **768x1024**, **1440x900**. Juga uji orientasi lanskap pada 360.

| # | Kriteria | 360 | 768 | 1440 |
|---|---|---|---|---|
| R1 | Tidak ada scroll horizontal pada halaman cari, detail, lacak, login, dashboard | | | |
| R2 | Daftar RS dan peta tersusun masuk akal (mobile: daftar dan peta bisa dialihkan, bukan saling menutupi) | | | |
| R3 | Filter (layanan, BPJS, bed, wilayah) tetap terjangkau dan terbaca; tidak terpotong | | | |
| R4 | Kartu RS menampilkan nama, kota, layanan ringkas, label sumber/kesegaran tanpa terpotong; nama panjang membungkus | | | |
| R5 | Tabel/riwayat tracking dan dashboard operator terbaca (scroll dalam kontainer atau tumpuk) | | | |
| R6 | Pagination terpakai dan tidak menutupi konten; ada indikator halaman/total | | | |
| R7 | Gambar/tiles peta tidak melebihi viewport | | | |

## 3. Keyboard dan fokus

| # | Kriteria | Hasil |
|---|---|---|
| K1 | Seluruh alur (cari, filter, buka detail, lacak, login, ubah status) bisa dilakukan hanya dengan keyboard (2.1.1) | |
| K2 | Urutan Tab mengikuti urutan visual; tidak ada keyboard trap, termasuk di peta dan dialog (2.1.2, 2.4.3) | |
| K3 | Indikator fokus selalu terlihat dengan kontras cukup (2.4.7, 2.4.11) | |
| K4 | Ada skip link ke konten utama (2.4.1) | |
| K5 | Dialog/konfirmasi memindahkan fokus ke dalam, `Esc` menutup, fokus kembali ke pemicu | |
| K6 | `Enter` mengirim form pencarian dan login; `Esc` menutup panel filter mobile | |
| K7 | Setelah navigasi rute atau hasil dimuat, fokus/pengumuman berpindah wajar (tidak tertinggal di elemen hilang) | |

## 4. State kosong, galat, dan memuat

| # | Skenario | Perilaku yang diharapkan | Hasil |
|---|---|---|---|
| S1 | **Memuat**: daftar/detail/lookup lambat (throttle jaringan) | Indikator memuat terlihat dan diumumkan; tidak ada layout loncat besar; tombol kirim dinonaktifkan agar tidak dobel | |
| S2 | **Kosong**: filter tanpa hasil (`total=0`, `data=[]`) | Pesan jelas + cara melonggarkan filter; bukan layar kosong | |
| S3 | **Kosong**: RS tanpa layanan/bed (`[]`) | Teks "belum ada data", bukan nol | |
| S4 | **Null vs 0**: bed `total=null`, `available=0` | `null` tampil sebagai "tidak diketahui", `0` tampil sebagai "0"; tidak pernah tertukar | |
| S5 | **422** (parameter/validasi) | Pesan per field dalam bahasa pengguna, tanpa detail teknis | |
| S6 | **404** detail RS / kode lacak tidak dikenal | Pesan generik yang sama untuk kode salah dan tidak ada; tidak membocorkan alasan | |
| S7 | **429** lookup/login | Pesan tunggu + `Retry-After` dihormati; tombol tidak spam | |
| S8 | **401** sesi habis di dashboard operator | Diarahkan ke login dengan pesan; draf tidak hilang diam-diam | |
| S9 | **403** CSRF/RS lain | Pesan akses ditolak; tidak ada perubahan data | |
| S10 | **409** versi/transisi bentrok | Pesan "muat ulang data"; data dimuat ulang dan versi baru dipakai | |
| S11 | **500/503** atau jaringan putus | Pesan ramah + tombol coba lagi; tanpa stack trace/SQL/URL basis data | |
| S12 | **Peta gagal** (tile diblokir, geolokasi ditolak) | Pesan peta; daftar tetap berfungsi; sort jarak turun ke nama | |
| S13 | **Data basi**: `stale`/`unknown` | Peringatan terlihat; hasil tidak disajikan sebagai fakta terkini | |

## 5. Label simulasi dan sumber

| # | Kriteria | Hasil |
|---|---|---|
| L1 | Setiap RS/layanan/bed dengan `data_origin=simulation` menampilkan label "Simulasi" yang terlihat tanpa interaksi tambahan, juga ketika freshness `fresh` | |
| L2 | Label origin lain (`curated`, `satusehat_sandbox`, `satusehat_production`) dibedakan dan punya penjelasan singkat | |
| L3 | Kesegaran ditampilkan per item dengan waktu `source_updated_at` (zona Asia/Jakarta) dan `meta.as_of`; bukan `synced_at` | |
| L4 | `verified_at` hanya tampil sebagai "terverifikasi" bila terisi; `null` tidak tampil sebagai terverifikasi | |
| L5 | Sumber (`source_url`) tampil sebagai tautan hanya bila ada; `null` tidak menampilkan tautan kosong | |
| L6 | Halaman lacak menyatakan jelas "skenario sintetis, bukan rujukan nyata"; tidak ada klaim medis/diagnosis/triase | |
| L7 | Urutan "kecocokan" tidak diklaim sebagai kualitas klinis atau jaminan diterima; jarak berlabel garis lurus, bukan ETA | |
| L8 | Daftar default tidak mencampur data simulasi dengan data nyata (`dataset=real`); tampilan skenario demo mengirim `dataset=simulation` atau `all` dan memberi label | |
| L9 | Tidak ada data pasien nyata, NIK, atau identitas operator pada halaman publik (cek juga tab Network) | |

## 6. Skenario usability 60 detik

Target produk PRD: pengguna menemukan profil RS yang cocok dalam < 60 detik. **Belum terbukti**; hasil di bawah adalah bukti awal, bukan SLA. Gunakan 5 partisipan non-tim untuk tiap skenario bila memungkinkan, catat waktu dengan stopwatch dari tugas diberikan sampai pengguna menyebut jawaban yang benar, tanpa bantuan.

| # | Skenario (diberikan tanpa petunjuk antarmuka) | Jawaban benar | Batas | Waktu / kesulitan / catatan |
|---|---|---|---|---|
| U1 | "Cari RS simulasi yang punya NICU **dan** ICU di Jakarta Selatan." | RS yang tampil memenuhi kedua layanan | 60 d | |
| U2 | "Dari hasil itu, buka detail satu RS dan sebutkan apakah data bed-nya terbaru atau sudah lama." | Menyebut freshness dan bahwa data simulasi | 60 d | |
| U3 | "Anda punya kode SR-DEMO-001. Cari tahu status rujukan dan langkah berikutnya." | Menyebut `public_status` dan `next_step` | 60 d | |
| U4 | "Sebagai operator, ubah status rujukan satu langkah, lalu pastikan pelacak publik menampilkan perubahan." | Riwayat baru tampil di lacak | 60 d (setelah login) | |
| U5 | "Peta tidak termuat. Temukan RS terdekat/yang cocok lewat daftar saja." | Pengguna memakai daftar/filter tanpa peta | 60 d | |

Catat juga: berapa partisipan menemukan label simulasi tanpa diminta, dan apakah ada yang mengira data adalah fasilitas nyata (kegagalan serius).

## 7. Keamanan dan privasi yang terlihat di UI

- [ ] Cookie `siagars_session` HttpOnly, Path=/api/v1, SameSite=Lax (cek tab Application).
- [ ] Token CSRF dan session tidak muncul di URL, `localStorage`, atau log konsol.
- [ ] Kode tracking tidak masuk URL (lookup memakai body POST) dan tidak tersimpan di riwayat/analitik.
- [ ] Pesan galat login sama untuk email salah dan password salah.
- [ ] Tombol/aksi operator disembunyikan untuk non-operator **dan** server tetap menolak (diuji API: 401/403).

## 8. Pelaporan

Untuk setiap kegagalan catat: ID checklist, langkah, hasil aktual vs diharapkan, lebar viewport, browser/OS, bukti (tangkapan layar/trace Playwright). Kegagalan A2, S4, S13, L1, L6, L9 bersifat blokir rilis demo.
