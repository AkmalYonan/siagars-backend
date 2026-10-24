# Status implementasi SATUSEHAT MSI

Dokumen ini mencatat kondisi implementasi adapter SATUSEHAT Master Sarana Index (MSI) pada versi kerja 0.1. Dokumen ini bukan lagi prompt implementasi. Sumber kebenaran kontrak tetap `openapi.yaml`; keputusan teknis dan status proyek berada di `DECISIONS.md`, `PRD.md`, dan `../README.md`.

## Status saat ini

Adapter MSI Sandbox read-only sudah tersedia. Sinkronisasi dijalankan secara manual dan hasilnya di-upsert ke MongoDB agar API publik membaca data melalui repository yang sama dengan data seed dan kurasi.

Perintah utama:

```bash
go run ./cmd/manage sync-satusehat
```

Entrypoint kompatibel berikut juga didukung:

```bash
go run . sync-satusehat
```

Implementasi utama berada di:

- `internal/satusehat/token.go`: OAuth2 client credentials dan cache token dalam proses.
- `internal/satusehat/msi_client.go`: client MSI, pagination, retry terbatas, dan validasi respons.
- `internal/satusehat/mapper.go`: normalisasi data MSI ke model rumah sakit SiagaRS.
- `internal/repository/mongo`: upsert dan provenance data hasil sinkronisasi.
- `migrations/002_satusehat_fields.js`: field serta index yang diperlukan adapter.

## Alur sinkronisasi

1. Aplikasi memvalidasi konfigurasi dan menolak environment SATUSEHAT production.
2. Client mengambil token dari Sandbox.
3. Data wilayah dan rumah sakit dibaca per halaman untuk cakupan yang dikonfigurasi.
4. Mapper mempertahankan nilai unknown sebagai `null` atau field kosong sesuai model; data tidak direka.
5. Repository melakukan upsert idempotent berdasarkan identitas sumber.
6. Ringkasan `created`, `updated`, `unchanged`, `skipped`, dan `failed` dicetak tanpa membocorkan secret.

Data lokal yang tidak muncul dalam satu proses sinkronisasi tidak otomatis dihapus atau dinonaktifkan. Kegagalan sinkronisasi juga tidak menghapus last-known-good data.

## Konfigurasi

Gunakan variabel environment yang didokumentasikan di `.env.example`, termasuk base URL Sandbox, OAuth client ID/secret, serta kode wilayah bila diperlukan. Kredensial hanya boleh berada di server atau secret store dan tidak boleh dikirim ke frontend maupun disimpan di repository.

## Batas integrasi

- Hanya SATUSEHAT Sandbox; production sengaja ditolak.
- Hanya pembacaan direktori fasilitas dari MSI. Implementasi ini bukan integrasi FHIR dan tidak mengirim data pasien.
- MSI belum menjadi sumber layanan, tempat tidur, status BPJS, atau antrean. Field tersebut tetap berasal dari sumber lain dan membawa provenance masing-masing.
- `source_updated_at` MSI tidak dianggap sebagai waktu pembaruan klinis bila sumber tidak menyediakannya.
- Sinkronisasi nasional terjadwal dan deteksi fasilitas yang hilang dari sumber belum tersedia.

## Bukti verifikasi

Unit test menggunakan server mock untuk token, pagination, retry, mapping, dan kondisi gagal. Suite integrasi MongoDB menguji idempotensi dan provenance. Uji live Sandbox yang dicatat pada 23 Oktober 2026 untuk Jabodetabek menghasilkan 14 kabupaten/kota dan 516 rumah sakit dibuat tanpa skip/gagal; pengulangan berikutnya menghasilkan 516 `unchanged`.

Hasil Sandbox adalah bukti teknis adapter, bukan klaim kesiapan integrasi medis production.

## Referensi

- `../README.md`: cara menjalankan command dan status verifikasi.
- `DECISIONS.md`: keputusan desain, keamanan, dan provenance.
- `../PRD.md`: scope produk serta batas klaim.
- `openapi.yaml`: kontrak HTTP yang dikonsumsi frontend.
