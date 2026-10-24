# Handoff Claude Code: Integrasi SATUSEHAT MSI ke Backend Go + MongoDB

Tanggal: 23 Oktober 2026  
Project target: `output/SiagaRS_Backend_v0.1/backend`  
Status: siap diimplementasikan, dimulai dari adapter read-only SATUSEHAT Sandbox.

## Instruksi utama untuk Claude Code

Implementasikan integrasi SATUSEHAT **hanya pada repository MongoDB ini**. Jangan mengubah folder `backend` di root workspace karena folder tersebut masih salinan lama berbasis PostgreSQL.

Sebelum mengubah kode:

1. Baca seluruh `AGENTS.md`.
2. Baca `README.md`, `docs/DECISIONS.md`, `PRD.md`, `docs/openapi.yaml`, dan `.env.example`.
3. Periksa kode aktual pada:
   - `internal/config/config.go`
   - `internal/domain/model.go`
   - `internal/repository/mongo.go`
   - `internal/integrations/satusehat/source.go`
   - `cmd/manage/main.go`
   - `migrations/*.json`
   - `seed/seed.go`
   - `tests/integration/mongo_test.go`
4. Pertahankan seluruh perubahan MongoDB dan pekerjaan pengguna yang sudah ada.
5. Jangan mengakses atau mencetak isi `atlas-credentials.env`. Jangan menyalin credential ke source, dokumentasi, test fixture, command output, atau log.

## Sasaran implementasi

Tambahkan sinkronisasi read-only rumah sakit dari SATUSEHAT Master Sarana Index (MSI) Sandbox ke collection MongoDB `hospitals`.

Alur yang diinginkan:

```text
cmd/manage sync-satusehat
        |
        v
OAuth2 TokenProvider -----> SATUSEHAT Sandbox
        |                         |
        |                         v
        +------------------- MSI pages
                                  |
                                  v
                         validate + normalize
                                  |
                                  v
                    MongoDB regions + hospitals
                                  |
                                  v
                    API publik membaca MongoDB lokal
```

API publik SiagaRS tidak boleh bergantung langsung pada uptime SATUSEHAT. Endpoint `/api/v1/hospitals` dan detail rumah sakit tetap membaca MongoDB. Kegagalan upstream tidak boleh menghapus last-known-good data.

## Scope fase ini

### Wajib

- OAuth2 `client_credentials` untuk SATUSEHAT Sandbox.
- Cache token in-memory berdasarkan `expires_in` dengan safety margin.
- HTTP client MSI bertipe dan teruji.
- Pagination seluruh rumah sakit dengan `jenis_sarana=104`.
- Mapping response MSI ke dokumen internal.
- Upsert provinsi, kabupaten/kota, dan rumah sakit ke MongoDB.
- Perintah `go run ./cmd/manage sync-satusehat`.
- Summary sinkronisasi tanpa data sensitif.
- Unit test menggunakan `httptest.Server`.
- Integration test MongoDB untuk idempotency dan perlindungan data embedded.
- README, `.env.example`, keputusan teknis, dan test command diperbarui.

### Tidak termasuk

- Rujukan resmi SATUSEHAT.
- `POST`, `PUT`, atau `PATCH` resource FHIR.
- Data pasien, NIK, nomor IHS pasien, diagnosis, nakes, obat, klaim, atau rekam medis.
- Sinkronisasi bed, ICU, NICU, PICU, BPJS, atau layanan rumah sakit dari MSI.
- Integrasi production.
- Endpoint publik/admin baru untuk memicu sinkronisasi.
- Scheduler atau queue baru. Fase awal menggunakan command manual.
- Perubahan kontrak HTTP publik apabila tidak diperlukan.

## Endpoint resmi yang digunakan

### OAuth2 Sandbox

```http
POST https://api-satusehat-stg.dto.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials
Content-Type: application/x-www-form-urlencoded

client_id=<SATUSEHAT_CLIENT_ID>
client_secret=<SATUSEHAT_CLIENT_SECRET>
```

