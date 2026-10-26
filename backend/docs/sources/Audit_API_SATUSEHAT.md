# Audit API SATUSEHAT untuk SiagaRS

Tanggal audit: 23 Oktober 2026  
Ruang lingkup: workspace resmi SATUSEHAT PUBLIC di Postman, katalog FHIR, Master Data, dan playbook Rujukan Pasien.  
Tujuan: menentukan API mana yang layak mendukung SiagaRS tanpa mengubah MVP menjadi sistem rekam medis atau rujukan produksi.

## Ringkasan keputusan

1. **Paling relevan untuk MVP:** OAuth2 `client_credentials` dan Master Sarana Index (MSI) untuk mengambil identitas serta data dasar rumah sakit.
2. **Relevan sebagai pengayaan terkontrol:** FHIR `Organization` dan `Location` untuk struktur unit/lokasi milik organisasi yang telah terdaftar. Keduanya bukan pengganti MSI sebagai direktori nasional.
3. **Relevan untuk roadmap rujukan resmi, bukan MVP demo:** `Task`, `CarePlan`, `ServiceRequest`, `Encounter`, serta `Bundle` sesuai playbook Rujukan Pasien.
4. **Belum boleh dijadikan klaim:** ketersediaan bed real-time nasional, penerimaan pasien, kepesertaan BPJS, dan kelengkapan layanan rumah sakit. `Location.operationalStatus` dapat menyimpan status suatu bed, tetapi dokumentasi yang diaudit tidak membuktikan adanya endpoint publik nasional yang lengkap, real-time, dan boleh ditampilkan ulang untuk kebutuhan tersebut.
5. **Tidak diperlukan untuk V0.1:** data pasien, diagnosis, hasil pemeriksaan, obat, imunisasi, klaim, dan registri penyakit. Semua ini membawa data klinis/personal dan berada di luar scope SiagaRS saat ini.

## 1. Pola akses umum

| Bagian | Kegunaan |
|---|---|
| OAuth2 `POST /oauth2/v1/accesstoken?grant_type=client_credentials` | Menghasilkan bearer token server-to-server. Token dan client secret hanya boleh berada di backend. |
| FHIR R4 base URL `/fhir-r4/v1` | Endpoint transaksi resource FHIR. Operasi umumnya berupa pencarian `GET`, pembuatan `POST`, penggantian `PUT`, atau perubahan yang didukung API. |
| Master Data base URL `/masterdata` | Akses master data seperti Master Sarana Index. |
| Sandbox | Pengujian dan workshop; harus menjadi lingkungan awal SiagaRS. |
| Production | Hanya setelah partner/fasyankes terverifikasi, kredensial resmi tersedia, izin penggunaan jelas, dan uji keamanan selesai. |
| Error Response | Contoh respons validasi/autentikasi/rate-limit/server; adapter harus memetakan status dan tidak membocorkan payload sensitif. |

Token di-cache di backend sampai mendekati `expires_in`, direfresh sekali pada 401, dan tidak dibuat ulang untuk setiap request. Jangan pernah meletakkan `client_id`, `client_secret`, token, atau Organization ID privat di Next.js/browser/log.

## 2. Koleksi yang tersedia di workspace Postman

Workspace yang terlihat saat audit memuat 42 koleksi. Nomor yang tidak berurutan adalah penamaan resmi koleksi, bukan koleksi yang hilang dari daftar audit.

### Fondasi dan modul pelayanan

| Koleksi | Kegunaan |
|---|---|
| 00. FHIR Resource - Contoh Penggunaan | Contoh operasi dan payload resource FHIR individual. |
| 01. Pelayanan - Rawat Jalan | Alur resume medis pasien rawat jalan. |
| 02. Pelayanan - Rawat Inap | Alur pelayanan dan resume medis pasien yang menginap. |
| 03. Pelayanan - IGD | Registrasi, triase, asesmen, pemeriksaan, tindakan, obat, dan pemulangan IGD. |
| 04. Pelayanan - Farmasi | Peresepan, dispensing/pengeluaran obat, serta data terkait farmasi. |

### Kesehatan ibu, bayi, anak, remaja, dan imunisasi

