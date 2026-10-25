# Pipeline kurasi data RS

Alat untuk menambahkan fakta layanan, bed, dan BPJS dari **sumber publik yang sah** ke RS hasil MSI yang sudah ada di database. Folder ini tidak berisi data RS nyata. Semua berkas contoh fiktif (`PLACEHOLDER-*`, host `.invalid`) dan **ditolak oleh `--apply`**.

## Berkas

| Berkas | Fungsi |
|---|---|
| `template.json`, `template.csv` | Kerangka kosong. Sengaja ditolak validator sampai diisi. |
| `example.placeholder.json`, `example.placeholder.csv` | Contoh fiktif untuk dry-run. Bukan fakta RS. |

## Menjalankan

~~~powershell
# Dry-run (default): validasi dan laporan saja, tanpa koneksi database, tidak ada yang ditulis.
go run ./cmd/manage curate data/curation/berkas-saya.json
go run . curate data/curation/berkas-saya.json

# Apply: hanya setelah checklist manusia di bawah selesai dan PIC menyetujui.
go run . curate data/curation/berkas-saya.json --apply
~~~

Kedua entrypoint menerima `curate <file> [--apply]`. Untuk otomasi, Anda juga dapat memakai `CURATE_FILE` dan `CURATE_APPLY=true`. Tes `go test ./seed` memanggil parser dan alur dry-run secara langsung.

`--apply` membaca `MONGODB_URI` dan `MONGODB_DATABASE` dari environment, mensyaratkan migration 002, dan **tidak membuat, menghapus, atau mereset** dokumen. Arahkan ke database yang memang dimaksud; jangan ke `siagars` tanpa persetujuan pemilik data.

## Format

JSON: `{"schema_version":1,"hospitals":[...]}`. CSV: satu RS per baris, kolom `svc_<slug>` dan `bed_<kategori>_total|available`. Sel kosong berarti **tidak diketahui (null)**, bukan 0.

| Field | Aturan |
|---|---|
| `external_id` | Wajib. Kode SATUSEHAT MSI RS yang **sudah ada** di database (hasil `sync-satusehat`). Tanpa spasi. |
| `source_namespace` | Opsional, default `satusehat:msi:sandbox`. |
| `name` | Opsional tetapi dianjurkan. Jika ada dan tidak sama dengan nama di database, RS dilewati (jaring pengaman salah ID). |
| `bpjs` | `true`, `false`, atau null/kosong. Hanya mengisi jika di database masih null. Sumber dan waktunya (`source_url`, `source_updated_at` pada record) disimpan sebagai `bpjs_source_url` / `bpjs_source_updated_at` dan tampil di API. |
| `source_url` | Wajib, `https://`, halaman publik tempat fakta dibaca. Bisa ditimpa per layanan/bed (JSON). |
| `source_updated_at` | Wajib, RFC3339 UTC (`2026-10-20T08:00:00Z`), **waktu yang tertulis di sumber**, bukan waktu Anda menyalin. Tidak boleh di masa depan. Bisa ditimpa per layanan/bed. |
| `services[]` | `slug` salah satu master layanan (`icu`, `nicu`, `picu`, `igd-24h`, `ambulance`, `rawat-inap`), `status` `available\|limited\|unavailable\|unknown`. |
| `beds[]` | `category` `icu\|nicu\|picu\|general`, `total` dan `available` bilangan bulat >= 0 atau null; `available <= total` bila keduanya diisi. Entri dengan keduanya kosong ditolak: hapus saja. |

`data_origin` dan `verified_at` **bukan input**: berkas yang memuatnya ditolak. Validator selalu menetapkan `data_origin=curated` dan tidak pernah mengisi `verified_at`.

## Perilaku validator dan apply

