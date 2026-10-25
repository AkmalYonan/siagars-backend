package seed

import "siagars/backend/internal/repository"

func str(s string) *string { return &s }

// BPJSDemoMembers are fictional participants for the BPJS membership simulation. Their NIKs start with
// 9999, a region code that does not exist, so none can belong to a real person. Share these with the
// frontend for demos and tests.
var BPJSDemoMembers = []repository.BPJSMemberDoc{
	{NIK: "9999010101900001", Name: "Budi Santoso", BirthDate: "1990-01-01", Status: "active", MembershipCode: str("PBPU"), MembershipName: str("Pekerja Bukan Penerima Upah"), CareClass: str("3"), FacilityCode: str("SIM-FKTP-001"), FacilityName: str("Puskesmas Simulasi Kebayoran")},
	{NIK: "9999020202850002", Name: "Siti Aminah", BirthDate: "1985-02-02", Status: "inactive", InactiveReason: str("Tunggakan iuran 3 bulan"), MembershipCode: str("PBPU"), MembershipName: str("Pekerja Bukan Penerima Upah"), CareClass: str("2"), FacilityCode: str("SIM-FKTP-002"), FacilityName: str("Klinik Simulasi Sehat")},
	{NIK: "9999030303950003", Name: "Agus Pratama", BirthDate: "1995-03-03", Status: "active", MembershipCode: str("PPU"), MembershipName: str("Pekerja Penerima Upah")},
	{NIK: "9999040404700004", Name: "Ratna Dewi", BirthDate: "1970-04-04", Status: "active", MembershipCode: str("PBI"), MembershipName: str("Penerima Bantuan Iuran"), CareClass: str("3"), FacilityCode: str("SIM-FKTP-001"), FacilityName: str("Puskesmas Simulasi Kebayoran")},
	{NIK: "9999050505880005", Name: "José Álvarez-Núñez", BirthDate: "1988-05-05", Status: "active", MembershipCode: str("PPU"), MembershipName: str("Pekerja Penerima Upah"), CareClass: str("1"), FacilityCode: str("SIM-FKTP-003"), FacilityName: str("Dokter Keluarga Simulasi")},
}
