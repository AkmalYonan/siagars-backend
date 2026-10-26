# SiagaRS
## Spesifikasi teknis sistem V0.1

Tim MalasNgoding | Dua proyek: Next.js dan Go | Baseline 23 Oktober 2026

Dokumen ini menyatukan kontrak lintas proyek. Frontend menampilkan informasi dan menerima interaksi; Go memiliki validasi, autentikasi, business logic, penyimpanan serta adapter eksternal. Database yang direkomendasikan adalah PostgreSQL. Belum ada klaim aplikasi atau integrasi telah dibangun.

### 01 / Arsitektur dan batas tanggung jawab

Browser -> origin SiagaRS -> reverse proxy -> Next.js untuk halaman, Go untuk `/api/v1/*` -> PostgreSQL. Go juga dapat memanggil adapter SATUSEHAT melalui jalur server-to-server yang diotorisasi.

Same-origin dipilih agar cookie operator, CSRF dan konfigurasi development/production konsisten. Dua repository tetap terpisah. Di development, Next.js rewrite meneruskan API ke Go localhost:8080. Pada hosting yang membutuhkan, Next.js boleh menjadi forwarding HTTP tipis. Tidak ada business logic atau database kedua di frontend.

| Komponen | Pemilik dan fungsi |
|---|---|
| Frontend Next.js | Tauriq; routes, komponen, filter URL, peta/daftar, forms, timeline |
| API Go | Akmal; query, auth, RS scope, state machine, transaksi dan DTO |
| PostgreSQL | Data pilot, sessions, status layanan/bed, rujukan sintetis dan audit |
| SATUSEHAT adapter | Opsional; master data sandbox, normalisasi dan provenance |
| Product/QA | Radja sebagai usulan selain peran ketua; scope, validasi dan demo |

### 02 / Prinsip implementasi

Satu service Go cukup V0.1. Tidak memerlukan microservices, queue atau Redis. Cache token in-memory dan query database pilot memadai untuk skala demo. Tambah infrastruktur hanya setelah ada kebutuhan terukur.

Next.js dan Go dipilih tim. Versi stabil, router Go, SQL library, UI library dan hosting dikunci saat setup setelah compatibility check. Jangan mengikuti versi terbaru tanpa uji build.

---PAGE---
## Model domain dan kualitas data

### 03 / Entitas minimum

| Entitas | Relasi dan constraint penting |
|---|---|
| hospitals + regions | UUID RS, wilayah FK; BPJS/koordinat boleh unknown; external ID+namespace unik |
| services | Slug stabil: icu, nicu, picu, igd-24h, ambulance |
| hospital_services | Unik RS+slug; status, provenance dan version |
| bed_availability | Unik RS+category; total/available nullable, nonnegatif dan available<=total bila known |
| operators + operator_hospitals | Akun dan RS yang ditugaskan; semua operasi admin dibatasi server |
| sessions | Hash session, expiry, revoke dan informasi CSRF |
| referrals | Pemilik RS, tujuan, hash kode, status, public note, version, is_demo=true |
| referral_status_history | Append-only per perubahan; from/to, waktu, note publik dan actor internal |
| audit_logs | Tindakan operator dan request ID, tanpa secret/data medis |

### 04 / Metadata per informasi

`data_origin`: curated, simulation, satusehat_sandbox atau satusehat_production. `source_url`: tautan sumber; nullable untuk simulasi internal. `source_updated_at`: waktu perubahan pada sumber. `synced_at`: waktu diambil sistem. `verified_at`: pemeriksaan nyata, bukan otomatis karena import. Layanan dan bed memiliki metadata sendiri.

`freshness`: fresh jika usia sumber <1 jam; aging jika 1-24 jam; stale jika >24 jam; unknown bila timestamp tidak ada/tidak valid. Ini kebijakan demo, bukan standar medis. Jam server menjadi acuan dan disertakan dalam meta.as_of. Field simulation tetap simulation sekalipun fresh.

`available=null` berarti belum diketahui. `available=0` berarti menurut sumber tidak ada bed kosong. BPJS null berarti belum diketahui, bukan false. Informasi yang hilang tidak diisi dengan tebakan. Keterbaruan dan kelengkapan tidak menjamin pasien diterima; tindakan pengguna berikutnya adalah konfirmasi kontak RS.

### 05 / Migrasi dan seed

Migration bernomor dan dapat diterapkan pada database kosong. Seed idempotent berisi 30-50 RS target dan minimal 10 profil lengkap bila kurasi berhasil; setiap data membawa label asal. Rujukan selalu sintetis. Seed tidak melakukan reset atau menimpa perubahan operator default. Backup sebelum perubahan schema yang berdampak.