| Koleksi | Kegunaan |
|---|---|
| 06. Antenatal Care (ANC) | Pencatatan pemeriksaan dan pemantauan kehamilan. |
| 07. Intranatal Care (INC) | Pencatatan persalinan sampai sekitar enam jam setelah melahirkan. |
| 08. Postnatal Care (PNC) | Pelayanan dan pemantauan ibu setelah persalinan. |
| 09. Neonatus | Pelayanan kesehatan bayi baru lahir. |
| 10. Skrining Hipotiroid Kongenital (SHK) | Pencatatan skrining hipotiroid pada bayi baru lahir. |
| 11. Kematian Maternal dan Perinatal | Pelaporan kematian maternal/perinatal dan data pendukungnya. |
| 12. MTBS Prioritas | Manajemen terpadu balita sakit. |
| 13. Imunisasi | Pencatatan pemberian dan riwayat imunisasi. |
| 14. Gizi | Asesmen gizi, malnutrisi, dan intervensi terkait. |
| 15. Tumbuh Kembang | Antropometri serta skrining/stimulasi perkembangan anak. |
| 16. PKPR Luar Gedung | Pemeriksaan kesehatan anak usia sekolah dan remaja di sekolah/posyandu. |
| 18. Imunisasi Covid-19 | Pencatatan imunisasi COVID-19. |
| 49. Data Kelahiran | Pelaporan data kelahiran. |

### Skrining, registri penyakit, dan layanan khusus

| Koleksi | Kegunaan |
|---|---|
| 19. Skrining PTM | Skrining penyakit tidak menular pada orang dewasa. |
| 20. Registrasi Kanker | Registri episode dan data klinis kanker. |
| 21. Simplifikasi Registrasi Jantung | Registri kasus jantung/hipertensi yang disederhanakan. |
| 22. Simplifikasi Registrasi Stroke | Registri episode dan pelayanan stroke. |
| 23. Registrasi Uronefrologi | Registri penyakit ginjal/urologi. |
| 24. Gigi | Pelayanan kesehatan gigi dan mulut. |
| 31. Registrasi Mata | Registri/pelayanan kesehatan mata. |
| 32. Registrasi Telinga | Registri/pelayanan kesehatan telinga. |
| 34. Tuberkulosis | Pelaporan dan pelayanan TB. |
| 36. HIV | Pelaporan dan pelayanan HIV. |
| 42. Geriatri | Asesmen dan pelayanan kesehatan lanjut usia. |
| 43. Upaya Berhenti Merokok | Asesmen serta intervensi berhenti merokok. |
| 46. GHPR/Rabies | Pelaporan gigitan hewan penular rabies dan penanganannya. |
| 47. Anthrax | Pelaporan dan pelayanan kasus anthrax. |
| 48. Sertifikat Medis Penyebab Kematian | Penyusunan/pelaporan penyebab kematian secara medis. |
| 53. Medical Check Up | Pencatatan rangkaian pemeriksaan kesehatan berkala. |

### Pembiayaan, rujukan, dokumen, dan teknis

| Koleksi | Kegunaan |
|---|---|
| 25. Modul Klaim Asuransi Swasta | Eligibilitas, akun, billing, klaim, invoice, dan pembayaran non-BPJS. |
| 26. Modul Klaim BPJS-K | Alur pembiayaan/klaim BPJS Kesehatan. Bukan API sederhana untuk menentukan apakah sebuah RS “menerima BPJS”. |
| 30. Rujukan Pasien | Pencarian kandidat, rekomendasi fasyankes, persetujuan/penolakan, pengiriman rujukan, dan registrasi kunjungan rujukan. |
| 51. TTE | Penandatanganan elektronik dokumen terkait layanan kesehatan. |
| Additional: Set-up DICOM Router | Konfigurasi pengiriman/citra medis DICOM. |
| Master Data API - APIGEE (v2.0) (PUBLIC) | Contoh akses master data melalui gateway API. |
| Master Data API - KPTL | Referensi Kode Pembiayaan Tindakan dan Layanan Kesehatan Nasional. |
| SATUSEHAT RME | Contoh/dukungan integrasi sistem Rekam Medis Elektronik. |

