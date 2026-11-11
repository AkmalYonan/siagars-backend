package service

import (
	"siagars/backend/internal/domain"
	"slices"
	"strings"
	"unicode"
)

// ModelQuestion rejects likely personal or clinical inputs before the conversational fallback.
func (m AssistantMessage) ModelQuestion(original string) (string, bool) {
	if strings.ContainsAny(original, "@/\\<>") || strings.ContainsFunc(original, unicode.IsDigit) ||
		m.has("nama", "bernama", "alamat", "nik", "nomor", "kode", "tanggal lahir", "rekam medis", "pasien",
			"keluhan", "gejala", "saya sakit", "anak saya", "ibu saya", "ayah saya", "demam", "nyeri", "sesak", "muntah", "batuk") {
		return "", false
	}
	question := strings.Join(strings.Fields(original), " ")
	return question, question != ""
}

func (m AssistantMessage) IsGreeting() bool {
	switch strings.TrimSpace(m.text) {
	case "halo", "halo siagars", "hai", "hai siagars", "hi", "hei", "halo kak", "hai kak", "selamat pagi", "selamat siang", "selamat sore", "selamat malam":
		return true
	default:
		return false
	}
}

func (m AssistantMessage) UnrelatedRequest() bool {
	return m.has("buat puisi", "buatkan puisi", "tulis puisi", "tuliskan puisi",
		"buat kode", "buatkan kode", "tulis kode", "buat cerita", "buatkan cerita", "tulis cerita")
}

// SiteGuide returns only backend-authored guidance for recognized SiagaRS topics.
func (m AssistantMessage) SiteGuide() string {
	if m.has("arti skor", "cara membaca skor", "kenapa skor", "skor rekomendasi", "cara membaca hasil", "kualitas data", "sumber data") {
		return "Skor menunjukkan kecocokan dengan filter dan kepastian data. Buka rincian kartu untuk melihat komponen skor dan data yang belum diketahui. Lihat Tentang Data di /tentang-data."
	}
	if !m.has("cara", "gimana", "bagaimana", "di mana", "dimana", "panduan") {
		return ""
	}
	switch {
	case m.has("cek bpjs", "periksa bpjs", "cek kepesertaan bpjs"):
		return "Buka halaman Cek BPJS di /cek-bpjs untuk menggunakan fitur simulasi SiagaRS."
	case m.has("lacak rujukan", "lacak status rujukan", "cek status rujukan", "tracking rujukan"):
		return "Buka Lacak Rujukan di /lacak-rujukan dan masukkan kode pelacakan di halaman tersebut."
	case m.has("bandingkan", "membandingkan", "perbandingan"):
		return "Buka Bandingkan di /bandingkan untuk melihat layanan, BPJS, dan kelengkapan data antar rumah sakit."
	case m.has("filter", "cari rs", "cari rumah sakit", "mencari rs", "mencari rumah sakit", "pilih rs"):
		return "Buka Pilih RS di /pilih-rs untuk memilih layanan, wilayah, dan preferensi BPJS. Daftar rumah sakit tersedia di /rumah-sakit."
	}
	return ""
}

type AssistantHints struct {
	Terms              []string `json:"terms"`
	ServiceIDs         []string `json:"service_ids"`
	ExcludedServiceIDs []string `json:"excluded_service_ids,omitempty"`
}

// FilterHints rebuilds the input from canonical service IDs and fixed cue words.
// It only handles conflicting service mentions. Location and BPJS stay local.
func (m AssistantMessage) FilterHints(masters []domain.Service, selected []string) (AssistantHints, bool) {
	if len(selected) == 0 || len(selected) > 6 {
		return AssistantHints{}, false
	}
	negations := []string{"tidak perlu", "nggak perlu", "gak perlu", "ga perlu", "tak perlu", "bukan", "jangan", "tanpa"}
	if !m.has(negations...) {
		return AssistantHints{}, false
	}

	terms := map[string]string{
		"yang saya cari": "yang saya cari", "yang dibutuhkan": "yang dibutuhkan",
		"butuh": "butuh", "perlu": "perlu", "melainkan": "melainkan", "dan": "dan", "atau": "atau",
	}
	for _, phrase := range negations {
		terms[phrase] = phrase
	}
	serviceTerms := map[string]string{}
	for _, entry := range masters {
		// Keep the outbound vocabulary to the services shipped with SiagaRS.
		aliases, allowed := assistantServiceAliases[entry.Slug]
		if !allowed || !slices.Contains(selected, entry.Slug) {
			continue
		}
		for _, phrase := range append([]string{normalizeAssistantText(entry.Slug), normalizeAssistantText(entry.Name)}, aliases...) {
			if phrase != "" {
				serviceTerms[phrase] = entry.Slug
			}
		}
	}
	conflictingService := false
	var excluded []string
	for phrase, slug := range serviceTerms {
		terms[phrase] = slug
		for _, negation := range negations {
			if m.has(negation+" "+phrase) ||
				(strings.HasSuffix(negation, "perlu") && m.has(phrase+" "+negation) && !m.has(phrase+" "+negation+" bpjs")) {
				conflictingService = true
			}
			if m.has(negation+" "+phrase) ||
				(strings.HasSuffix(negation, "perlu") && m.has(phrase+" "+negation+" yang")) {
				excluded = append(excluded, slug)
			}
		}
	}
	if !conflictingService {
		return AssistantHints{}, false
	}
	if len(selected) == 1 {
		return AssistantHints{}, true
	}
	for _, slug := range selected {
		if _, allowed := assistantServiceAliases[slug]; !allowed {
			return AssistantHints{}, true
		}
	}
	var phrases []string
	for phrase := range terms {
		phrases = append(phrases, phrase)
	}
	slices.SortFunc(phrases, func(a, b string) int { return len(b) - len(a) })
	slices.Sort(excluded)
	hints := AssistantHints{ServiceIDs: selected, ExcludedServiceIDs: slices.Compact(excluded)}
	for rest := strings.TrimSpace(m.text); rest != ""; {
		start := rest
		for _, phrase := range phrases {
			after, found := strings.CutPrefix(rest, phrase)
			if found && (after == "" || after[0] == ' ') {
				hints.Terms = append(hints.Terms, terms[phrase])
				rest = strings.TrimSpace(after)
				break
			}
		}
		if rest == start {
			_, rest, _ = strings.Cut(rest, " ")
		}
		if len(hints.Terms) > 24 {
			return AssistantHints{}, true
		}
	}
	return hints, true
}
