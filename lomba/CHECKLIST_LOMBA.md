# Checklist Lomba - SiagaRS (WIA, MIA 2026)

Disusun 24 Oktober 2026 dari empat PDF di folder GuideBook (Guidebook WIA, PPT Technical Meeting MIA 2026, Q&A MIA 2026, Mentoring Class) dan scope berjalan di `backend/PRD.md`. Aturan sudah dicocokkan ulang dengan teks PDF sebelum ditulis. Jika dokumen panitia berubah, panitia berhak memperbarui aturan dan akan mengumumkannya lewat email resmi atau website kompetisi (Guidebook, Syarat Umum); itu yang berlaku.

Catatan sumber: "Guidebook" = Guidebook WIA.pdf; "TM" = PPT Technical Meeting; "Q&A" = Q_A MIA 2026; "Mentoring" = [WIA] Mentoring Class.

## 1. Tanggal penting

| Tanggal (2026) | Kegiatan | Sumber / catatan |
|---|---|---|
| 24 Sep - 5 Okt | Pendaftaran Batch 1 | Guidebook |
| 6 - 16 Okt | Pendaftaran Batch 2 | Guidebook |
| 17 Okt | Grand Opening dan Technical Meeting | Guidebook, TM |
| 18 Okt | Learn It: mentoring awal (online) | Guidebook alur kompetisi, Q&A no. 14 |
| 19 Okt - 1 Nov | Build It: pengerjaan karya | Guidebook, TM |
| **1 Nov** | **First Draft Submission** | TM dan Guidebook bagian J; lihat ambiguitas A1 |
| 3 - 6 Nov | Review mentor | Guidebook, TM |
| 7 Nov | Laporan evaluasi (feedback mentor) | Guidebook, TM |
| 8 Nov | Sesi konsultasi terbatas | Guidebook, TM |
| 8 - 10 Nov | Fix It: revisi | Guidebook (alur kompetisi menulis 3-10 Nov, lihat A2) |
| **11 Nov** | **Final Submission (babak penyisihan)** | Guidebook, TM |
| 12 - 20 Nov | Seleksi 10 finalis | Guidebook, TM |
| 21 Nov | Pengumuman finalis | Guidebook, TM |
| 22 Nov | Mentoring public speaking | Guidebook, TM |
| 28 Nov | Final dan acara puncak (onsite) | Guidebook, TM |

Jadwal internal tim (usulan di Scope_MVP, bukan aturan panitia): QA responsif dan aksesibilitas 30 Okt; freeze draft 31 Okt.

Hari ini 24 Oktober 2026: sisa 8 hari ke draft dan 18 hari ke final.

## 2. Syarat umum (yang relevan untuk tim)

- [ ] Tim 1-3 orang, satu ketua penanggung jawab pendaftaran, komunikasi dan pengumpulan karya
- [ ] Peserta WNI dan pelajar/mahasiswa aktif, status aktif sampai 28 November 2026 (Guidebook bagian F)
- [ ] Ketua hanya boleh memimpin satu tim per cabang
- [ ] Karya orisinal, bukan plagiarisme; bantuan AI diperbolehkan selama orisinalitas ide terjaga (Guidebook bagian J)
- [ ] Karya berupa inovasi/ide baru, bukan redesign aplikasi yang sudah ada (Q&A no. 18)
- [ ] Konfirmasi kehadiran final onsite jika lolos; tidak hadir tanpa alasan sah dianggap mundur. Opsi presentasi online tersedia dengan penyesuaian bobot nilai (Q&A no. 8 dan 38)
- [ ] Misi gamifikasi (XP dan Diamond) wajib diikuti; tidak memengaruhi nilai juri final secara langsung (Q&A no. 10)

## 3. First Draft Submission (target 1 November)

Dikumpulkan lewat website MIA 2026 (mia2026.com).

**Repo publik (GitHub/GitLab) dengan README.md berisi:**
- [ ] Judul dan deskripsi singkat 1-2 kalimat
- [ ] Tech stack
- [ ] Cara menjalankan (clone, install, run)
- [ ] Progres saat ini
- Template: README_REPO_PUBLIK.md

**PDF (PowerPoint/Google Slides/Canva):**
- [ ] Maksimal 10 halaman dan maksimal 10 MB
- [ ] Nama berkas `WIA_FSDMIA2026_NamaTim` (lihat A3)
- [ ] Konsep singkat (latar belakang, tujuan, masalah)
- [ ] Sitemap
- [ ] Wireframe atau mockup sederhana tampilan utama
- [ ] Penjelasan fitur utama yang direncanakan
- [ ] Screenshot minimal 2 halaman yang sudah dikerjakan (wajib landing page + 1 halaman lain)
- Kerangka: DRAFT_SUBMISSION.md