Token hanya berada di backend memory. Jangan menyimpan token ke MongoDB.

### Master Sarana Index Sandbox

```http
GET https://api-satusehat-stg.dto.kemkes.go.id/masterdata/v1/mastersaranaindex/mastersarana?jenis_sarana=104&status_aktif=true&page=1&limit=200
Authorization: Bearer <access_token>
Accept: application/json
```

Dokumentasi resmi:

- https://satusehat.kemkes.go.id/platform/docs/id/master-data/master-sarana-index/rest-api-msi/
- https://satusehat.kemkes.go.id/platform/docs/id/postman-workshop/endpoint-information/
- https://www.postman.com/satusehat/satusehat-public/overview

Jangan menebak behavior yang tidak didokumentasikan. Jika payload sandbox berbeda dari dokumentasi, simpan contoh yang sudah disanitasi sebagai fixture test dan sesuaikan DTO secara defensif.

## Struktur kode target

Gunakan atau sesuaikan struktur berikut tanpa membuat abstraction berlebihan:

```text
internal/integrations/satusehat/
├── source.go          # interface yang sudah ada
├── errors.go          # error upstream bertipe
├── token.go           # OAuth TokenProvider
├── msi_types.go       # DTO JSON response MSI
├── msi_client.go      # FetchPage + HTTP policy
└── mapper.go          # MSI DTO -> tipe import internal

internal/service/
└── satusehat_sync.go  # loop pagination + summary

internal/repository/
├── mongo.go           # pertahankan repository utama
└── satusehat_import.go
```

Nama file boleh disesuaikan dengan gaya codebase, tetapi boundary harus tetap:

- integration package mengerti HTTP/payload SATUSEHAT;
- service mengatur pagination dan workflow sinkronisasi;
- repository mengerti BSON/upsert MongoDB;
- HTTP handler publik tidak mengetahui payload MSI.

## Konfigurasi

Saat ini `config.Load()` sengaja menolak `SATUSEHAT_ENABLED=true`. Ubah dengan tetap mempertahankan default aman.

Konfigurasi yang diperlukan:

```env
SATUSEHAT_ENABLED=false
SATUSEHAT_ENV=sandbox
SATUSEHAT_CLIENT_ID=
SATUSEHAT_CLIENT_SECRET=
SATUSEHAT_ORGANIZATION_ID=
SATUSEHAT_PAGE_LIMIT=200
SATUSEHAT_HTTP_TIMEOUT_SECONDS=10
```

Aturan:

- Default tetap `SATUSEHAT_ENABLED=false`.
- Ketika disabled, API dan seluruh fitur seed/demo tetap bekerja.
- `SATUSEHAT_ENV` fase ini hanya menerima `sandbox`.
- `production` harus ditolak dengan error eksplisit.
- Bila enabled, client ID dan client secret wajib non-empty.
- Organization ID belum digunakan untuk MSI; jangan mengirimkannya jika endpoint tidak memerlukan.
- Jangan menyediakan base URL production melalui default tersembunyi.
- Untuk test, base URL di-inject melalui constructor client, bukan environment production.
- Jangan menampilkan nilai credential di pesan validasi.

Pertimbangkan memisahkan konfigurasi menjadi:

```go
type SATUSEHATConfig struct {
    Enabled     bool
    Environment string
    ClientID    string
    ClientSecret string
    PageLimit   int
    HTTPTimeout time.Duration
}
```

API server tidak perlu otomatis menyinkronkan saat startup. Command `manage sync-satusehat` yang membuat client dan menjalankan importer.

## TokenProvider

Implementasikan token provider yang concurrency-safe:

- Simpan access token dan expiry di memory.
- Gunakan `expires_in` dari response.
- Safety margin minimal 60 detik atau proporsional jika expiry pendek.
- Gunakan `singleflight.Group` agar request bersamaan tidak membuat banyak token.
- Jangan log form body, token, client ID, atau secret.
- Batasi ukuran response OAuth.
- Validasi status HTTP, content type yang masuk akal, JSON, token non-empty, dan `expires_in > 0`.
- Pada response MSI 401, invalidate token lalu retry tepat satu kali.
- Jangan retry 400/403.

