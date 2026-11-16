package erfassung

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"steuer/internal/store"
)

func neuerDienst(t *testing.T) (*Dienst, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Eingang"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return Neu(dir, s, 2024), s, dir
}

func TestPDFMitTextebene(t *testing.T) {
	dir := t.TempDir()
	pfad := filepath.Join(dir, "lstb.pdf")
	schreibe(t, pfad, erzeugePDF(textLohnsteuer))
	e, err := Analysiere(pfad, 2020)
	if err != nil {
		t.Fatal(err)
	}
	if e.Beleg.Art != ArtLohnsteuerbescheinigung || e.Beleg.Steuerjahr != 2024 {
		t.Fatalf("Beleg: art=%q jahr=%d hinweis=%q\n%s", e.Beleg.Art, e.Beleg.Steuerjahr, e.Beleg.Hinweis, e.Beleg.Text)
	}
	got := posMap(e)
	if got["lstb.3"].BetragCent != 4825000 || got["lstb.25"].BetragCent != 394013 || got["lstb.4"].BetragCent != 712300 {
		t.Errorf("Positionen: %+v\nText:\n%s", e.Positionen, e.Beleg.Text)
	}
	if e.Beleg.SHA256 == "" || e.Beleg.Datei != "lstb.pdf" {
		t.Errorf("Beleg: %+v", e.Beleg)
	}
}

func TestPDFUmlauteUndEuro(t *testing.T) {
	dir := t.TempDir()
	pfad := filepath.Join(dir, "r.pdf")
	schreibe(t, pfad, erzeugePDF(textHandwerker))
	e, err := Analysiere(pfad, 2020)
	if err != nil {
		t.Fatal(err)
	}
	if e.Beleg.Art != ArtHandwerker || e.Beleg.Belegdatum != "2024-03-12" || e.Beleg.BetragCent != 80500 {
		t.Errorf("Beleg: %+v\n%s", e.Beleg, e.Beleg.Text)
	}
}

func TestPDFOhneTextebene(t *testing.T) {
	if TexterkennungVerfuegbar() {
		t.Skip("Tesseract vorhanden")
	}
	dir := t.TempDir()
	pfad := filepath.Join(dir, "scan.pdf")
	schreibe(t, pfad, erzeugePDF(nil))
	e, err := Analysiere(pfad, 2023)
	if err != nil {
		t.Fatal(err)
	}
	if e.Beleg.Hinweis != "Texterkennung nicht verfügbar - bitte manuell erfassen" || e.Beleg.Steuerjahr != 2023 || e.Beleg.Art != ArtSonstiges {
		t.Errorf("Beleg: %+v", e.Beleg)
	}
}

func TestBildOhneTesseract(t *testing.T) {
	alt := tesseractSuchen
	tesseractSuchen = func() string { return "" }
	defer func() { tesseractSuchen = alt }()
	pfad := filepath.Join(t.TempDir(), "foto.jpg")
	schreibe(t, pfad, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0})
	e, err := Analysiere(pfad, 2024)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.Beleg.Hinweis, "Texterkennung nicht verfügbar") {
		t.Errorf("Hinweis: %q", e.Beleg.Hinweis)
	}
}

func TestDefektesPDF(t *testing.T) {
	pfad := filepath.Join(t.TempDir(), "kaputt.pdf")
	schreibe(t, pfad, []byte("%PDF-1.4 das ist kein PDF"))
	e, err := Analysiere(pfad, 2024)
	if err != nil {
		t.Fatal(err)
	}
	if e.Beleg.Hinweis == "" {
		t.Error("Hinweis erwartet")
	}
}

func TestTextDateienLatin1(t *testing.T) {
	pfad := filepath.Join(t.TempDir(), "r.csv")
	// Windows-1252: "Gesamtbetrag;1.190,00 €" mit Ä
	schreibe(t, pfad, []byte("Rechnung\n\xc4rztliche Leistung; Rechnungsnummer 1\nGesamtbetrag;1.190,00 \x80\n"))
	e, err := Analysiere(pfad, 2024)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.Beleg.Text, "Ärztliche") || e.Beleg.BetragCent != 119000 {
		t.Errorf("Beleg: %+v", e.Beleg)
	}
}