---PAGE---
## Kontrak API bersama

### 06 / Konvensi

Prefix `/api/v1`; JSON snake_case; ID string opaque; UTC RFC3339 pada wire dan WIB pada UI. Sukses `{data,meta?}`. Galat `{error:{code,message,fields?},request_id}`. Meta hasil list: page, limit, total, total_pages, as_of dan data_scope=pilot.

| Method | Path relatif prefix | Tujuan |
|---|---|---|
| GET | /health; /ready | Liveness; readiness database |
| GET | /hospitals; /hospitals/{id} | Hasil pencarian; detail |
| GET | /services | Master filter layanan |
| POST | /referrals/lookup | Timeline sintetis dari body tracking_code |
| POST | /auth/login; /auth/logout | Membuat/mencabut session |
| GET | /auth/me; /auth/csrf | Profil scope RS; token CSRF |
| GET | /admin/referrals | Rujukan sesuai scope operator |
| PATCH | /admin/referrals/{id}/status | Status + public_note + version |
| PATCH | /admin/hospitals/{id}/services/{slug} | Status layanan + version |
| PATCH | /admin/hospitals/{id}/beds/{category} | Total/available + version |

HTTP: 400 malformed; 401 session hilang; 403 scope/CSRF; 404 tidak ditemukan; 405 method; 409 konflik version/transisi; 422 validasi; 429 rate limit; 500 kegagalan internal; 503 readiness. FE memetakan code, tidak menafsirkan pesan sebagai enum.

### 07 / Search dan filter

Contoh: GET `/api/v1/hospitals?q=bekasi&services=nicu&bpjs=true&page=1&limit=20`.

`services` comma-separated menggunakan AND; filter wilayah via province_code/city_code; q maksimal 100 karakter. Limit default20/max50, page>=1. Parameter unknown ditolak 422. Sort default name lalu ID sebagai tie-break. `sort=distance` membutuhkan lat/lng berpasangan dan valid. Jarak Haversine adalah garis lurus, bukan waktu tempuh.

Map V0.1 dan list memakai halaman hasil aktif yang sama. Jangan menampilkan total marker seolah semua RS sudah dimuat. Sort match dan filter has_available_beds termasuk Should. Filter bed hanya memasukkan available>0, data fresh dan non-simulation; mode demo dapat menampilkan bed simulasi dengan label tanpa dianggap bukti ketersediaan nyata.

---PAGE---
## Payload dan state machine

### 08 / Contoh wire format

Respons detail ringkas berikut hanya contoh sintetis, bukan data rumah sakit nyata:

```json
{
  "data": {
    "id": "demo-hospital-01",
    "name": "RS Demo Siaga",
    "bpjs": null,
    "data_origin": "simulation",
    "services": [{
      "slug": "nicu",
      "status": "available",
      "data_origin": "simulation",
      "source_url": null,
      "source_updated_at": "2026-10-23T08:00:00Z",
      "synced_at": "2026-10-23T08:01:00Z",
      "verified_at": null,
      "freshness": "fresh",
      "version": 1
    }]
  },
  "meta": {"as_of": "2026-10-23T08:20:00Z"}
}
```

ID contoh di atas bersifat ilustratif; implementasi memakai UUID. DTO detail aktual menambah alamat, wilayah, koordinat, telepon dan beds sebagaimana PRD. Status layanan: available, limited, unavailable, unknown.

### 09 / State machine lokal rujukan

submitted -> verified -> searching -> confirmed -> completed. verified/searching -> rejected. confirmed -> searching berarti dialihkan dan memerlukan alasan. completed/rejected terminal. Semua transisi lain 409. Dialihkan adalah event/label riwayat, bukan status tersendiri.

PATCH status menerima status, public_note dan version. Update current status, history dan audit dalam satu transaksi. Increment version; dua update dengan version sama menghasilkan satu sukses dan satu 409. FE refetch saat konflik. Server menentukan timestamp. Tidak ada optimistic UI untuk perubahan status kritis ini.

---PAGE---
## Session dan akses publik

### 10 / Auth operator

Gunakan opaque session random >=256 bit, simpan hash di database, TTL awal 8 jam, revoke logout. Password memakai library hash tepercaya. Cookie siagars_session: HttpOnly, Secure pada HTTPS, SameSite=Lax, Path=/api/v1. Development HTTP lokal punya pengecualian Secure=false yang eksplisit.

GET `/auth/csrf` mengembalikan token terkait session. Semua mutasi operator membawa X-CSRF-Token dan Origin valid. Login memeriksa Origin dan JSON content type; rate limit. Backend memeriksa scope operator_hospitals pada setiap akses admin. Menyembunyikan tombol frontend tidak menggantikan otorisasi.