**Hal terkait:**
- [ ] Draft tidak memengaruhi nilai akhir dan tidak mengeliminasi; tujuannya feedback mentor (Q&A no. 4, 19 dan 35). Tetap kumpulkan sesuai progres meski belum selesai
- [ ] Q&A no. 6 dan 7 menyebut draft proposal sesuai progres wajib dilampirkan; lihat A4

## 4. Final Submission (11 November)

Dikumpulkan lewat website MIA 2026.

- [ ] Link Google Drive akses publik berisi:
  - [ ] Proposal PDF maksimal 30 halaman, nama `WIA_PenyisihanMIA2026_NamaTim`, memakai template panitia (bit.ly/ProposalMIA2026 di Guidebook/TM)
  - [ ] Surat keterangan orisinal karya bermaterai Rp10.000 (template: bit.ly/SuratOrisinalitasKaryaMIA2026)
- [ ] Link video pitching YouTube pengaturan publik, maksimal 5 menit, memuat: perkenalan anggota (nama dan peran), demo fitur utama website, rencana pengembangan ke depan, deskripsi YouTube yang menarik (lihat PITCH_OUTLINE.md)
- [ ] 30 halaman dihitung dari cover sampai penutup; lampiran di luar hitungan; boleh mengembangkan isi di luar template selama poin inti panitia tersampaikan (Q&A no. 7)
- [ ] Seluruh komponen wajib, termasuk landing page, lengkap pada tahap final (Q&A no. 30)
- [ ] Figma tidak wajib untuk WIA (Q&A no. 24 dan 36)
- [ ] Website dapat dibuka di Chrome/Edge/Firefox dan dapat diakses online (Guidebook bagian K). Scope_MVP mencatat hosting dan domain belum dikonfirmasi

## 5. Rubrik babak penyisihan (100%)

Tabel Guidebook dan TM konsisten:

| Kriteria | Bobot | Yang dinilai (ringkas dari panitia) | Bukti SiagaRS yang tersedia / yang masih dibutuhkan |
|---|---|---|---|
| Orisinalitas ide dan konsep | 20% | keunikan ide dan kreativitas konsep; tidak meniru; relevan dengan tema | provenance per informasi dan tracking sintetis; **perlu** bahan pembanding "solusi sejenis" yang benar-benar diriset (belum ada di dokumen) |
| Fungsionalitas website | 25% | navigasi mudah, semua fitur berjalan baik, responsif di beberapa perangkat | API baseline berjalan lokal (smoke test); **perlu** bukti frontend, responsif 360/768/1440 px dan E2E |
| Desain dan estetika | 20% | layout, warna, tipografi harmonis; daya tarik visual dan pengalaman pengguna | **perlu** bukti dari frontend (di luar dokumen backend) |
| Konten dan informasi | 20% | informasi jelas, relevan, akurat; materi rapi dan mudah dipahami | label asal data, kesegaran, null vs 0; data nyata MSI Sandbox 516 RS Jabodetabek tanpa layanan/bed/BPJS |
| Kesesuaian tema dan subtema | 15% | sesuai tema lomba; representasi nilai edukasi, budaya atau teknologi | **perlu** keputusan subtema resmi (lihat A5) |

Total 100%. Poin tambahan menurut Q&A: backend dinilai plus (no. 34); data nyata atau mendekati nyata sangat disarankan (no. 40); satu subtema lebih disarankan (no. 31).

## 6. Rubrik babak final (tabel panitia tidak konsisten)

Tabel Guidebook dan TM: Orisinalitas dan konsep website 15%, Fungsionalitas dan user experience 25%, Desain dan kreativitas visual 20%, Presentasi dan komunikasi 20%. Jumlah 80%, sedangkan baris total tertulis 100%. Jangan dinormalisasi sendiri; lihat A6.

## 7. Babak final (28 November, jika lolos)

- [ ] Hadir 30 menit sebelum acara untuk registrasi ulang dan uji coba teknis
- [ ] Pitch deck .pptx atau .pdf maksimal 15 halaman; presentasi maksimal 10 menit, tanya jawab juri 5 menit; Indonesia atau Inggris
- [ ] Urutan wajib: Cover, Latar Belakang dan Masalah, Tujuan dan Manfaat, Solusi/Fitur Utama, Demo Website, Rencana Pengembangan, Kesimpulan (lihat PITCH_OUTLINE.md)
- [ ] Bawa laptop berisi presentasi dan prototipe siap jalan; backup presentasi .ppt/.pdf di Google Drive; prototipe dapat diakses online, siapkan alternatif offline
- [ ] Live demo sangat disarankan (Q&A no. 26 dan 29)
- [ ] Panitia tidak menanggung transportasi atau akomodasi; konsumsi hanya untuk anggota terdaftar (Q&A no. 25 dan 32)

## 8. Ambiguitas dan hal yang perlu ditanyakan ke panitia