- Satu record bermasalah menolak perintah; tidak ada penerapan sebagian dari berkas yang tidak valid. Exit non-zero.
- `0` dan `null` dijaga berbeda: `available: 0` berarti penuh, null berarti tidak diketahui.
- Apply hanya menambah. Layanan/bed yang sudah ada (termasuk hasil pembaruan operator) **tidak ditimpa** dan dilaporkan sebagai "tetap". `bpjs` hanya diisi bila masih null. Field RS dari MSI, origin tingkat RS, dan `version` item operator tidak disentuh. Item baru: origin `curated`, `version` 1, `verified_at` null, `synced_at` waktu apply.
- Mengulang apply tidak mengubah apa pun. Untuk mengoreksi data curated yang sudah masuk, gunakan alur pembaruan operator atau migration/skrip terpisah yang ditinjau; pipeline ini tidak menimpa.
- RS yang tidak ditemukan (belum di-sync dari MSI) dilaporkan dan membuat exit non-zero.
- Audit log belum ditulis oleh pipeline ini; simpan berkas kurasi yang diterapkan (versi, PIC, tanggal) sebagai catatan.

## Mencari sumber publik yang sah

1. Mulai dari RS yang sudah ada di database (`external_id` dari MSI); jangan menambah RS yang tidak ada di sana.
2. Utamakan sumber resmi yang dapat dibuka siapa saja tanpa login atau akal-akalan: situs resmi RS, halaman resmi dinas kesehatan, portal informasi fasilitas milik Kemenkes/BPJS Kesehatan, atau dokumen publik yang diterbitkan RS sendiri. Periksa ketentuan penggunaan dan lisensi sumber; jangan mengambil data dari balik login, hasil scraping yang dilarang, atau aplikasi pribadi.
3. Catat URL yang persis memuat fakta (bukan beranda) dan waktu pembaruan yang tertulis di sana. Jika sumber tidak mencantumkan waktu, jangan mengarang: fakta itu belum layak dikurasi (freshness akan `unknown`, dan kurasi mensyaratkan waktu).
4. Jangan memakai data pasien, tangkapan layar internal, pesan pribadi, atau informasi yang hanya diberikan lisan tanpa dokumen.
5. Fakta yang tidak ditemukan dibiarkan kosong (null). Jangan menebak "tidak ada" sebagai 0 atau `unavailable`.
6. Ketersediaan bed adalah data cepat basi. `available` hanya dikurasi bila sumber memuat angka berstempel waktu; kalau tidak, isi `total` saja atau tinggalkan kosong.

## Kriteria "10 profil lengkap"

Satu profil dihitung lengkap bila seluruhnya benar:

- RS ada di database dengan `external_id` MSI dan lokasi/alamat dasar dari MSI.
- `bpjs` terisi true/false dari sumber yang disebut (bukan null).
- Minimal 3 dari 5 layanan master punya status bukan `unknown`, tiap item dengan `source_url` dan `source_updated_at` sendiri atau warisan yang benar.
- Minimal 2 kategori bed dengan `total` terisi dari sumber; `available` hanya bila ada angka berstempel waktu.
- Seluruh `source_url` dapat dibuka dan memuat fakta yang diklaim.
- Checklist verifikasi manusia di bawah lulus untuk RS itu.

Profil simulasi tidak dihitung. Pipeline tidak mengisi `verified_at`; profil baru dianggap terverifikasi setelah manusia memeriksa dan pembaruan penanda itu dilakukan lewat jalur terpisah yang sah.

## Checklist verifikasi manusia (sebelum `--apply`)

- [ ] Dry-run tanpa error, `placeholder=false`, tidak ada `WARN` yang tidak dipahami.
- [ ] Setiap `source_url` dibuka manual hari ini; isi halaman cocok dengan nilai di berkas.
- [ ] `source_updated_at` sama dengan waktu yang tertulis di sumber (zona WIB diubah ke UTC dengan benar).
- [ ] Nama RS di berkas sama dengan RS yang dimaksud dan `external_id` cocok dengan MSI (isi kolom `name`).
- [ ] Tidak ada 0 yang sebenarnya "tidak diketahui", dan tidak ada null yang sebenarnya 0.
- [ ] Tidak ada data pribadi/pasien, tidak ada isi dari balik login, tidak ada fakta karangan.
- [ ] Database target benar dan bukan produksi bersama tanpa persetujuan; tersedia cadangan/catatan berkas.
- [ ] Pihak kedua meninjau berkas (four-eyes); catat nama peninjau dan tanggal di luar repo bila berisi keputusan internal.
- [ ] Setelah apply: periksa `GET /hospitals/{id}`: origin item `curated`, `verified_at` null, freshness sesuai usia sumber.