Dependency `golang.org/x/sync` sudah tersedia; gunakan `singleflight` dari dependency yang ada.

## DTO MSI minimum

Response terdokumentasi:

```go
type MSIResponse struct {
    StatusCode int           `json:"status_code"`
    Message    string        `json:"message"`
    Page       int           `json:"page"`
    TotalPage  int           `json:"total_page"`
    Data       []MSIFacility `json:"data"`
}

type MSIRegion struct {
    Code string `json:"kode"`
    Name string `json:"nama"`
}

type MSICodedValue struct {
    Code string `json:"kode"`
    Name string `json:"nama"`
}

type MSIFacility struct {
    SATUSEHATCode string  `json:"kode_satusehat"`
    FacilityCode  string  `json:"kode_sarana"`
    Name          string  `json:"nama"`
    Phone         *string `json:"telp"`
    Address       *string `json:"alamat"`
    Longitude     *string `json:"longitude"`
    Latitude      *string `json:"latitude"`
    Operational   bool    `json:"operasional"`
    FacilityStatus string  `json:"status_sarana"`
    Active        bool    `json:"status_aktif"`
    Province      MSIRegion `json:"provinsi"`
    City          MSIRegion `json:"kabkota"`
    FacilityType  MSICodedValue `json:"jenis_sarana"`
    Subtype       MSICodedValue `json:"subjenis"`
    FacilityClass MSICodedValue `json:"kelas_sarana"`
}
```

Gunakan pointer/custom nullable type jika sandbox ternyata mengirim `null` pada object atau boolean yang didokumentasikan wajib. Decoder tidak boleh panic.

## Normalisasi dan mapping

Mapping minimum:

| MSI | MongoDB `hospitals` |
|---|---|
| `kode_satusehat` | `external_id` |
| lingkungan | `source_namespace = satusehat:msi:sandbox` |
| `nama` | `name` |
| `alamat` | `address` |
| `provinsi.kode` | `province_code` |
| `kabkota.kode` | `city_code` |
| `latitude` | `latitude` nullable double |
| `longitude` | `longitude` nullable double |
| `telp` | `phone` nullable |
| waktu import | `synced_at` |
| sandbox | `data_origin = satusehat_sandbox` |

Aturan penting:

- Trim string; string kosong menjadi `nil` untuk field nullable.
- Parse latitude dan longitude bersama-sama. Jika salah satu kosong/invalid/out of range, keduanya `nil`.
- Hanya terima `jenis_sarana.kode == "104"`.
- Record tanpa `kode_satusehat`, nama, provinsi, atau kabupaten/kota di-skip dan dihitung dalam summary.
- `bpjs = nil`. MSI tidak membuktikan status penerimaan BPJS.
- `services` dan `beds` jangan diisi dari MSI.
- `source_updated_at = nil` karena response MSI yang didokumentasikan tidak memberi timestamp perubahan per record.
- `verified_at = nil`; `status_sarana=valid` bukan verifikasi manual SiagaRS.
- `synced_at` hanya menunjukkan waktu sistem mengambil data, bukan umur sumber.
- Karena `source_updated_at=nil`, freshness publik harus tetap `unknown`.
- Jangan membuat URL sumber yang mengandung token atau credential.

## Model MongoDB dan migration

Collection `hospitals` sudah memiliki unique partial index:

```text
(source_namespace, external_id)
```

Gunakan index tersebut sebagai kunci idempotency.

Tambahkan migration bernomor baru; jangan mengedit `001_initial.json` yang sudah diterapkan. Field internal yang disarankan:

```text
source_active       bool optional
source_status       string optional
facility_code       string optional
facility_type_code  string optional
facility_subtype    string optional
facility_class      string optional
last_seen_at        date optional
```