### 11 / Tracking sebagai capability demo

POST lookup menerima kode melalui body, bukan URL. Kode random aktual memiliki >=128 bit entropy dan disimpan hash; SR-DEMO-001 hanya contoh publik sintetis. Respons hanya public_status, public_note, next_step, RS tujuan, updated_at dan history publik. Jangan masukkan identitas pasien, diagnosis, internal note atau actor identity.

Tracking/operator menggunakan Cache-Control no-store. Kode tidak disimpan di URL, localStorage, analytics atau log. Default rate limit lookup 10/menit/IP; login gagal 5/15 menit/IP+akun. Tuning mengikuti observasi demo. Gunakan trusted proxy agar X-Forwarded-For tidak bisa dipalsukan sembarang.

### 12 / Error, log dan operasional

Request ID memudahkan penelusuran. Log JSON berisi route, status, durasi dan jenis kegagalan; redact cookie, credential dan kode lookup. Body limit awal 64 KB; request timeout 10 detik; upstream timeout5 detik. Health tidak membocorkan konfigurasi; readiness memeriksa DB.

Environment backend meliputi DATABASE_URL, PUBLIC_ORIGIN, DEMO_MODE=true dan SATUSEHAT_ENABLED=false. `.env.example` hanya placeholder. NEXT_PUBLIC_* pada FE hanya data konfigurasi publik; dokumentasi Next.js menyatakan prefix tersebut memasukkan nilai ke bundle browser [R3].

---PAGE---
## Integrasi SATUSEHAT dan strategi delivery

### 13 / Integrasi bertahap

Master Sarana Index membutuhkan OAuth2 client_credentials server-to-server dan kredensial partner terverifikasi. Public workspace membuka dokumentasi request, tidak memberi akses bebas atau izin publikasi semua data [R1]. Location adalah struktur lokasi; Organization struktur organisasi. Field BPJS, fasilitas atau bed tidak boleh diasumsikan lengkap hanya karena resource tersedia.

Adapter opsional menerima akses sah; ambil master data sandbox read-only dahulu. Token disimpan di Go dengan expiry dari expires_in dan safety margin; jangan generate token per request pengguna. Import upsert memakai external ID+namespace, provenance per field dan hasil unknown untuk field tak tersedia. Timeout/retry bounded, 401 refresh sekali, 429 patuhi Retry-After. Kegagalan upstream mempertahankan data terakhir dengan usia sumber yang sebenarnya.

Tracking resmi SATUSEHAT melibatkan Task, ServiceRequest, CarePlan dan Encounter dengan proses antarfasyankes [R2]. State machine demo SiagaRS mengacu konsep progres tersebut, tetapi belum merupakan implementasi interoperabilitas atau rujukan medis resmi. Jangan mengirim transaksi klinis produksi sebagai langkah demo.

### 14 / Kontrak dan definition of done

Akmal membuat OpenAPI `docs/openapi.yaml`; Tauriq memakai snapshot versi cocok. Contoh DTO, enum, galat, nullable dan auth wajib ditulis. Perubahan schema/enum membutuhkan perubahan FE+BE yang berpasangan. Fixture deterministik membuat FE bisa berjalan saat BE sedang dikerjakan.

Gate: FE lint/typecheck/test/build; Go build/gofmt/vet/test; integrasi DB, auth/CSRF/RS scope, konflik version, filter AND dan null/0. E2E pencarian->detail->kontak serta operator->status->lookup. Uji mobile dan keyboard; data pilot dapat dijalankan lokal. Jangan mengklaim tes lulus sebelum kode tersedia.

### Referensi resmi (akses 23 Oktober 2026)

[R1] SATUSEHAT MSI ReST API: https://satusehat.kemkes.go.id/platform/docs/id/master-data/master-sarana-index/rest-api-msi/

[R2] SATUSEHAT Rujukan Pasien: https://satusehat.kemkes.go.id/platform/docs/id/interoperability/rujukan/

[R3] Next.js Environment Variables: https://nextjs.org/docs/app/guides/environment-variables

[R4] SATUSEHAT Public Postman: https://www.postman.com/satusehat/satusehat-public/overview

[R5] SATUSEHAT Location: https://satusehat.kemkes.go.id/platform/docs/id/fhir/resources/location/

Referensi produk: empat PDF GuideBook dan percakapan Tim MalasNgoding. Parameter internal seperti batas kesegaran, TTL session, rate limit dan target performa adalah usulan engineering V0.1 yang dapat dituning, bukan ketentuan SATUSEHAT/lomba.
