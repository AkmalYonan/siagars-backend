# Outline Pitch - SiagaRS (video penyisihan dan presentasi final)

Dua materi berbeda: (A) video pitching YouTube maksimal 5 menit untuk final submission 11 November, dan (B) pitch deck maksimal 15 halaman untuk babak final onsite 28 November (jika lolos). Aturan dari Guidebook bagian J dan K serta PPT Technical Meeting; isi dari backend/PRD.md, README.md, DECISIONS.md, CHANGELOG.md dan sources/Scope_MVP.md (status diperbarui 26 Oktober 2026).

Tanda: **[ISI]** diisi tim; **[BELUM TERBUKTI]** jangan diucapkan/ditulis sebagai fakta sebelum ada bukti; **[PLACEHOLDER]** ganti dengan tangkapan layar atau materi nyata.

## Daftar klaim yang aman dan yang belum terbukti

Aman (ada di dokumen/kode, sebut sumbernya bila ditanya):

- API Go + MongoDB Atlas berjalan lokal dengan seed sintetis; smoke test HTTP lokal lulus; unit test dan suite integrasi (database tes sementara) lulus pada 23 Oktober 2026
- Filter layanan memakai AND; null berbeda dari nol; setiap informasi punya asal data, waktu sumber, waktu sinkronisasi dan waktu verifikasi sendiri; kesegaran dihitung dari waktu sumber
- State machine rujukan sintetis, optimistic locking, auth operator dengan CSRF dan scope RS
- Adapter SATUSEHAT MSI Sandbox read-only; live Sandbox Jabodetabek 23 Oktober 2026: 516 RS dibuat, 0 skip/gagal, run kedua 516 unchanged. Sebut sebagai data Sandbox
- Layanan/bed/BPJS pada RS nyata adalah data contoh berlabel `simulation`, bukan fakta RS
- Backend online di Vercel (`https://siagars-backend.vercel.app`, dokumentasi `/docs`); 25 Oktober 2026: 32 pemeriksaan publik dan 15 pemeriksaan login/mutasi operator dari origin FE lulus di production
- Cek BPJS **simulasi** dengan peserta sintetis (API 0.3.0, staging); input dicatat tersamar + terenkripsi, dihapus otomatis 30 hari

Belum terbukti (jangan diklaim): RS nyata terkurasi dengan layanan/bed/BPJS lengkap; pengguna menemukan RS <60 detik; hasil riset atau wawancara pengguna; performa (p95); Docker; E2E UI frontend-backend; suite `qa/` terhadap production; cek BPJS di production (sebelum dirilis); cek ke BPJS Kesehatan sungguhan; integrasi produksi SATUSEHAT, FHIR atau rujukan resmi; dampak ke pasien; "lebih baik dari solusi sejenis" tanpa riset pembanding; jumlah pengguna; data kesembuhan/waktu tunggu apa pun.

---

## A. Video pitching (maksimal 5 menit, YouTube publik)

Wajib memuat (panitia): perkenalan anggota (nama dan peran), demo fitur utama website, rencana pengembangan ke depan, deskripsi YouTube menarik.

| Waktu | Bagian | Isi | Catatan |
|---|---|---|---|
| 0:00-0:20 | Pembuka dan masalah | nama produk, tagline "Temukan rumah sakit yang tepat, bukan sekadar yang terdekat", satu kalimat hipotesis masalah | tulis sebagai hipotesis (Scope_MVP bagian 01); tanpa angka statistik kecuali ada sumbernya **[ISI sumber jika ada]** |
| 0:20-0:50 | Perkenalan tim | Radja Admiral (ketua; scope, riset, QA, dokumentasi, pitching), M. Tauriq Arya (frontend Next.js dan integrasi), Akmal Bariq Yonanda (backend Go, database, auth, tracking, adapter) | peran dari Scope_MVP; pembagian Radja adalah usulan, konfirmasi dengan tim |
| 0:50-2:50 | Demo fitur utama | (1) cari RS dengan filter layanan dan BPJS, lihat peta/daftar; (2) buka detail: asal data, waktu sumber, kesegaran, null vs 0; (3) lacak kode demo SR-DEMO-001 dan lihat timeline; (4) cek BPJS simulasi dengan peserta demo `9999...` (opsional, bila sudah dirilis); (5) login operator, ubah status rujukan, refresh pelacakan dan lihat event baru | rekam dari aplikasi nyata; tampilkan label simulasi; jangan menampilkan secret atau email nyata; kode SR-DEMO-001 dan NIK `9999...` adalah data publik sintetis; jangan memasukkan NIK asli |
| 2:50-3:30 | Data dan kejujuran sumber | 30 RS seed fiktif berlabel simulation; data MSI Sandbox 516 RS Jabodetabek read-only dengan layanan/bed/BPJS contoh berlabel simulasi; tidak ada data pasien | sebut "Sandbox", bukan produksi |
| 3:30-4:20 | Teknis singkat (nilai plus backend) | satu service Go di Vercel, MongoDB Atlas, OpenAPI + Scalar, optimistic locking, CSRF, adapter terpisah, enkripsi riwayat cek BPJS | Q&A panitia no. 34: backend adalah poin plus; tidak menjamin nilai |
| 4:20-4:50 | Rencana pengembangan | kurasi 30-50 RS dengan minimal 10 profil lengkap (target Scope_MVP, belum tercapai); uji usability dengan pengguna nyata; optimasi lanjutan berdasarkan benchmark; akses SATUSEHAT yang sah bila tersedia | query pushdown dan benchmark awal sudah tersedia; tandai rencana lanjutan sebagai rencana; ide AI discovery 500 RS belum masuk kontrak v0.1 (DECISIONS.md) |
| 4:50-5:00 | Penutup | nama produk, tautan repo/demo **[ISI jika sudah ada]** | total maksimal 5:00; targetkan 4:45 agar aman |

