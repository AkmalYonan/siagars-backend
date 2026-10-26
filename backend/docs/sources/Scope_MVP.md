# SiagaRS
## Scope MVP V0.1

Tim MalasNgoding | Pre-production | Baseline 23 Oktober 2026

Temukan rumah sakit yang tepat, bukan sekadar yang terdekat.

SiagaRS membantu keluarga pasien menemukan rumah sakit berdasarkan kebutuhan fasilitas, memeriksa sumber dan umur informasi, menghubungi rumah sakit dan memantau progres rujukan simulasi. Visi jangka panjang mencakup Indonesia; V0.1 membuktikan alur lengkap menggunakan data pilot.

### 01 / Masalah dan proposisi nilai

Hipotesis masalah: informasi rumah sakit tersebar; lokasi terdekat belum tentu sesuai fasilitas; umur informasi tidak jelas; progres rujukan sulit dipantau. Pernyataan ini berasal dari brainstorming dan perlu divalidasi lewat wawancara, bukan dianggap hasil riset pengguna.

Nilai utama: pencarian berbasis kebutuhan, informasi yang dapat ditelusuri, dan timeline status yang jelas. Target awal: pengguna menemukan profil RS yang sesuai dalam kurang dari 60 detik pada skenario uji. Ini target produk, bukan janji layanan medis.

### 02 / Pengguna dan pembagian peran

| Peran | Tugas inti |
|---|---|
| Keluarga pasien | Cari, filter, cek detail, hubungi RS dan lacak kode demo |
| Operator demo | Update status layanan, bed dan progres rujukan dalam scope RS |
| Radja Admiral | Ketua; usulan pemilik scope, riset, QA, dokumentasi dan pitching |
| M. Tauriq Arya | Fullstack; fokus frontend Next.js dan integrasi kontrak API |
| Akmal Bariq Yonanda | Backend Go, database, auth, tracking dan adapter |

Prioritas teknis mengikuti pilihan tim: dua proyek Next.js dan Go. PostgreSQL menjadi rekomendasi penyimpanan. Pembagian tambahan Radja serta pilihan library masih perlu review tim.

---PAGE---
## Ruang lingkup V0.1

### 03 / Must, Should, Could dan batas versi

| Prioritas | Hasil yang harus terlihat |
|---|---|
| Must | Landing; pencarian nama/kota/provinsi; filter layanan/BPJS; peta+daftar; detail; sumber+waktu; tracking sintetis; operator minimum; UI state lengkap |
| Should | Bandingkan 2-3 RS; sort kecocokan/jarak; filter bed; berbagi profil |
| Could | Favorit lokal, geolokasi, PWA, dark mode, statistik persebaran |
| Di luar V0.1 | Diagnosis, rekam medis, NIK, booking, pembayaran, chat dokter, klaim BPJS, ambulans live dan rujukan produksi |

Dashboard operator minimum masuk Must agar demo membuktikan perubahan data dari operator sampai pengguna. Perbandingan tiga RS tetap menjadi target setelah Must stabil. Daftar fitur ini menyelesaikan perbedaan prioritas dalam percakapan sebelumnya.

### 04 / Cakupan data yang jujur

Target kurasi 30-50 rumah sakit lintas wilayah, dengan minimal 10 profil lengkap untuk skenario demo. Jabodetabek dapat menjadi pilot mendalam; pilihan wilayah final menunggu dataset yang berhasil diverifikasi. Bila target belum tercapai, tampilkan jumlah aktual.

Pisahkan empat atribut: asal data, waktu pembaruan sumber, waktu sinkronisasi dan waktu verifikasi. Data simulasi tidak menjadi faktual karena timestamp baru. Nilai yang belum diketahui tidak boleh ditampilkan sebagai nol atau tidak tersedia.

Asal data: curated, simulation, satusehat_sandbox, satusehat_production. Integrasi produksi hanya setelah akses dan izin relevan. Data rumah sakit publik tidak berarti seluruh field klinis dalam API boleh dipublikasikan.

### 05 / Indikator keberhasilan

- Usability: minimal 4 dari 5 peserta uji dapat menyelesaikan pencarian fasilitas pada pilot dalam <=60 detik tanpa bantuan. Rekrut pengguna nyata bila memungkinkan; catat keterbatasan sampel.
- Fungsional: search -> detail -> kontak/rute dan login operator -> update -> tracking berjalan utuh.
- Transparansi: semua status/bed memiliki sumber atau label simulasi; unknown dan stale terlihat.
- Kualitas: mobile 360 px, tablet 768 px, desktop 1440 px; keyboard; tidak ada CTA inti mati; fallback daftar saat peta gagal.
- Reproduksibilitas: demo seed dapat dijalankan lokal tanpa akses SATUSEHAT; map online membutuhkan tile provider sehingga fallback daftar wajib.

---PAGE---
## Perjalanan dan bukti demo

### 06 / Alur utama

Cari lokasi -> pilih layanan -> hasil peta/daftar -> detail sumber dan status -> hubungi RS atau buka navigasi.

Lacak rujukan: masukkan kode sintetis -> status saat ini -> riwayat waktu -> langkah berikutnya. Seluruh halaman tracking berlabel simulasi dan tidak menyatakan rujukan medis resmi telah diproses.

