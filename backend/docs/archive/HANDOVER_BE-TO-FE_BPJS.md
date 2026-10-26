# Balasan Backend: Cek BPJS (Simulasi) dan Riwayat Operator

Untuk: M. Tauriq Arya (frontend). Dari: Akmal Bariq Yonanda (backend). Tanggal: 25 Oktober 2026. Menjawab [`HANDOVER_FE-TO-BE_BPJS.md`](HANDOVER_FE-TO-BE_BPJS.md). Versi API: **0.3.0**.

Kontrak resmi: [`openapi.yaml`](openapi.yaml) (tag **BPJS (simulasi)** dan **Admin**), contoh respons siap tempel: [`fixtures.json`](fixtures.json) (`bpjs_found`, `bpjs_not_found`).

## 1. Status dan lokasi

| | |
|---|---|
| Staging (`develop`) | **Tersedia** setelah push versi ini. URL dan bypass seperti [`HANDOVER_v0.2.md`](HANDOVER_v0.2.md) |
| Production | **Belum dirilis** (menunggu keputusan Akmal; butuh migration dan kunci server baru) |

Bentuk endpoint **sama dengan usulan FE** (§3-§5 dokumen Arya) tanpa perubahan nama field. Dua tambahan: dua endpoint admin untuk riwayat (bagian 4) dan pencatatan input (bagian 3).

## 2. `POST /api/v1/bpjs/check` (publik)

Request dan respons persis usulan FE. Ringkasnya:

```json
// request
{ "nik": "9999010101900001", "name": "Budi Santoso", "birth_date": "1990-01-01" }
// respons 200 (found)
{ "data": { "result": "found",
  "participant": { "nik_masked": "9999********0001", "name_masked": "BU** SA*****", "status": "active", "inactive_reason": null,
    "membership_type": { "code": "PBPU", "name": "Pekerja Bukan Penerima Upah" }, "care_class": "3",
    "primary_facility": { "code": "SIM-FKTP-001", "name": "Puskesmas Simulasi Kebayoran" } },
  "data_origin": "simulation", "checked_at": "2026-10-25T08:00:00Z" } }
// respons 200 (not_found): result "not_found", participant null
```

- **Selalu 200 untuk input valid**, termasuk tidak cocok. `not_found` tidak membedakan NIK, nama, atau tanggal lahir yang salah.
- Pencocokan: NIK dan tanggal lahir persis; nama tidak membedakan huruf besar-kecil dan spasi ganda dirapikan (huruf beraksen harus sama).
- Respons **tidak pernah** memuat NIK atau nama lengkap; selalu tersamar (NIK: 4 awal + 8 `*` + 4 akhir; nama: 2 huruf awal tiap kata, huruf besar).
- `Cache-Control: no-store`. Rate limit **10 permintaan/menit per IP**; berlebih dibalas `429` + `Retry-After` (nilai tetap `60` detik).
- Galat: `400 invalid_json` (body rusak atau field asing), `413 body_too_large`, `422 validation_error` (pesan Bahasa Indonesia, tanpa menggemakan input), `429 rate_limited`, `5xx internal_error`, `501 not_implemented` (server tanpa dukungan fitur). `404` **tidak** dipakai untuk "tidak cocok".
- Validasi: NIK tepat 16 digit; nama 1-100 karakter (huruf, spasi, `.`, `'`, `-`); `birth_date` `YYYY-MM-DD` valid, tidak di masa depan (WIB), tidak sebelum 1900-01-01.

## 3. Yang berbeda dari usulan FE: input DICATAT

Atas keputusan pemilik backend, setiap pemeriksaan **dicatat** agar operator dapat melihat riwayat input halaman `/cek-bpjs`. Ini berbeda dari butir §6.1-6.2 usulan FE ("jangan simpan"). Pengamannya:

- Yang tersimpan: NIK dan nama **tersamar**, tahun lahir, hasil, status, waktu, dan **data asli terenkripsi** (AES-256-GCM, kunci hanya ada di environment backend). Sidik jari NIK (HMAC) dipakai untuk menghitung pengulangan tanpa menyimpan NIK terbaca.
- **Dihapus otomatis 30 hari** setelah pemeriksaan.
- Data asli hanya bisa dibuka operator yang login **dengan memasukkan ulang passwordnya**; setiap percobaan dicatat di audit (tanpa data pribadi).
- Tidak ada NIK/nama/tanggal lahir di log, URL, atau respons publik.
- Input yang ditolak validasi (422) tidak dicatat.

### Permintaan ke FE (mohon dikerjakan)

1. **Ubah teks persetujuan** di `/cek-bpjs`. Teks lama ("hanya memakai data demo") tidak cukup. Usulan:
   > "Ini simulasi dan bukan layanan BPJS Kesehatan. Gunakan hanya data demo, jangan masukkan NIK atau data pribadi asli. Data yang Anda masukkan dicatat untuk riwayat operator dan dihapus otomatis setelah 30 hari."
2. Pertahankan FE seperti usulan sendiri: tanpa atribut `name` pada input, data hanya di memori, formulir dikosongkan setelah hasil.
3. Tambahkan halaman riwayat operator (bagian 4).