## 3. Resource FHIR dan kegunaannya

Katalog resmi saat audit mencantumkan 47 resource interoperabilitas. Koleksi contoh resource di Postman menampilkan subset yang paling umum, sedangkan resource pembiayaan dan beberapa resource klinis tambahan muncul di koleksi use case.

### Identitas, organisasi, dan kunjungan

| Resource | Kegunaan |
|---|---|
| `Patient` | Identitas/demografi pasien dan IHS pasien. **Di luar MVP SiagaRS.** |
| `Practitioner` | Identitas tenaga kesehatan dan IHS nakes. |
| `RelatedPerson` | Orang yang berhubungan dengan pasien, misalnya wali/keluarga. |
| `Organization` | Organisasi/suborganisasi seperti rumah sakit, departemen, atau unit. |
| `Location` | Lokasi fisik pelayanan: gedung, ruang, kamar, atau bed; dapat memuat tipe, koordinat, dan status operasional. |
| `Endpoint` | Alamat teknis endpoint komunikasi milik organisasi/sistem. |
| `Encounter` | Satu episode kunjungan atau interaksi pasien dengan layanan. |
| `EpisodeOfCare` | Pengelompokan rangkaian perawatan pasien dalam periode tertentu. |

### Masalah klinis, asesmen, dan hasil

| Resource | Kegunaan |
|---|---|
| `Condition` | Keluhan, masalah kesehatan, atau diagnosis. |
| `Observation` | Hasil pengukuran/pemeriksaan, misalnya tanda vital atau hasil observasi. |
| `ClinicalImpression` | Ringkasan penilaian klinis/prognosis dari evaluasi data pasien. |
| `FamilyMemberHistory` | Riwayat kesehatan keluarga. |
| `AllergyIntolerance` | Alergi atau intoleransi pasien. |
| `RiskAssessment` | Estimasi/penilaian risiko klinis. |
| `QuestionnaireResponse` | Jawaban terhadap formulir atau instrumen skrining. |
| `Goal` | Sasaran perawatan pasien. |

### Permintaan, rencana, tindakan, dan workflow

| Resource | Kegunaan |
|---|---|
| `CarePlan` | Rencana perawatan, aktivitas, dan pihak yang berkontribusi. |
| `ServiceRequest` | Permintaan layanan, pemeriksaan, tindakan, atau tindak lanjut/rujukan. |
| `Task` | Status dan pelaksana tugas/workflow; inti pencarian kandidat dan persetujuan rujukan. |
| `Procedure` | Tindakan/prosedur medis yang telah dilakukan. |
| `NutritionOrder` | Instruksi/order diet atau nutrisi. |

### Obat dan imunisasi

| Resource | Kegunaan |
|---|---|
| `Medication` | Definisi/identitas produk obat yang dipakai dalam transaksi klinis. |
| `MedicationRequest` | Resep atau permintaan obat. |
| `MedicationDispense` | Penyiapan dan penyerahan obat. |
| `MedicationAdministration` | Pemberian obat kepada pasien. |
| `MedicationStatement` | Pernyataan/riwayat obat yang digunakan pasien. |
| `Immunization` | Catatan pemberian vaksin. |

### Diagnostik, spesimen, citra, dan dokumen

| Resource | Kegunaan |
|---|---|
| `Specimen` | Sampel pemeriksaan laboratorium. |
| `Substance` | Bahan/zat yang terkait dengan spesimen atau proses klinis. |
| `DiagnosticReport` | Laporan hasil diagnostik laboratorium/radiologi. |
| `ImagingStudy` | Metadata studi pencitraan medis/DICOM. |
| `Composition` | Struktur dokumen klinis, misalnya resume medis. |
| `DocumentReference` | Metadata dan referensi ke dokumen seperti PDF atau gambar. |

### Pembiayaan dan klaim