Operator: login -> daftar rujukan milik RS -> ubah status sesuai transisi -> simpan -> pengguna refresh dan melihat event baru. Operator juga dapat mengubah layanan/bed pada RS yang ditugaskan.

### 07 / Cerita demo yang konsisten

Rina mencari RS dengan NICU dan BPJS di wilayah pilot. Ia melihat beberapa hasil dan membedakan data yang diketahui, stale dan simulasi. Pada detail ia memeriksa kontak lalu membuka rute. Berikutnya ia memasukkan kode demo. Operator memperbarui tahap searching menjadi confirmed; halaman tracking memperlihatkan riwayat baru setelah refresh.

Cerita ini menguji alur informasi dan tracking. Tersedianya bed dalam prototipe tidak menjamin penerimaan pasien. Cocok fasilitas berarti sesuai kriteria pencarian yang tercatat, bukan rekomendasi kualitas klinis.

### 08 / Struktur halaman

- `/`: beranda, manfaat, cakupan pilot dan CTA.
- `/rumah-sakit`: query dan filter, peta/daftar, pagination.
- `/rumah-sakit/[id]`: profil, fasilitas, bed, sumber, telepon/rute.
- `/lacak-rujukan`: form kode dan timeline sintetis.
- `/tentang-data`: cakupan, provenance, kesegaran dan keterbatasan.
- `/operator/login`, `/operator/rujukan`, `/operator/layanan`: dashboard minimum.
- `/bandingkan`: 2-3 RS, prioritas Should.

### 09 / Prinsip pengalaman

Pencarian publik tanpa login. Mobile menjadi prioritas. Status memakai teks/ikon selain warna. Waktu ditampilkan dengan zona WIB. Peta selalu memiliki alternatif daftar. Hasil kosong menawarkan reset filter; request gagal menawarkan retry. Sumber dan waktu ditempatkan dekat informasi yang dirujuk.

---PAGE---
## Rencana kerja dan kepatuhan lomba

### 10 / Milestone yang dapat dijalankan

| Waktu | Deliverable |
|---|---|
| 23-24 Oktober | Scope, wireframe, schema dan kontrak API; audit sumber data |
| 25-26 Oktober | Landing/search/detail; API publik, migrations dan seed |
| 27-29 Oktober | Tracking, auth/operator dan integrasi frontend-backend |
| 30 Oktober | QA responsif, aksesibilitas, galat dan alur demo |
| 31 Oktober | Freeze draft; README, screenshot dan PDF pengumpulan |
| 1 November | First Draft Submission |
| 3-7 November | Review mentor 3-6; hasil feedback 7 November |
| 8-11 November | Revisi 8-10; final submission 11 November |

Finalis diumumkan 21 November, sesi public speaking 22 November, final 28 November 2026 menurut dokumen. Jam penutupan belum diverifikasi. Gunakan 1 November untuk draft karena timeline dan technical meeting konsisten, walau satu paragraf guidebook menulis 2 November.

### 11 / Checklist artefak lomba

Draft: repo publik dan README (judul/deskripsi, stack, cara jalan, progres); PDF maksimal 10 halaman dan 10 MB, konsep, sitemap, wireframe/mockup, fitur, screenshot landing+minimal satu halaman lain. Materi technical meeting menggunakan nama WIA_FSDMIA2026_NamaTim; guidebook memiliki typo 2025, konfirmasi bila portal berbeda.

Final submission: proposal PDF maksimal 30 halaman, surat orisinalitas dan video pitching publik YouTube maksimal lima menit sesuai ketentuan. Q&A menjelaskan 30 halaman cover sampai penutup, lampiran terpisah. Final presentasi maksimal 15 halaman, 10 menit + tanya jawab lima menit. Paket pre-production ini merupakan bahan internal, bukan pengganti berkas resmi tersebut.

Rubrik penyisihan: orisinalitas 20%, fungsi 25%, desain 20%, konten 20%, kesesuaian tema 15%. Tabel final dalam dokumen yang diberikan mencantumkan empat bobot berjumlah 80% tetapi total tertulis 100%; minta klarifikasi, jangan menormalisasi sendiri.

### 12 / Risiko dan keputusan terbuka

Akses SATUSEHAT, hosting, domain, tile provider, dataset pilot dan kapasitas jam kerja belum dikonfirmasi. Tindakan awal: Radja validasi kebutuhan dengan 3-5 wawancara; Akmal audit akses/field master data tanpa menjadi dependency; Tauriq siapkan wireframe dan fixture kontrak. Tunda Could lalu Should bila waktu kurang.

### Referensi internal

GuideBook/Guidebook WIA.pdf (19 halaman); PPT Technical Meeting MIA 2026.pdf (40); Q_A MIA 2026 - Sheet2.pdf (5); [WIA] Mentoring Class MIA 2026.pdf (33). Dasar: aturan pengumpulan, rubrik, frontend minimum, dummy data diizinkan, fokus subtema, user journey/sitemap dan desain database. Interpretasi produk dan target uji adalah usulan Tim MalasNgoding, bukan aturan panitia.