Tujuan `source_active` adalah mencegah fasilitas yang sudah tidak aktif terus tampil selamanya. Untuk dokumen simulation/curated lama yang tidak memiliki field tersebut, perlakukan sebagai aktif.

Update pembacaan rumah sakit agar mengecualikan hanya dokumen dengan `source_active=false`, misalnya secara semantik:

```go
bson.D{{Key: "source_active", Value: bson.D{{Key: "$ne", Value: false}}}}
```

Jangan memerlukan migration destructive atau backfill massal bila filter kompatibel dengan dokumen lama.

## Upsert MongoDB

Tambahkan repository method khusus, misalnya:

```go
type HospitalImport struct {
    ID                string
    ExternalID        string
    SourceNamespace   string
    Name              string
    Address           *string
    ProvinceCode      string
    ProvinceName      string
    CityCode          string
    CityName          string
    Latitude          *float64
    Longitude         *float64
    Phone             *string
    SourceURL         *string
    SourceActive      bool
    SourceStatus      string
    FacilityCode      string
    FacilityTypeCode  string
    FacilitySubtype   string
    FacilityClass     string
    SyncedAt          time.Time
}

type HospitalImporter interface {
    UpsertSATUSEHATHospitals(context.Context, []HospitalImport) (ImportResult, error)
}
```

Untuk setiap batch:

1. Upsert province ke `regions` dengan `kind=province`.
2. Upsert city ke `regions` dengan `kind=city` dan `parent_code` province.
3. Bulk upsert hospitals berdasarkan `source_namespace + external_id`.

Gunakan `$set` hanya untuk field milik MSI:

- nama;
- alamat;
- region;
- koordinat;
- telepon;
- metadata sumber;
- `synced_at` dan `last_seen_at`.

Gunakan `$setOnInsert` untuk:

- `_id` UUID baru;
- `bpjs: nil`;
- `services: []`;
- `beds: []`.

Dengan demikian sinkronisasi ulang tidak menimpa:

- embedded `services` hasil operator/kurasi;
- embedded `beds` hasil operator/demo;
- `bpjs` jika suatu hari dikurasi dari sumber berbeda;
- UUID lokal yang sudah dipakai referral/operator.

Jangan menggunakan `ReplaceOne` terhadap seluruh dokumen hospital.

Gunakan `BulkWrite` per halaman atau chunk yang terukur. Ordered/unordered harus dipilih sengaja dan hasil partial failure harus masuk summary. Jangan menulis query BSON dari string input.

## UUID lokal

`_id` rumah sakit tetap UUID opaque milik SiagaRS. `kode_satusehat` disimpan pada `external_id`, bukan dijadikan `_id`.

Pada insert baru, buat UUID v4 menggunakan `crypto/rand` atau helper internal yang aman. Pada update, pertahankan `_id` lama melalui `$setOnInsert`.

## Workflow sinkronisasi

Service sinkronisasi harus:

1. Memastikan integration enabled dan environment sandbox.
2. Mulai dari page 1.
3. Memanggil `FetchPage(ctx, page, limit)`.
4. Memvalidasi page dan `total_page` agar tidak terjadi loop tanpa batas.
5. Map record valid; record invalid masuk counter `skipped`.
6. Upsert satu batch ke MongoDB.
7. Lanjut sampai halaman terakhir.
8. Mengembalikan summary.

Contoh summary:

```go
type SyncSummary struct {
    Pages    int
    Fetched  int
    Created  int
    Updated  int
    Unchanged int
    Skipped  int
    Failed   int
    StartedAt time.Time
    FinishedAt time.Time
}
```

Summary log hanya berisi angka, environment, durasi, dan error category. Jangan log payload penuh, header Authorization, credential, atau query yang berisi informasi sensitif.

## Perintah manage

Tambahkan command:

```text
go run ./cmd/manage sync-satusehat
```

Perbarui usage:

```text
go run ./cmd/manage migrate|seed|operator|sync-satusehat
```

Command harus:

- membutuhkan `MONGODB_URI` dan database yang valid;
- membutuhkan `SATUSEHAT_ENABLED=true`;
- menolak production;
- menggunakan context yang dapat dibatalkan dan timeout total yang masuk akal, lebih panjang dari timeout 60 detik command lama;
- menutup Mongo client;
- keluar non-zero saat sinkronisasi gagal;
- tidak menjalankan migration atau seed secara otomatis;
- tidak menghapus data.

Contoh cara jalan yang didokumentasikan, tanpa nilai rahasia:

```powershell
$env:MONGODB_URI='mongodb+srv://...'
$env:MONGODB_DATABASE='siagars'
$env:SATUSEHAT_ENABLED='true'
$env:SATUSEHAT_ENV='sandbox'
$env:SATUSEHAT_CLIENT_ID='...'
$env:SATUSEHAT_CLIENT_SECRET='...'
go run ./cmd/manage sync-satusehat
```

## HTTP policy

- Timeout per request: default 10 detik.
- Batasi ukuran body response OAuth dan MSI.
- Set `Accept: application/json` untuk MSI.
- Validasi status HTTP sebelum decode.
- 401 MSI: invalidate token, refresh, retry sekali.
- 403: gagal permanen; tidak retry.
- 400/422: gagal konfigurasi/request; tidak retry.
- 429: baca `Retry-After`; retry terbatas dan tetap tunduk context deadline.
- 5xx/network timeout: maksimal dua retry untuk GET MSI menggunakan exponential backoff + jitter.
- Jangan retry request OAuth secara agresif; dokumentasi workspace menyebut percobaan autentikasi gagal dapat dikenai waktu tunggu.
- Jangan retry write FHIR karena fase ini tidak mempunyai write upstream.

## Last-known-good dan kegagalan parsial

- Jika token/upstream gagal sebelum fetch, MongoDB tidak berubah.
- Jika halaman tertentu gagal setelah retry, command gagal dan melaporkan halaman/error category.
- Jangan menghapus halaman lain atau seluruh hasil sinkronisasi sebelumnya.
- Jangan memperbarui `synced_at` untuk record yang tidak benar-benar diterima dan divalidasi.
- Jangan menandai `verified_at`.
- Fasilitas yang tidak muncul pada satu run jangan langsung dihapus/nonaktifkan karena run bisa parsial.
- Set `source_active=false` hanya jika upstream secara eksplisit mengirim record tersebut sebagai inactive, bukan karena record tidak terlihat.

## Pengujian unit adapter

Gunakan `httptest.Server`, tanpa credential nyata.

Minimal test:

1. OAuth request memakai method, query, content type, dan form yang benar.
2. Token di-cache dan tidak dibuat ulang sebelum expiry.
3. Token expired direfresh.
4. Concurrent access hanya membuat satu token request.
5. MSI menerima Authorization bearer yang benar.
6. Pagination menentukan `NextPage` dengan benar.
7. Empty page berhenti dengan aman.
8. `total_page` invalid tidak menyebabkan infinite loop.
9. 401 menginvalidate token dan retry satu kali.
10. 403 tidak retry.
11. 429 menghormati retry policy.
12. 5xx/timeout bounded.
13. Invalid JSON dan oversized body ditolak.
14. Null field tidak panic.
15. Koordinat kosong/invalid/out-of-range menjadi nil berpasangan.
16. Record tanpa ID/nama/wilayah di-skip.
17. Secret/token tidak muncul dalam error string atau log.

## Pengujian integration MongoDB

Gunakan `TEST_MONGODB_URI` dan database test unik seperti pola test yang sudah ada. Jangan menyentuh atau drop database pengguna.

Minimal test:

- Import pertama membuat region dan hospital.
- Import kedua idempotent dan mempertahankan `_id`.
- Unique `(source_namespace, external_id)` mencegah duplikasi.
- Perubahan nama/alamat upstream memperbarui field MSI.
- Embedded `services` tidak tertimpa.
- Embedded `beds` tidak tertimpa.
- `bpjs` yang dikurasi tidak tertimpa menjadi nil.
- Dokumen `source_active=false` tidak muncul di pencarian publik.
- Dokumen simulation tanpa `source_active` tetap muncul.
- Import invalid tidak merusak last-known-good.
- Region parent/code konsisten.

