package service

import (
	"cmp"
	"siagars/backend/internal/domain"
	"slices"
	"strings"
	"unicode"
)

const AssistantExample = "Contoh: Cari RS di Kota Bekasi yang punya NICU dan menerima BPJS."

type AssistantQuery struct {
	Services     []string `json:"services"`
	ProvinceCode *string  `json:"province_code"`
	CityCode     *string  `json:"city_code"`
	Location     *string  `json:"location_label"`
	BPJS         string   `json:"bpjs"`
	Dataset      string   `json:"dataset"`
}

type AssistantMessage struct {
	text string
}

func NewAssistantMessage(message string) AssistantMessage {
	return AssistantMessage{text: " " + normalizeAssistantText(message) + " "}
}

func normalizeAssistantText(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), " ")
}

func (m AssistantMessage) has(phrases ...string) bool {
	for _, phrase := range phrases {
		if phrase != "" && strings.Contains(m.text, " "+phrase+" ") {
			return true
		}
	}
	return false
}

func (m AssistantMessage) EmergencyKeywords() []string {
	out := []string{}
	for _, group := range [][]string{
		{"tidak sadar", "nggak sadar", "gak sadar", "tak sadar", "tidak sadarkan diri"},
		{"kejang", "kejang kejang"},
		{"henti napas", "henti nafas", "tidak bernapas", "tidak bernafas", "berhenti bernapas", "berhenti bernafas"},
		{"perdarahan hebat", "pendarahan hebat"},
		{"koma"},
		{"batuk darah"},
	} {
		if m.has(group...) {
			out = append(out, group[0])
		}
	}
	return out
}

func (m AssistantMessage) MedicalAdvice() bool {
	return m.has("diagnosa", "diagnosis", "diagnosisnya", "diagnosanya", "sakit apa", "penyakit apa",
		"obat apa", "apa obat", "obat untuk", "obatnya", "resep obat", "minum obat", "dosis",
		"cara mengobati", "cara menyembuhkan")
}

func (m AssistantMessage) HospitalSearch() bool {
	return m.has("rs", "rumah sakit", "rsud", "rsup", "faskes", "layanan", "bpjs") &&
		m.has("cari", "carikan", "mencari", "butuh", "perlu", "rekomendasi", "terdekat", "rs di", "rumah sakit di")
}

var assistantServiceAliases = map[string][]string{
	"icu":        {"perawatan intensif dewasa"},
	"nicu":       {"perawatan intensif bayi", "perawatan intensif neonatal", "intensif bayi baru lahir"},
	"picu":       {"perawatan intensif anak", "intensif pediatrik"},
	"igd-24h":    {"igd", "ugd", "igd 24 jam", "unit gawat darurat", "instalasi gawat darurat"},
	"ambulance":  {"ambulans", "ambulan"},
	"rawat-inap": {"rawat inap", "opname"},
}

func (m AssistantMessage) Query(masters []domain.Service) AssistantQuery {
	query := AssistantQuery{Services: []string{}, BPJS: "any", Dataset: "real"}
	for _, entry := range masters {
		if m.has(normalizeAssistantText(entry.Name), normalizeAssistantText(entry.Slug)) ||
			m.has(assistantServiceAliases[entry.Slug]...) {
			query.Services = append(query.Services, entry.Slug)
		}
	}
	slices.Sort(query.Services)
	query.Services = slices.Compact(query.Services)
	if m.has("bpjs") && !m.has("tanpa bpjs", "non bpjs", "tidak perlu bpjs", "bpjs opsional", "bpjs tidak wajib", "bpjs tidak harus") {
		query.BPJS = "wajib"
	}
	return query
}

type regionMatch struct {
	region     domain.Region
	start, end int
}

func regionNames(region domain.Region) []string {
	name := normalizeAssistantText(region.Name)
	names := []string{name}
	if region.Kind == "city" {
		bare := name
		for _, prefix := range []string{"kota administrasi ", "kota adm ", "kabupaten ", "kab ", "kota "} {
			if stripped, found := strings.CutPrefix(name, prefix); found {
				bare = stripped
				break
			}
		}
		names = append(names, bare)
		if strings.HasPrefix(name, "kabupaten ") {
			names = append(names, "kab "+bare)
		}
	}
	if name == "dki jakarta" {
		names = append(names, "jakarta", "dki")
	}
	return names
}