| Resource | Kegunaan |
|---|---|
| `Coverage` | Kepesertaan/penjamin pasien. |
| `CoverageEligibilityRequest` | Permintaan pengecekan eligibilitas manfaat. |
| `CoverageEligibilityResponse` | Jawaban atas eligibilitas/manfaat yang tersedia. |
| `Account` | Akun finansial yang mengelompokkan biaya. |
| `ChargeItem` | Item biaya atas layanan/tindakan. |
| `ChargeItemDefinition` | Aturan/definisi item tarif. |
| `ChargeItemResponse` | Respons pemrosesan item biaya; dicatat sebagai non-FHIR pada playbook klaim. |
| `BillingStatus` | Status proses billing; dicatat sebagai non-FHIR pada playbook klaim. |
| `Claim` | Pengajuan klaim. |
| `ClaimResponse` | Hasil adjudikasi/pemrosesan klaim. |
| `Invoice` | Tagihan terstruktur. |
| `PaymentNotice` | Notifikasi bahwa pembayaran dikirim/dilakukan. |
| `PaymentReconciliation` | Rekonsiliasi rincian pembayaran. |

## 4. Master Data API

| API/master | Kegunaan | Relevansi ke SiagaRS |
|---|---|---|
| Master Sarana Index (MSI) | Master identitas 35 jenis fasyankes; satu endpoint daftar dapat difilter antara lain dengan `jenis_sarana`, kode SATUSEHAT, kode sarana, nama, dan wilayah. Kode jenis yang didokumentasikan: praktik mandiri 101, Puskesmas 102, klinik 103, rumah sakit 104. | **Sangat tinggi.** Sumber kandidat untuk seed/sinkronisasi direktori rumah sakit. |
| Master Wilayah | Kode dan nama provinsi, kabupaten/kota, kecamatan, kelurahan/desa yang mengikuti referensi Kemendagri. | **Tinggi.** Normalisasi filter lokasi. |
| Master Patient Index (MPI) | Pencarian/pembuatan identitas pasien dan nomor IHS pasien. | **Tidak untuk MVP.** SiagaRS tidak memproses NIK atau pasien nyata. |
| Master SDMK/Practitioner | Referensi tenaga kesehatan dan nomor IHS praktisi. | Rendah untuk MVP; diperlukan bila kelak menjadi peserta rujukan resmi. |
| KFA | Kamus produk farmasi dan alat kesehatan; mencakup detail produk, referensi BPOM/LKPP/KFA, harga JKN tertentu, dan pencarian produk/alkes. | Rendah; berguna hanya jika fitur pencarian alat/obat benar-benar ditambahkan. |
| KPTL | Standar kode pembiayaan tindakan dan layanan kesehatan. | Tidak diperlukan untuk direktori; relevan jika ada tarif/klaim. |
| Master Data Layanan | Standardisasi kode layanan. | Potensial untuk normalisasi slug layanan SiagaRS, tetapi field dan hak akses harus diuji. |
| Master Data Pembiayaan | Standardisasi data pembiayaan/tarif. | Di luar MVP. |

## 5. Alur Rujukan Pasien resmi

### Rawat jalan

1. Pra-permintaan kandidat fasyankes: `Task` dari fasyankes perujuk dan respons kriteria dari SATUSEHAT Rujukan.
2. Pencarian kandidat: `Task` pencarian dan `Task` rekomendasi fasyankes.
3. Pengiriman rujukan: `Bundle` berisi `ServiceRequest` + `CarePlan`.
4. Fasyankes tujuan mendaftarkan kunjungan dengan `Encounter`.

### Rawat inap dan rawat darurat

1. Pra-permintaan dan pencarian kandidat dengan `Task`.
2. Pengiriman tugas rujukan menggunakan `Bundle` berisi `Task` + `CarePlan`.
3. Fasyankes tujuan menerima/menolak melalui pembaruan `Task`.
4. Setelah disetujui, permintaan rujukan dikirim dengan `ServiceRequest`.
5. Kunjungan tujuan dicatat menggunakan `Encounter`.

### Implikasi untuk SiagaRS