Persiapan:
- [ ] Skrip tertulis dan latihan dengan stopwatch; durasi akhir tidak lebih dari 5:00
- [ ] Rekam layar 1080p dari aplikasi yang berjalan; cadangkan rekaman demo bila server gagal
- [ ] Anggota tim hadir dan menyebut nama serta peran sendiri
- [ ] Upload ke YouTube dengan visibilitas Publik; cek dari akun lain atau jendela privat
- [ ] Judul berisi SiagaRS dan nama tim; deskripsi YouTube: ringkasan masalah, fitur utama, daftar anggota dan peran, tautan repo, catatan "data simulasi dan sandbox, bukan layanan medis", **tanpa secret**
- [ ] Tidak memakai musik atau gambar yang hak ciptanya tidak jelas
- [ ] Tautan video dimasukkan ke formulir final submission 11 November

## B. Presentasi final (maksimal 15 halaman, 10 menit + tanya jawab 5 menit)

Format .pptx atau .pdf; Indonesia atau Inggris. Urutan wajib dari panitia: Cover, Latar Belakang dan Masalah, Tujuan dan Manfaat, Solusi/Fitur Utama, Demo Website, Rencana Pengembangan, Kesimpulan. Kriteria final menurut Guidebook/TM: orisinalitas dan konsep 15%, fungsionalitas dan UX 25%, desain dan kreativitas visual 20%, presentasi dan komunikasi 20% (jumlah 80%, belum jelas; lihat CHECKLIST_LOMBA.md A6). Presentasi utama oleh ketua atau perwakilan; anggota lain boleh menjawab tanya jawab; juri menilai kontribusi tiap anggota.

| Slide | Bagian panitia | Isi | Menit | Pembicara **[ISI]** |
|---|---|---|---|---|
| 1 | Cover | nama tim, nama anggota, nama produk SiagaRS, tagline | 0:15 | |
| 2 | Latar belakang dan masalah | fakta atau data relevan dari studi kasus **[ISI: sumber resmi; belum ada angka di dokumen]**; hipotesis masalah | 0:45 | |
| 3 | Latar belakang dan masalah | 2-3 masalah inti dari sisi pengguna dan operator (info tersebar; fasilitas tidak cocok dengan jarak; umur info dan progres rujukan tidak jelas) | 0:30 | |
| 4 | Tujuan dan manfaat | tujuan, manfaat bagi keluarga pasien, operator RS dan pembuat kebijakan **[ISI: hanya manfaat yang bisa dijelaskan secara konkret]** | 0:30 | |
| 5 | Solusi/fitur utama | gambaran produk, alur utama dan diagram arsitektur | 0:30 | |
| 6 | Solusi/fitur utama | pencarian berbasis kebutuhan (layanan AND, BPJS, wilayah, jarak) | 0:30 | |
| 7 | Solusi/fitur utama | transparansi sumber: empat atribut data, kesegaran, null vs 0, label simulasi | 0:30 | |
| 8 | Solusi/fitur utama | tracking rujukan sintetis dan dashboard operator, state machine | 0:30 | |
| 9 | Solusi/fitur utama | keunggulan dibanding solusi sejenis **[BELUM TERBUKTI: perlu riset pembanding nyata sebelum slide ini dibuat; jika tidak ada, ganti dengan "pembeda desain"]** | 0:30 | |
| 10 | Demo website | tautan demo online dan alur yang akan didemokan; **live demo** dianjurkan panitia | 2:30 | |
| 11 | Demo website (cadangan) | tangkapan layar tiap langkah dan rekaman offline bila jaringan gagal | (cadangan) | |
| 12 | Teknis dan data | stack, kontrak OpenAPI (22 operasi), data seed vs MSI Sandbox 516 RS vs data contoh berlabel, batasan | 0:30 | |
| 13 | Rencana pengembangan | roadmap: kurasi data, usability testing, performa, akses SATUSEHAT sah, tahap berikutnya | 0:30 | |
| 14 | Kesimpulan | value proposition dan alasan layak dipilih | 0:30 | |
| 15 | Penutup / terima kasih | tautan repo, kontak tim, QR demo | 0:15 | |