func TestDurchlauf(t *testing.T) {
	d, s, dir := neuerDienst(t)
	ein := filepath.Join(dir, "Eingang")

	// 1. Lohnsteuerbescheinigung als PDF, 2. Spende als Text, 3. gerade erst geschrieben, 4. Hilfsdatei
	schreibe(t, filepath.Join(ein, "lstb.pdf"), erzeugePDF(textLohnsteuer))
	schreibe(t, filepath.Join(ein, "spende.txt"), []byte(strings.Join(textSpende, "\n")))
	schreibe(t, filepath.Join(ein, "neu.txt"), []byte("Rechnung\nGesamtbetrag 5,00 EUR"))
	schreibe(t, filepath.Join(ein, "download.crdownload"), []byte("x"))
	alt(t, filepath.Join(ein, "lstb.pdf"))
	alt(t, filepath.Join(ein, "spende.txt"))
	alt(t, filepath.Join(ein, "download.crdownload"))

	neu, fehler := d.Durchlauf()
	if neu != 2 || len(fehler) != 0 {
		t.Fatalf("neu=%d fehler=%v", neu, fehler)
	}
	for _, n := range []string{"neu.txt", "download.crdownload"} {
		if _, err := os.Stat(filepath.Join(ein, n)); err != nil {
			t.Errorf("%s soll im Eingang bleiben: %v", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(ein, "lstb.pdf")); err == nil {
		t.Error("lstb.pdf hätte verschoben werden müssen")
	}

	b24, _ := s.Belege(2024)
	b25, _ := s.Belege(2025)
	if len(b24) != 1 || len(b25) != 1 {
		t.Fatalf("Belege 2024=%d 2025=%d", len(b24), len(b25))
	}
	if b25[0].Datei != "Belege/2025/20250120_spendenbescheinigung (Tierschutzverein Beispielstadt e.V.).txt" {
		t.Errorf("Datei = %q", b25[0].Datei)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(b25[0].Datei))); err != nil {
		t.Errorf("Ablagedatei fehlt: %v", err)
	}
	if !strings.HasPrefix(b24[0].Datei, "Belege/2024/") || !strings.Contains(b24[0].Datei, "lohnsteuerbescheinigung (Muster Maschinenbau GmbH)") {
		t.Errorf("Datei = %q", b24[0].Datei)
	}
	pos, _ := s.Positionen(2024)
	if len(pos) != 9 {
		t.Errorf("Positionen 2024: %d", len(pos))
	}
	for _, p := range pos {
		if p.BelegID == nil || *p.BelegID != b24[0].ID || !p.Vorschlag || p.Bestaetigt {
			t.Errorf("Position: %+v", p)
		}
	}
	if v, _ := s.Vorjahreswerte(2024); len(v) != 0 {
		t.Error("Vorjahreswerte dürfen nicht automatisch gespeichert werden")
	}

	// Doppelt: gleicher Inhalt unter anderem Namen
	schreibe(t, filepath.Join(ein, "spende kopie.txt"), []byte(strings.Join(textSpende, "\n")))
	alt(t, filepath.Join(ein, "spende kopie.txt"))
	neu, fehler = d.Durchlauf()
	if neu != 0 || len(fehler) != 0 {
		t.Fatalf("Doppelt: neu=%d fehler=%v", neu, fehler)
	}
	if _, err := os.Stat(filepath.Join(ein, "Doppelt", "spende kopie.txt")); err != nil {
		t.Errorf("Doppelte Datei nicht in Eingang/Doppelt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ein, "spende kopie.txt")); err == nil {
		t.Error("Duplikat noch im Eingang")
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "Belege", "2025")); len(entries) != 1 {
		t.Errorf("Belege/2025 enthält %d Dateien", len(entries))
	}

	// "neu.txt" ist jetzt alt genug
	alt(t, filepath.Join(ein, "neu.txt"))
	if neu, _ = d.Durchlauf(); neu != 1 {
		t.Errorf("neu.txt: neu=%d", neu)
	}
}

func TestNamensKollisionUndDatumsRueckfall(t *testing.T) {
	d, s, dir := neuerDienst(t)
	ein := filepath.Join(dir, "Eingang")
	// Zwei verschiedene Belege mit gleichem Datum/Art/Aussteller -> Zähler
	for i, betrag := range []string{"10,00", "20,00"} {
		inhalt := "Elektro Fischer GmbH\nRechnung\nRechnungsnummer " + string(rune('1'+i)) + "\nRechnungsdatum: 03.05.2024\nGesamtbetrag: " + betrag + " EUR\n"
		p := filepath.Join(ein, "r"+string(rune('a'+i))+".txt")
		schreibe(t, p, []byte(inhalt))
		alt(t, p)
	}
	// Ohne Belegdatum: Erfassungsdatum im Namen
	p := filepath.Join(ein, "x.txt")
	schreibe(t, p, []byte("Nur ein Brief ohne Datum"))
	alt(t, p)
	neu, fehler := d.Durchlauf()
	if neu != 3 || len(fehler) != 0 {
		t.Fatalf("neu=%d fehler=%v", neu, fehler)
	}
	b, _ := s.Belege(2024)
	namen := map[string]bool{}
	for _, x := range b {
		namen[x.Datei] = true
	}
	heute := time.Now().Format("20060102")
	for _, w := range []string{
		"Belege/2024/20240503_rechnung (Elektro Fischer GmbH).txt",
		"Belege/2024/20240503_rechnung (Elektro Fischer GmbH)_2.txt",
		"Belege/2024/" + heute + "_sonstiges (Nur ein Brief ohne Datum).txt",
	} {
		if !namen[w] {
			t.Errorf("Datei %q fehlt in %v", w, namen)
		}
	}
}

func TestDateinamenBereinigen(t *testing.T) {
	got := ablageName("2024-03-12", "rechnung", `A/B: "Test"? <x> | y*`)
	if got != "20240312_rechnung (A_B_ _Test__ _x_ _ y_)" {
		t.Errorf("ablageName = %q", got)
	}
	if got := ablageName("2024-03-12", "rechnung", "Firma GmbH  "); got != "20240312_rechnung (Firma GmbH)" {
		t.Errorf("ablageName = %q", got)
	}
	if got := ablageName("2024-03-12", "sonstiges", ""); got != "20240312_sonstiges" {
		t.Errorf("ablageName = %q", got)
	}
}

func TestEingangFehlt(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	d := Neu(filepath.Join(t.TempDir(), "gibtsnicht"), s, 2024)
	if n, f := d.Durchlauf(); n != 0 || len(f) != 0 {
		t.Errorf("n=%d f=%v", n, f)
	}
}