## 4. Riwayat untuk operator (butuh login)

Keduanya memakai sesi operator seperti endpoint admin lain (lihat [`HANDOVER_v0.2.md`](HANDOVER_v0.2.md) bagian 3 untuk login/CSRF). Semua operator boleh melihat (tidak terikat satu RS).

### `GET /api/v1/admin/bpjs-checks?page=&limit=&result=`

`limit` default 20 (maks 50), `result` = `found` atau `not_found` (opsional). Parameter lain `422`. Terbaru dulu.

```json
{ "data": [ { "id": "6f1c2f7e-...", "nik_masked": "9999********0001", "name_masked": "BU** SA*****", "birth_year": 1990,
    "result": "found", "status": "active", "checked_at": "2026-10-25T08:00:00Z", "repeat_count": 2 } ],
  "meta": { "page": 1, "limit": 20, "total": 1, "total_pages": 1, "as_of": "2026-10-25T08:05:00Z" } }
```

`status` null untuk `not_found`. `repeat_count` = berapa kali NIK yang sama muncul dalam 30 hari terakhir.

### `POST /api/v1/admin/bpjs-checks/{id}/reveal` (tombol "Lihat data asli")

Mutasi: wajib header `Origin`, `X-CSRF-Token`, dan body password operator yang sedang login (UI: minta password lewat dialog, jangan disimpan).

```json
// request
{ "password": "<password operator>" }
// respons 200
{ "data": { "id": "6f1c2f7e-...", "nik": "9999010101900001", "name": "Budi Santoso", "birth_date": "1990-01-01", "checked_at": "2026-10-25T08:00:00Z" } }
```

| Status | `error.code` | Arti / tampilan |
|---|---|---|
| 401 | `invalid_credentials` | Password salah, atau sesi habis -> "Password tidak valid" |
| 403 | `invalid_csrf` / `invalid_origin` | Muat ulang token CSRF / cek origin |
| 404 | `not_found` | Entri sudah dihapus (lewat 30 hari) atau id salah |
| 409 | `history_raw_unavailable` | Data asli tidak tersedia untuk entri ini -> tampilkan "Hanya data tersamar yang tersedia" |
| 429 | `rate_limited` | Maks 5 percobaan per 15 menit; tampilkan hitung mundur |

Tampilkan data asli hanya selama dialog terbuka; jangan menaruhnya di URL, `localStorage`, atau log.

## 5. Data demo (fiktif, boleh dipakai di repo FE)

NIK diawali `9999`, kode wilayah yang tidak ada, sehingga tidak mungkin milik orang nyata. Nama dibandingkan tanpa membedakan huruf besar-kecil.

| NIK | Nama | Tanggal lahir | Hasil yang diharapkan |
|---|---|---|---|
| `9999010101900001` | Budi Santoso | 1990-01-01 | **active** lengkap: PBPU "Pekerja Bukan Penerima Upah", kelas 3, faskes "Puskesmas Simulasi Kebayoran" |
| `9999020202850002` | Siti Aminah | 1985-02-02 | **inactive**, `inactive_reason: "Tunggakan iuran 3 bulan"`, PBPU, kelas 2 |
| `9999030303950003` | Agus Pratama | 1995-03-03 | **active** sebagian kosong: PPU, `care_class: null`, `primary_facility: null` ("Belum diketahui") |
| `9999040404700004` | Ratna Dewi | 1970-04-04 | **active** PBI: `membership_type.code: "PBI"`, kelas 3 |
| `9999050505880005` | José Álvarez-Núñez | 1988-05-05 | **active** PPU kelas 1; menguji huruf beraksen (kirim sebagai UTF-8) |

Kombinasi lain (termasuk NIK 16 digit apa pun yang tidak ada di tabel) menghasilkan `not_found`. Contoh: NIK demo dengan nama atau tanggal lahir salah.

## 6. Pengujian cepat (staging)

```bash
curl -s -X POST "$STAGING_URL/api/v1/bpjs/check" \
  -H "x-vercel-protection-bypass: $VERCEL_AUTOMATION_BYPASS_SECRET" -H "Content-Type: application/json" \
  -d '{"nik":"9999010101900001","name":"Budi Santoso","birth_date":"1990-01-01"}'
```

Dari FE, tetap lewat route handler proxy ([`API_FRONTEND.md`](API_FRONTEND.md) bagian 2a). Endpoint publik tidak butuh Origin; endpoint admin butuh origin FE yang diizinkan.

## 7. Batasan

- Ini **simulasi**. Tidak ada cek ke BPJS Kesehatan; tidak ada API publik untuk itu. Cek kepesertaan nyata butuh kerja sama resmi dengan BPJS Kesehatan (VClaim) dan kredensialnya.
- Rate limit dihitung per instance server (Vercel serverless), jadi pembatasan enumerasi tidak sepenuhnya ketat.
- Bila pengguna tetap memasukkan data asli, datanya tersimpan (terenkripsi, 30 hari) sesuai bagian 3. Pemberitahuan di FE adalah satu-satunya pencegahnya.
