package httpapi

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strings"

	"siagars/backend/internal/service"
)

const assistantGreeting = "Halo! Saya asisten SiagaRS. Mau cari rumah sakit, membandingkan pilihan, atau butuh bantuan memakai fitur SiagaRS?"
const assistantScopeReply = "Saya bisa membantu mencari rumah sakit berdasarkan layanan, wilayah, dan BPJS. " + service.AssistantExample

var assistantPassages = map[string]string{
	"greeting": assistantGreeting,
	"outside":  assistantScopeReply,
	"search":   "Untuk mencari rumah sakit, sebutkan kota dan layanan yang kamu butuhkan. Kalau ingin memfilter BPJS, sebutkan juga di pesan. Kamu bisa memakai formulir di /pilih-rs.",
	"compare":  "Buka /bandingkan untuk melihat perbedaan layanan, BPJS, dan kelengkapan data antar rumah sakit.",
	"referral": "Buka /lacak-rujukan dan masukkan kode pelacakan langsung di halaman tersebut untuk melihat status rujukan.",
	"bpjs":     "Buka /cek-bpjs untuk mencoba fitur simulasi cek kepesertaan BPJS SiagaRS.",
	"data":     "Skor rekomendasi mengukur kecocokan dengan filter dan kepastian data. Lihat rincian skor dan status data pada kartu rumah sakit atau buka /tentang-data.",
}

type assistantAnswerer interface {
	Answer(context.Context, string, map[string]string) ([]string, error)
}

func (s *Server) assistantAnswer(r *http.Request, question string) string {
	answerer, ok := s.assistantLLM.(assistantAnswerer)
	if !ok {
		return ""
	}
	var reply string
	s.withAssistantLLM(r, func(ctx context.Context) string {
		ids, err := answerer.Answer(ctx, question, maps.Clone(assistantPassages))
		if err != nil {
			return err.Error()
		}
		if len(ids) == 0 || len(ids) > 2 || len(ids) == 2 && (ids[0] == ids[1] || slices.Contains(ids, "outside")) {
			return "invalid_answer"
		}
		passages := make([]string, 0, len(ids))
		for _, id := range ids {
			passage, ok := assistantPassages[id]
			if !ok {
				return "invalid_answer"
			}
			passages = append(passages, passage)
		}
		reply = strings.Join(passages, "\n\n")
		return "answered"
	})
	return reply
}
