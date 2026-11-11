package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/service"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type assistantChatRequest struct {
	Message        string  `json:"message"`
	ConversationID *string `json:"conversation_id"`
	Context        *struct {
		Lat *float64 `json:"lat"`
		Lng *float64 `json:"lng"`
	} `json:"context"`
}

type assistantAlternative struct {
	Recommendation domain.Recommendation `json:"recommendation"`
	ReasonCode     string                `json:"reason_code"`
	Note           string                `json:"note"`
}

type assistantChatData struct {
	Reply             string                  `json:"reply"`
	IsEmergency       bool                    `json:"is_emergency"`
	EmergencyKeywords []string                `json:"emergency_keywords"`
	ParsedQuery       *service.AssistantQuery `json:"parsed_query"`
	Recommendations   []domain.Recommendation `json:"recommendations"`
	Alternatives      []assistantAlternative  `json:"alternatives"`
}

type assistantCandidateReader interface {
	AssistantCandidates(context.Context, service.AssistantSearch) ([]domain.Hospital, bool, error)
}

func (s *Server) assistantChat(w http.ResponseWriter, r *http.Request) {
	if !s.rate(w, r, "assistant-ip:"+s.clientIP(r), 30, time.Minute) {
		return
	}
	var body assistantChatRequest
	if !decode(w, r, &body) {
		return
	}
	body.Message = strings.TrimSpace(body.Message)
	if body.Message == "" || utf8.RuneCountInString(body.Message) > 1000 {
		fail(w, r, 422, "validation_error", "message harus berisi 1 sampai 1000 karakter")
		return
	}
	if id := body.ConversationID; id != nil && (!validUUID(*id) || (*id)[14] != '4' || !strings.ContainsRune("89ab", rune((*id)[19]))) {
		fail(w, r, 422, "validation_error", "conversation_id harus berupa UUID v4 lowercase")
		return
	}
	if c := body.Context; c != nil && (c.Lat == nil || c.Lng == nil ||
		*c.Lat < -90 || *c.Lat > 90 || *c.Lng < -180 || *c.Lng > 180) {
		fail(w, r, 422, "validation_error", "context harus berisi pasangan lat dan lng yang valid")
		return
	}

	message := service.NewAssistantMessage(body.Message)
	data := assistantChatData{
		EmergencyKeywords: message.EmergencyKeywords(),
		Recommendations:   []domain.Recommendation{},
		Alternatives:      []assistantAlternative{},
	}
	truncated := false
	model := "rule-nlp-v1"
	respond := func(reply string) {
		if truncated {
			reply += " Pencarian mencapai batas jumlah kandidat. Hasil hanya mencakup kandidat yang sempat diperiksa."
		}
		data.Reply = reply
		send(w, 200, map[string]any{
			"data": data,
			"meta": map[string]any{"model": model, "candidates_truncated": truncated},
		})
	}
	if len(data.EmergencyKeywords) > 0 {
		data.IsEmergency = true
		respond("Peringatan gawat darurat. Segera hubungi 119 untuk bantuan medis atau 112 jika tersedia di daerah Anda. Segera cari pertolongan di IGD terdekat.")
		return
	}
	if message.MedicalAdvice() {
		respond("Silakan hubungi tenaga kesehatan atau fasilitas kesehatan untuk pemeriksaan dan saran pengobatan.")
		return
	}
	const scopeReply = "Saya bisa membantu mencari rumah sakit berdasarkan layanan, wilayah, dan BPJS. " + service.AssistantExample
	if message.UnrelatedRequest() {
		respond(scopeReply)
		return
	}
	if guide := message.SiteGuide(); guide != "" {
		respond(guide)
		return
	}

	ctx := r.Context()
	masters, err := s.repo.Services(ctx)
	if err != nil {
		s.dbError(w, r, err)
		return
	}
	parsed := message.Query(masters)
	if len(parsed.Services) == 0 && !message.HospitalSearch() {
		respond(scopeReply)
		return
	}
	data.ParsedQuery = &parsed
	dir, ok := s.repo.(interface {
		Regions(context.Context) ([]domain.Region, error)
	})
	if !ok {
		fail(w, r, 501, "not_implemented", "Master wilayah belum tersedia")
		return
	}
	regions, err := dir.Regions(ctx)
	if err != nil {
		s.dbError(w, r, err)
		return
	}
	if choices := message.ResolveRegion(&parsed, regions); len(choices) > 0 {
		respond("Mohon pilih wilayah yang dimaksud (" + strings.Join(choices, " atau ") + ") dan kirim ulang permintaan lengkap. " + service.AssistantExample)
		return
	}
	useCoordinates := parsed.Location == nil && body.Context != nil && !message.MentionsPlace()
	missingLocation := parsed.Location == nil && !useCoordinates
	if len(parsed.Services) == 0 || missingLocation {
		switch {
		case len(parsed.Services) == 0 && missingLocation:
			respond("Sebutkan layanan yang dibutuhkan dan wilayah pencarian. " + service.AssistantExample)
		case len(parsed.Services) == 0:
			respond("Sebutkan layanan yang dibutuhkan, misalnya NICU atau IGD. " + service.AssistantExample)
		default:
			respond("Sebutkan kota, kabupaten, atau provinsi yang tersedia di SiagaRS. " + service.AssistantExample)
		}
		return
	}
	if len(parsed.Services) > 6 {
		respond("Pilih maksimal 6 layanan dalam satu pencarian. " + service.AssistantExample)
		return
	}
	if hints, needsClarification := message.FilterHints(masters, parsed.Services); needsClarification {
		services := s.interpretAssistantServices(r, hints)
		if len(services) == 0 {
			parsed.Services = []string{}
			respond("Mohon sebutkan hanya layanan yang ingin dicari dan kirim ulang permintaan lengkap. " + service.AssistantExample)
			return
		}
		parsed.Services = services
		model = "openrouter-filter-v1"
	}

	params := url.Values{"services": {strings.Join(parsed.Services, ",")}}
	if parsed.ProvinceCode != nil {
		params.Set("province_code", *parsed.ProvinceCode)
	}
	if parsed.CityCode != nil {
		params.Set("city_code", *parsed.CityCode)
	}
	if useCoordinates {
		params.Set("lat", strconv.FormatFloat(*body.Context.Lat, 'g', -1, 64))
		params.Set("lng", strconv.FormatFloat(*body.Context.Lng, 'g', -1, 64))
		label := "Sekitar lokasi Anda (20 km)"
		parsed.Location = &label
	}
	if parsed.BPJS == "wajib" {
		params.Set("bpjs", "required")
	}
	query, err := service.ParseRecommend(params)
	if err != nil {
		s.dbError(w, r, err)
		return
	}
	query.ServiceNames = make(map[string]string, len(masters))
	for _, entry := range masters {
		query.ServiceNames[entry.Slug] = entry.Name
	}
	now := time.Now().UTC()
	var results []domain.Recommendation
	results, truncated, err = s.assistantRecommendations(ctx, service.AssistantSearch{RecommendQuery: query}, now)
	if err != nil {
		s.dbError(w, r, err)
		return
	}
	const confirm = " Hubungi rumah sakit untuk memastikan informasi terkini."
	if len(results) > 0 {
		data.Recommendations = results
		respond(fmt.Sprintf("Saya menampilkan %d rumah sakit sesuai filter Anda (%s).", len(results), *parsed.Location) + confirm)
		return
	}
	if query.BPJSRequired {
		results, alternativesTruncated, err := s.assistantRecommendations(ctx, service.AssistantSearch{
			RecommendQuery: query, Alternatives: true,
		}, now)
		if err != nil {
			s.dbError(w, r, err)
			return
		}
		truncated = truncated || alternativesTruncated
		for _, result := range results {
			alternative := assistantAlternative{
				Recommendation: result,
				ReasonCode:     "bpjs_unknown",
				Note:           "Status BPJS belum diketahui, konfirmasi ke RS.",
			}
			if result.Hospital.BPJS != nil && !*result.Hospital.BPJS {
				alternative.ReasonCode, alternative.Note = "bpjs_not_accepted", "BPJS tidak diterima."
			}
			data.Alternatives = append(data.Alternatives, alternative)
		}
		if len(data.Alternatives) > 0 {
			respond(fmt.Sprintf("Pencarian ini belum menemukan RS yang memenuhi syarat BPJS. Berikut %d alternatif dengan layanan dan wilayah yang sama (%s).", len(data.Alternatives), *parsed.Location) + confirm)
			return
		}
	}
	respond("Pencarian ini belum menemukan rumah sakit sesuai filter Anda. Coba kirim ulang permintaan dengan wilayah yang lebih luas.")
}

func (s *Server) assistantRecommendations(ctx context.Context, search service.AssistantSearch, now time.Time) ([]domain.Recommendation, bool, error) {
	var candidates []domain.Hospital
	var truncated bool
	var err error
	if reader, ok := s.repo.(assistantCandidateReader); ok {
		candidates, truncated, err = reader.AssistantCandidates(ctx, search)
	} else {
		var all []domain.Hospital
		if all, err = s.repo.Hospitals(ctx); err == nil {
			candidates, truncated = service.AssistantCandidates(all, search)
		}
	}
	if err != nil {
		return nil, false, err
	}
	for i := range candidates {
		candidates[i].BPJS = service.AssistantBPJS(candidates[i])
	}
	request := search.RecommendRequest
	if search.Alternatives {
		request.BPJSRequired = false
	}
	return domain.Recommend(candidates, request, now).Recommendations, truncated, nil
}