- State machine lokal `submitted -> verified -> searching -> confirmed -> completed/rejected` boleh dipertahankan untuk demo, tetapi jangan diklaim setara dengan status resmi SATUSEHAT.
- Buat tabel pemetaan status lokal ke `Task.status`/`Task.businessStatus` hanya setelah akses sandbox dan payload versi terbaru diuji.
- Rujukan resmi melibatkan data pasien, nakes, organisasi, lokasi, diagnosis/kondisi, dan otorisasi fasyankes. Ini tidak cocok diaktifkan diam-diam pada aplikasi publik.
- Tidak ada endpoint publik SiagaRS untuk membuat rujukan produksi pada V0.1.

## 6. Pemetaan ke fitur SiagaRS

| Kebutuhan SiagaRS | Sumber yang disarankan | Catatan implementasi |
|---|---|---|
| Daftar rumah sakit | MSI `mastersarana`, filter `jenis_sarana=104` | Sinkronisasi backend read-only; simpan external ID, namespace, data sumber, `source_updated_at`, dan `synced_at`. |
| Nama/kode/wilayah/alamat dasar | MSI + Master Wilayah | Jangan menebak koordinat atau BPJS bila field tidak tersedia. |
| Struktur unit/departemen RS | `Organization` | Biasanya struktur milik organisasi, bukan direktori nasional siap pakai. |
| Gedung/ruang/kamar/bed | `Location` | Gunakan hanya bila organisasi dan izin memberi akses; status satu bed tidak sama dengan agregat ketersediaan publik. |
| Layanan seperti ICU/NICU/IGD | MSI/Organization/Location dan master layanan bila field resmi tersedia | Harus melalui pemetaan eksplisit; unsupported = `unknown`, bukan `false`. |
| Ketersediaan bed | Belum ada bukti feed publik nasional dari dokumentasi yang diaudit | Pertahankan input operator/pilot atau data simulasi berlabel. Jangan membuat timestamp “fresh” ketika sumber lama/gagal. |
| Status BPJS RS | Bukan otomatis dari koleksi klaim BPJS atau `Coverage` | Butuh sumber atribut fasilitas yang sah; `Coverage` adalah kepesertaan/penjamin pasien. |
| Pencarian kandidat rujukan | `Task` pada use case Rujukan | Roadmap; perlu akses sandbox dan aktor fasyankes resmi. |
| Permintaan rujukan | `ServiceRequest` + `CarePlan`/`Bundle` | Roadmap produksi, bukan endpoint publik MVP. |
| Persetujuan/penolakan rujukan | `Task` | Roadmap dengan audit, otorisasi, dan sinkronisasi status. |
| Registrasi kunjungan tujuan | `Encounter` | Dilakukan fasyankes tujuan dalam alur resmi. |
| Timeline demo publik | Database lokal SiagaRS | Hanya data sintetis, `Cache-Control: no-store`, token acak disimpan hash. |

## 7. Rencana integrasi bertahap

### Tahap A — MVP lomba

- `SATUSEHAT_ENABLED=false` secara default.
- Seed/kurasi 30–50 rumah sakit dan minimal 10 profil lengkap.
- Tambahkan adapter MSI read-only di backend, tetapi demo tetap berjalan tanpa upstream.
- Simpan provenance per field/kelompok data; jangan mengubah `unknown` menjadi nol/false.
- Bed dan tracking tetap simulasi atau input operator yang diberi label jelas.

### Tahap B — Sandbox

- Dapatkan akses partner/fasyankes yang sah dan kredensial sandbox.
- Implementasikan `TokenProvider` server-to-server dengan cache dan safety margin.
- Uji MSI: pagination, filter RS, perubahan schema, 401, 429, timeout, dan payload tidak lengkap.
- Uji `Organization`/`Location` read-only untuk organisasi sendiri.
- Uji playbook Rujukan dengan data sintetis dan akun resmi; belum ada write produksi.

### Tahap C — Pilot terkontrol

- Tentukan rumah sakit pilot, dasar hukum, peran data controller/processor, retention, audit, dan incident response.
- Rekonsiliasi ID lokal dengan kode SATUSEHAT/MSI; cegah duplikasi.
- Gunakan idempotency/optimistic locking untuk proses lokal dan simpan correlation/request ID.
- Bentuk mapping status rujukan resmi ↔ status UX SiagaRS dan mekanisme penanganan konflik.

### Tahap D — Produksi