## Kontrak publik

Jangan menambah endpoint baru dan jangan mengubah response frontend pada fase ini. `docs/openapi.yaml` hanya perlu diperbarui bila perilaku HTTP publik benar-benar berubah.

Data hasil import harus muncul melalui endpoint yang sudah ada dengan:

```json
{
  "data_origin": "satusehat_sandbox",
  "source_updated_at": null,
  "synced_at": "<waktu sinkronisasi UTC>",
  "verified_at": null,
  "freshness": "unknown",
  "bpjs": null,
  "services": [],
  "beds": []
}
```

Jangan mengisi layanan atau bed dengan `unknown` entries hanya agar array terlihat lengkap. Array kosong lebih jujur bila MSI tidak menyediakan data.

## Keamanan dan credential

Sebelum live test:

- Jangan baca atau mencetak `atlas-credentials.env`.
- Pastikan semua file credential tetap git-ignored.
- Jangan masukkan file credential ke ZIP/artifact.
- Jangan menyimpan SATUSEHAT secret di MongoDB.
- Jangan memanggil production.
- Jangan mengirim data pasien atau data medis.
- Jangan menulis payload FHIR.

Credential Atlas yang pernah tersimpan dalam file plaintext lokal harus dianggap perlu dirotasi oleh pemilik akun. Implementasi tidak boleh bergantung pada nilai credential lama tersebut.

## Definition of Done

Implementasi dianggap selesai bila:

- `SATUSEHAT_ENABLED=false` tetap membuat API berjalan seperti sebelumnya.
- `sync-satusehat` tersedia dan menolak konfigurasi tidak aman.
- Adapter OAuth dan MSI mempunyai unit test lengkap menggunakan mock server.
- Import MongoDB idempotent.
- UUID, services, beds, BPJS, operator, dan referral yang sudah ada tidak rusak/tertindih.
- Data sandbox berlabel `satusehat_sandbox`.
- `source_updated_at`/`verified_at` tidak dipalsukan.
- Tidak ada credential atau token di source/test/log/output.
- `gofmt -l` tidak menghasilkan file.
- `go build ./...` berhasil.
- `go vet ./...` berhasil.
- `go test ./...` berhasil.
- Integration test dijalankan hanya bila `TEST_MONGODB_URI` tersedia dan menggunakan database test terisolasi.
- README menjelaskan command, environment, keterbatasan, dan bahwa live sandbox belum terbukti jika credential tidak tersedia.

## Handoff yang wajib diberikan Claude setelah implementasi

Laporkan:

1. Ringkasan perilaku yang berubah.
2. Daftar file yang dibuat/diubah.
3. Migration baru dan dampaknya.
4. Command verifikasi yang benar-benar dijalankan.
5. Hasil unit/integration test.
6. Apakah live Sandbox diuji atau hanya mock.
7. Field MSI yang berhasil dipetakan.
8. Keterbatasan yang tersisa.
9. Konfirmasi bahwa tidak ada credential yang dicetak/di-commit.

Jangan mengklaim SATUSEHAT terhubung hanya karena mock test lulus. Live Sandbox baru boleh dinyatakan berhasil setelah request autentikasi dan MSI memakai credential sah serta hasilnya diverifikasi tanpa membocorkan data rahasia.

## Fase berikutnya, jangan dikerjakan sekarang

Setelah MSI stabil dan di-review, pekerjaan terpisah dapat mengevaluasi:

1. FHIR `Organization` dan `Location` read-only untuk fasyankes pilot.
2. Mapping layanan fasilitas yang benar-benar didukung sumber resmi.
3. Proof-of-concept Rujukan Pasien Sandbox menggunakan `Task`, `CarePlan`, `ServiceRequest`, dan `Encounter`.

Rujukan resmi tidak boleh dicampurkan ke pekerjaan MSI ini.
