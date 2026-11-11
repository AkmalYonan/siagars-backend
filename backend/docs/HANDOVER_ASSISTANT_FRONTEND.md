# Handover frontend · Asisten pencarian RS (BE-52)

Untuk integrasi halaman `/asisten` SiagaRS. Gunakan kontrak dan fixture di repo backend. [OpenAPI](openapi.yaml) · [Fixture](fixtures.json)

## Request

Panggil `POST /api/v1/assistant/chat` melalui rewrite pada origin frontend. Target production rewrite adalah `https://siagars-backend.vercel.app`. Endpoint publik dan menerima `Content-Type: application/json`.

```json
{
  "message": "Cari RS di Kota Bekasi yang punya NICU dan menerima BPJS",
  "context": { "lat": -6.2383, "lng": 106.9756 }
}
```

- `message` wajib dan dibatasi 1–1.000 karakter setelah trim. Kirim pasangan `lat`/`lng` jika tersedia dan pengguna mengizinkan. Backend mendahulukan wilayah yang disebut dalam pesan, lalu memakai radius 20 km jika lokasi hanya berasal dari koordinat.
- `conversation_id` opsional berupa UUID v4 lowercase. Backend memproses setiap pesan secara mandiri. Jika pengguna memperjelas wilayah, kirim ulang pencarian lengkap dengan layanan dan preferensi BPJS.
- Rate limit 30 request per menit per IP. Untuk `429`, tampilkan pesan coba lagi dan hormati `Retry-After`. Galat mengikuti `{ "error": { "code": "...", "message": "..." }, "request_id": "..." }`.

## Render respons

1. Untuk setiap `200`, tampilkan `data.reply` sebagai teks. Darurat, klarifikasi, dan hasil kosong sudah merupakan balasan akhir. Pertahankan CTA darurat saat API gagal.
2. Jika `data.is_emergency` bernilai `true`, tampilkan peringatan dan CTA panggilan `119` atau `112` jika tersedia di daerah pengguna. `emergency_keywords` memuat frasa yang terdeteksi. [PSC 119](https://kemkes.go.id/id/layanan/psc-119) · [Layanan 112](https://layanan112.komdigi.go.id/tentang)
3. Render `data.recommendations` menggunakan kartu `Recommendation` yang sama dengan `GET /api/v1/recommendations`. Pertahankan penanda kualitas dan provenance. Jika `data.alternatives` berisi item, tampilkan judul **Alternatif**, kartu dari `item.recommendation`, dan `item.note` pada setiap kartu. Kode alasan adalah `bpjs_not_accepted` atau `bpjs_unknown`.
4. `parsed_query` bisa `null`, termasuk untuk panduan fitur. Jika ada, `bpjs` berisi `wajib` atau `any`, `dataset` berisi `real`, dan kode wilayah atau label bisa `null`. Terima `meta.model` bernilai `rule-nlp-v1`, `openrouter-filter-v1`, atau `openrouter-answer-v1`. Jika `meta.candidates_truncated` bernilai `true`, gunakan balasan server untuk menjelaskan batas pencarian.

Untuk sapaan dan pertanyaan yang belum dikenali, OpenRouter dapat memilih dan merangkai bagian jawaban dari panduan SiagaRS. Gunakan `data.reply` untuk ketiga jalur. Hindari memasukkan data pribadi atau medis dalam pesan karena pertanyaan yang lolos penyaringan dapat dikirim ke provider.

## Uji integrasi

| Skenario | Hasil UI |
|---|---|
| “Cari RS di Kota Bekasi yang punya NICU dan menerima BPJS” | Filter NICU, Kota Bekasi (`3275`), BPJS wajib. Render kartu sesuai data yang tersedia. |
| “Cari NICU di Bekasi” saat master memuat kota dan kabupaten | Balasan meminta pengguna memilih wilayah dan mengirim ulang permintaan lengkap. |
| Data NICU tersedia, BPJS ditolak atau belum diketahui, dan hasil utama kosong | Bagian **Alternatif** dengan status BPJS per kartu. Gunakan fixture uji dengan kondisi ini. |
| “Cari RS di Kota Bekasi, pasien tidak sadar” | Peringatan dan CTA bantuan medis tampil segera. |
| “Apa obat untuk demam?” | Balasan mengarahkan ke tenaga kesehatan tanpa memberi obat atau dosis. |
| API gagal atau kuota habis | Tampilkan galat atau instruksi coba lagi. CTA darurat tetap tersedia. |