Slide 11 dan 15 tidak memakai jatah menit utama. Jumlah menit pada tabel 8:45, menyisakan sekitar 1:15 sebagai cadangan di bawah batas 10 menit; ini alokasi usulan, belum dilatih. Jumlah halaman tepat 15 sesuai batas; hapus slide 9 atau 11 bila ingin ruang.

Persiapan:
- [ ] Laptop pribadi berisi presentasi dan prototipe siap jalan; backup presentasi .ppt/.pdf di Google Drive
- [ ] Prototipe dapat diakses online dan cadangan offline (backend lokal + MongoDB Atlas butuh internet; **siapkan skenario bila Atlas tidak terjangkau**, mis. rekaman demo)
- [ ] Browser Chrome/Edge/Firefox diuji
- [ ] Login operator demo dibuat sendiri sebelum acara; jangan menaruh password di slide
- [ ] Latihan tiga kali dengan stopwatch; sisakan buffer
- [ ] Sesi tanya jawab: siapkan jawaban untuk pertanyaan di bawah

## Pertanyaan juri yang mungkin muncul (siapkan jawaban jujur)

| Pertanyaan | Jawaban yang didukung dokumen |
|---|---|
| Dari mana data RS Anda? | 30 RS seed fiktif berlabel simulation; 516 RS dari SATUSEHAT MSI Sandbox (read-only); layanan/bed/BPJS RS tersebut adalah data contoh berlabel simulasi karena MSI tidak menyediakannya; pipeline kurasi bersumber ada, datanya belum |
| Apakah ketersediaan bed akurat? | Tidak dijamin; setiap nilai membawa asal, waktu sumber dan kesegaran; bed simulasi tidak dianggap bukti nyata; konfirmasi lewat kontak RS |
| Apakah ini rujukan resmi? | Bukan. State machine lokal untuk demo, tidak mengirim apa pun ke SATUSEHAT; rujukan resmi memakai Task/ServiceRequest/CarePlan/Encounter dan tidak diimplementasikan |
| Bagaimana privasi data pasien? | Tidak ada data pasien, rekam medis atau diagnosis; kode tracking dan token session disimpan sebagai hash. Cek BPJS simulasi mencatat input untuk riwayat operator: tersamar, data asli terenkripsi AES-256-GCM, dibuka hanya dengan password ulang operator dan diaudit, dihapus otomatis 30 hari; pengguna diminta memakai data demo |
| Apakah cek BPJS-nya sungguhan? | Bukan. Tidak ada API publik BPJS untuk cek kepesertaan; cek resmi butuh kerja sama (VClaim). SiagaRS mencocokkan ke peserta sintetis (NIK `9999...`) dan selalu berlabel simulasi |
| Apakah sudah diuji pengguna? | Belum; target <60 detik adalah target, uji usability belum dilakukan **[ISI bila sudah]** |
| Apakah skalabel? | Backend berjalan serverless di Vercel; rate limit masih per instance. Query dasar, filter, pengurutan nama, dan paginasi sudah dijalankan di MongoDB serta memiliki benchmark awal. `sort=distance|match` masih mengurutkan hasil tersaring di aplikasi; angka benchmark bukan SLA produksi. |
| Mengapa bukan sekadar peta RS terdekat? | Pencarian berdasarkan kebutuhan fasilitas dan transparansi umur informasi (Scope_MVP) |
| Apa kontribusi tiap anggota? | Radja: scope/riset/QA/dokumentasi/pitching; Tauriq: frontend dan integrasi; Akmal: backend. **[ISI: sesuaikan dengan kenyataan]** |