- Hanya setelah review keamanan, privasi, perjanjian/izin, validasi SATUSEHAT, dan persetujuan stakeholder fasyankes.
- Pisahkan sandbox dan production secara keras; tidak ada fallback otomatis ke data nyata.
- Batasi field publik melalui allowlist DTO; jangan pernah meneruskan response FHIR mentah ke frontend.

## 8. Kontrak adapter backend yang disarankan

```go
type FacilitySource interface {
    FetchHospitals(ctx context.Context, page, limit int) (HospitalBatch, error)
}

type ReferralGateway interface {
    SearchCandidates(ctx context.Context, req CandidateRequest) (CandidateResult, error)
    GetTask(ctx context.Context, taskID string) (ReferralTask, error)
}
```

Implementasi pertama cukup `MSIFacilitySource`. `ReferralGateway` sebaiknya masih mock/sandbox dan tidak dihubungkan ke route publik. Normalisasi response upstream ke domain internal agar perubahan payload SATUSEHAT tidak langsung memecahkan frontend.

## 9. Checklist sebelum coding adapter

- [ ] Kredensial sandbox dan Organization ID resmi tersedia.
- [ ] Field MSI yang benar-benar dikembalikan sudah diuji, bukan hanya diasumsikan dari contoh.
- [ ] Lisensi/izin penayangan ulang field publik dikonfirmasi.
- [ ] Rate limit, pagination, `Retry-After`, dan masa token diuji.
- [ ] Mapping kode wilayah serta jenis sarana terdokumentasi.
- [ ] Semua data mempunyai `data_origin`, `source_updated_at`, `synced_at`, dan `verified_at` yang semantik-nya berbeda.
- [ ] Last-known-good tetap ditampilkan sebagai stale saat upstream gagal.
- [ ] Secret tidak ada di repo, frontend, screenshot, log, atau laporan lomba.
- [ ] Tes memastikan response publik tidak mengandung NIK, nomor IHS pasien, diagnosis, catatan internal, atau actor identity.
- [ ] Dokumentasi dipantau karena workspace resmi menyatakan koleksi sering diperbarui.

## 10. Sumber resmi

- [SATUSEHAT PUBLIC — Postman workspace](https://www.postman.com/satusehat/satusehat-public/overview)
- [Panduan Interoperabilitas SATUSEHAT](https://satusehat.kemkes.go.id/platform/docs/id/interoperability/)
- [Katalog resource FHIR interoperabilitas](https://satusehat.kemkes.go.id/platform/docs/id/fhir/resources/)
- [Endpoint sandbox dan production](https://satusehat.kemkes.go.id/platform/docs/id/postman-workshop/endpoint-information/)
- [Master Data SATUSEHAT](https://satusehat.kemkes.go.id/platform/docs/id/master-data/)
- [ReST API Master Sarana Index](https://satusehat.kemkes.go.id/platform/docs/id/master-data/master-sarana-index/rest-api-msi/)
- [ReST API KFA](https://satusehat.kemkes.go.id/platform/docs/id/master-data/kfa/rest-api-kfa/)
- [FHIR Organization](https://satusehat.kemkes.go.id/platform/docs/id/fhir/resources/organization/)
- [FHIR Location](https://satusehat.kemkes.go.id/platform/docs/id/fhir/resources/location/)
- [FHIR Task](https://satusehat.kemkes.go.id/platform/docs/id/fhir/resources/task/)
- [FHIR ServiceRequest](https://satusehat.kemkes.go.id/platform/docs/id/fhir/resources/service-request/)
- [Playbook Rujukan Pasien](https://satusehat.kemkes.go.id/platform/docs/id/interoperability/rujukan/)
- [KPTL](https://satusehat.kemkes.go.id/platform/docs/id/terminology/kptl/)

## Batas audit

Koleksi Postman dan portal resmi berubah berkala. Audit ini mencatat struktur yang tersedia pada tanggal di atas dan merangkum kegunaan API, bukan menyatakan bahwa project telah memperoleh akses, credential, izin produksi, atau data nasional yang lengkap. Detail mandatory field dan terminologi harus mengikuti versi playbook/use case yang dipakai ketika implementasi dilakukan.