Kontak panitia di Guidebook: WhatsApp Intan dan Nazla (nomor tertera di bagian M Guidebook; nomor pada hasil ekstraksi teks PDF terpotong, ambil dari PDF aslinya); email mia.upnvj@gmail.com (footer TM).

| No. | Ambiguitas | Fakta dari dokumen | Pertanyaan ke panitia |
|---|---|---|---|
| A1 | Tanggal draft | TM dan Guidebook bagian J: 1 November. Guidebook bagian I (alur kompetisi): kewajiban draft "2 November". Tabel timeline Guidebook mengaitkan First Draft dengan 3-6 November, tampaknya tabel bergeser | Tanggal dan **jam tutup** pengumpulan draft yang berlaku (zona WIB). Scope_MVP sudah memutuskan memakai 1 November |
| A2 | Jam penutupan final | 11 November tertulis tanpa jam; Scope_MVP mencatat jam penutupan belum diverifikasi. Rentang Fix It: tabel 8-10 Nov, alur kompetisi 3-10 Nov | Jam tutup final 11 November (WIB) dan rentang revisi yang benar |
| A3 | Penamaan berkas draft | TM: `WIA_FSDMIA2026_NamaTim`. Guidebook: `WIA_FDSMIA2025_NamaTim` (typo 2025, urutan huruf FDS vs FSD) | Format nama berkas draft yang benar, apakah "NamaTim" memakai spasi/underscore, dan apakah portal menolak nama lain |
| A4 | Draft proposal | Guidebook/TM draft hanya meminta README + PDF maksimal 10 halaman. Q&A no. 6 dan 7: draft proposal wajib dilampirkan sesuai progres. Q&A no. 11 dan 23 menuliskan "[Masukkan Link PPT]" (placeholder belum diisi) | Apakah PDF 10 halaman itu sudah dianggap draft proposal, atau perlu berkas proposal terpisah (template bit.ly/ProposalMIA2026) pada 1 November |
| A5 | Subtema | Subtema WIA di TM antara lain Public Service Tracking, Civic Reporting & Accountability, Data-Driven Decision Support; boleh lebih dari satu (TM) tetapi Q&A menyarankan satu. Teks subtema pertama terpotong pada hasil ekstraksi | Apakah studi kasus sistem rujukan/direktori RS dapat diterima di bawah Public Service Tracking dan/atau Data-Driven Decision Support; apakah subtema wajib dinyatakan di draft. Keputusan tim juga diperlukan |
| A6 | Rubrik final berjumlah 80% | Empat baris bobot (15+25+20+20) = 80%, total tertulis 100%, di Guidebook dan TM | Bobot final yang berlaku dan kriteria/baris yang hilang (kemungkinan ada kriteria kelima). Jangan mengasumsikan sendiri |
| A7 | Rubrik penyisihan | 20+25+20+20+15 = 100%, konsisten di kedua dokumen | Tidak perlu ditanyakan; hanya konfirmasi bila ada revisi |
| A8 | Typo 2025 | "WIA_FDSMIA2025" pada nama berkas dan "7 November 2025" pada feedback di Guidebook bagian J | Konfirmasi bahwa semua penyebutan 2025 itu 2026 |
| A9 | Surat orisinalitas dan materai | Bermaterai Rp10.000, format PDF/foto belum disebut | Apakah materai fisik di-scan atau materai elektronik diterima |
| A10 | Hosting | Final menyatakan prototipe harus bisa diakses online; Scope_MVP: hosting, domain dan tile peta belum dikonfirmasi | Apakah tautan lokal/tunnel diterima, atau perlu hosting publik tetap (keputusan deploy ada pada tim, bukan agen) |
| A11 | Data riil | Q&A no. 5 dan 40: dummy boleh, data nyata disarankan. Data MSI Sandbox bukan data produksi | Apakah data Sandbox SATUSEHAT dianggap "data nyata" oleh juri |

## 9. Tugas tim yang bergantung pada checklist ini

- [ ] Putuskan subtema (usulan: satu subtema; dokumen belum menetapkan)
- [ ] Putuskan satu repo atau dua repo publik (backend Go dan frontend Next.js)
- [ ] Audit repo publik: tidak ada `.env`, `atlas-credentials.env`, credential SATUSEHAT atau data pasien di berkas maupun riwayat git
- [ ] Ambil screenshot nyata (minimal landing + 1 halaman) dari aplikasi yang berjalan
- [ ] Kirim pertanyaan A1, A3, A4, A6 ke panitia sebelum 1 November; A2, A9 sebelum 11 November
- [ ] Wawancara 3-5 orang (usulan Scope_MVP) bila ingin klaim masalah didukung data; tanpa itu tetap sebut hipotesis
- [ ] Jalankan uji usability (target 4 dari 5 peserta, 60 detik) hanya jika ingin menyebut hasilnya; tanpa itu tulis "target"
- [ ] Siapkan skrip demo offline cadangan untuk final