// ResolveRegion fills codes only for an unambiguous region and returns labels for ambiguous matches.
func (m AssistantMessage) ResolveRegion(query *AssistantQuery, regions []domain.Region) []string {
	var matches []regionMatch
	provinceNames := map[string]string{}
	for _, region := range regions {
		if region.Kind == "province" {
			provinceNames[region.Code] = region.Name
		}
		for _, name := range regionNames(region) {
			for offset := 0; name != "" && offset < len(m.text); {
				start := strings.Index(m.text[offset:], " "+name+" ")
				if start < 0 {
					break
				}
				start += offset
				offset = start + 1
				if region.Kind == "city" && name != normalizeAssistantText(region.Name) {
					before := m.text[:offset]
					if strings.HasSuffix(before, " kota ") || strings.HasSuffix(before, " kabupaten ") || strings.HasSuffix(before, " kab ") {
						continue
					}
				}
				matches = append(matches, regionMatch{region, start, start + len(name) + 1})
			}
		}
	}
	provinces, cities := map[string]domain.Region{}, map[string]domain.Region{}
	for _, match := range matches {
		contained := slices.ContainsFunc(matches, func(other regionMatch) bool {
			return other.start <= match.start && other.end >= match.end &&
				(other.start < match.start || other.end > match.end)
		})
		if contained {
			continue
		}
		if match.region.Kind == "province" {
			provinces[match.region.Code] = match.region
		} else if match.region.Kind == "city" {
			cities[match.region.Code] = match.region
		}
	}
	var labels []string
	for _, city := range cities {
		if len(provinces) > 0 {
			if city.ParentCode == nil || provinces[*city.ParentCode].Code == "" {
				continue
			}
		}
		label := city.Name
		if city.ParentCode != nil && provinceNames[*city.ParentCode] != "" {
			label += ", " + provinceNames[*city.ParentCode]
		}
		labels = append(labels, label)
		if len(cities) == 1 && len(provinces) <= 1 {
			query.CityCode, query.ProvinceCode, query.Location = &city.Code, city.ParentCode, &label
			return nil
		}
	}
	if len(labels) == 1 && len(provinces) == 1 {
		for _, city := range cities {
			if city.ParentCode != nil && provinces[*city.ParentCode].Code != "" {
				query.CityCode, query.ProvinceCode, query.Location = &city.Code, city.ParentCode, &labels[0]
				return nil
			}
		}
	}
	if len(cities) == 0 && len(provinces) == 1 {
		for _, province := range provinces {
			query.ProvinceCode, query.Location = &province.Code, &province.Name
		}
		return nil
	}
	if len(labels) == 0 || len(provinces) > 1 {
		for _, province := range provinces {
			labels = append(labels, province.Name)
		}
	}
	if len(labels) < 2 && len(cities) > 0 {
		for _, city := range cities {
			labels = append(labels, city.Name)
		}
	}
	slices.Sort(labels)
	return slices.Compact(labels)
}

// MentionsPlace prevents a fallback to GPS when the message names an unrecognized area.
func (m AssistantMessage) MentionsPlace() bool {
	return m.has("kota", "kabupaten", "kab", "provinsi") ||
		(m.has("di") && !m.has("di sini", "di dekat saya", "di sekitar saya", "di sekitar sini", "di lokasi saya", "di sekitar lokasi saya"))
}

// AssistantSearch filters to documented availability before applying the candidate cap.
type AssistantSearch struct {
	RecommendQuery
	Alternatives bool
}

// AssistantBPJS treats legacy values without a recorded origin and demo data as unknown.
func AssistantBPJS(h domain.Hospital) *bool {
	if h.BPJSDataOrigin == nil || *h.BPJSDataOrigin == "" || *h.BPJSDataOrigin == "simulation" {
		return nil
	}
	return h.BPJS
}

// AssistantCandidates mirrors the Mongo query for stores without candidate pushdown.
func AssistantCandidates(hospitals []domain.Hospital, q AssistantSearch) ([]domain.Hospital, bool) {
	out := []domain.Hospital{}
	for _, h := range hospitals {
		if h.Operational != nil && !*h.Operational || h.DataOrigin == "simulation" ||
			q.Province != "" && h.ProvinceCode != q.Province || q.City != "" && h.CityCode != q.City ||
			q.Area != nil && !q.Area.Contains(h) {
			continue
		}
		bpjs := AssistantBPJS(h)
		accepted := bpjs != nil && *bpjs
		if q.BPJSRequired && accepted == q.Alternatives {
			continue
		}
		allAvailable := true
		for _, slug := range q.Services {
			if !slices.ContainsFunc(h.Services, func(s domain.Service) bool {
				return s.Slug == slug && s.Status == "available" && s.DataOrigin != "" && s.DataOrigin != "simulation"
			}) {
				allAvailable = false
				break
			}
		}
		if allAvailable {
			out = append(out, h)
		}
	}
	slices.SortFunc(out, func(a, b domain.Hospital) int { return cmp.Compare(a.ID, b.ID) })
	if len(out) > MaxCandidates {
		return out[:MaxCandidates], true
	}
	return out, false
}
