package erfassung

import (
	"strings"
	"testing"

	"steuer/internal/modell"
)

func analysiereText(zeilen []string, standard int) Ergebnis {
	return auswerten("test.txt", "hash", textErgebnis{Text: strings.Join(zeilen, "\n")}, standard)
}

func posMap(e Ergebnis) map[string]modell.Position {
	m := map[string]modell.Position{}
	for _, p := range e.Positionen {
		m[p.Kategorie] = p
	}
	return m
}

func TestLohnsteuerbescheinigung(t *testing.T) {
	e := analysiereText(textLohnsteuer, 2020)
	if e.Beleg.Art != ArtLohnsteuerbescheinigung {
		t.Fatalf("Art = %q", e.Beleg.Art)
	}
	if e.Beleg.Steuerjahr != 2024 {
		t.Errorf("Steuerjahr = %d", e.Beleg.Steuerjahr)
	}
	if !strings.Contains(e.Beleg.Aussteller, "Muster Maschinenbau GmbH") || strings.Contains(e.Beleg.Aussteller, "Beispielweg") {
		t.Errorf("Aussteller = %q", e.Beleg.Aussteller)
	}
	want := map[string]int64{
		"lstb.3": 4825000, "lstb.4": 712300, "lstb.17": 31250, "lstb.22a": 448725,
		"lstb.23a": 448725, "lstb.25": 394013, "lstb.26": 82050, "lstb.27": 62725,
	}
	got := posMap(e)
	for k, v := range want {
		p, ok := got[k]
		if !ok || p.BetragCent != v {
			t.Errorf("%s = %+v; erwartet %d", k, p, v)
		}
	}
	if _, ok := got["lstb.5"]; ok {
		t.Error("Soli 0 soll keinen Vorschlag erzeugen")
	}
	if len(got) != len(want) {
		t.Errorf("Positionen: %d, erwartet %d", len(got), len(want))
	}
	for _, p := range e.Positionen {
		if !p.Vorschlag || p.Bestaetigt || p.Steuerjahr != 2024 || !modell.KategorieBekannt(p.Kategorie) {
			t.Errorf("Position fehlerhaft: %+v", p)
		}
	}
	if e.Beleg.BetragCent != 4825000 {
		t.Errorf("Beleg-Betrag = %d", e.Beleg.BetragCent)
	}
}

func TestSteuerbescheid(t *testing.T) {
	e := analysiereText(textBescheid, 2020)
	if e.Beleg.Art != ArtSteuerbescheid || e.Beleg.Steuerjahr != 2023 {
		t.Fatalf("Art=%q Jahr=%d", e.Beleg.Art, e.Beleg.Steuerjahr)
	}
	if e.Beleg.Aussteller != "Finanzamt Musterstadt" {
		t.Errorf("Aussteller = %q", e.Beleg.Aussteller)
	}
	if e.Beleg.Belegdatum != "2025-03-15" {
		t.Errorf("Belegdatum = %q", e.Beleg.Belegdatum)
	}
	if len(e.Positionen) != 0 {
		t.Errorf("Bescheid soll keine Positionen liefern: %+v", e.Positionen)
	}
	want := map[string]int64{
		"bescheid.zve": 4198000, "bescheid.est": 751200, "bescheid.werbungskosten": 123000,
		"bescheid.eink_nichtselbst": 4680000, "bescheid.eink_selbst": 325000,
		"bescheid.gde": 5005000, "bescheid.erstattung": 38800,
	}
	got := map[string]int64{}
	for _, v := range e.Vorjahreswerte {
		if v.Steuerjahr != 2023 {
			t.Errorf("Jahr des Vorjahreswerts: %+v", v)
		}
		got[v.Schluessel] = v.BetragCent
	}
	for k, v := range want {
		if g, ok := got[k]; !ok || g != v {
			t.Errorf("%s = %d (vorhanden %v); erwartet %d", k, g, ok, v)
		}
	}
}

func TestNachzahlungNegativ(t *testing.T) {
	e := analysiereText([]string{"Einkommensteuerbescheid 2022", "Finanzamt Beispielstadt", "Festgesetzte Einkommensteuer 5.000,00 EUR", "Nachzahlung 412,00 EUR"}, 2020)
	for _, v := range e.Vorjahreswerte {
		if v.Schluessel == "bescheid.erstattung" && v.BetragCent != -41200 {
			t.Errorf("Nachzahlung = %d", v.BetragCent)
		}
	}
	if e.Beleg.Steuerjahr != 2022 {
		t.Errorf("Jahr = %d", e.Beleg.Steuerjahr)
	}
}

func TestHandwerker(t *testing.T) {
	e := analysiereText(textHandwerker, 2020)
	b := e.Beleg
	if b.Art != ArtHandwerker || b.Belegdatum != "2024-03-12" || b.Steuerjahr != 2024 || b.BetragCent != 80500 {
		t.Errorf("Beleg: %+v", b)
	}
	if b.Aussteller != "Malerbetrieb Beispiel GmbH" {
		t.Errorf("Aussteller = %q", b.Aussteller)
	}
	p := posMap(e)["h.handwerker"]
	if p.BetragCent != 59500 || !p.Vorschlag || p.Bestaetigt {
		t.Errorf("Position: %+v", p)
	}
}

func TestSpende(t *testing.T) {
	e := analysiereText(textSpende, 2020)
	if e.Beleg.Art != ArtSpende || e.Beleg.BetragCent != 15000 || e.Beleg.Belegdatum != "2025-01-20" || e.Beleg.Steuerjahr != 2025 {
		t.Errorf("Beleg: %+v", e.Beleg)
	}
	if e.Beleg.Aussteller != "Tierschutzverein Beispielstadt e.V." {
		t.Errorf("Aussteller = %q", e.Beleg.Aussteller)
	}
	if p := posMap(e)["sa.spenden"]; p.BetragCent != 15000 {
		t.Errorf("Position: %+v", p)
	}
}

func TestRechnungArbeitsmittel(t *testing.T) {
	e := analysiereText(textRechnung, 2020)
	if e.Beleg.Art != ArtRechnung || e.Beleg.BetragCent != 119000 || e.Beleg.Hinweis != "" {
		t.Errorf("Beleg: %+v", e.Beleg)
	}
	if p := posMap(e)["n.arbeitsmittel"]; p.BetragCent != 119000 || p.Notiz == "" {
		t.Errorf("Position: %+v", p)
	}
}

func TestBetragUnsicher(t *testing.T) {
	e := analysiereText([]string{"Rechnung", "Rechnungsnummer 1", "Datum: 01.02.2024", "Beratung", "Summe 1.234"}, 2020)
	if e.Beleg.BetragCent == 0 || !strings.Contains(e.Beleg.Hinweis, "unsicher") {
		t.Errorf("Beleg: %+v", e.Beleg)
	}
	e = analysiereText([]string{"Rechnung", "Rechnungsnummer 1", "Gesamtbetrag 10,00 EUR", "Gesamtbetrag 20,00 EUR"}, 2020)
	if !strings.Contains(e.Beleg.Hinweis, "Mehrere") {
		t.Errorf("Hinweis: %q", e.Beleg.Hinweis)
	}
	e = analysiereText([]string{"Rechnung", "Rechnungsnummer 1", "Beratung ohne Preis"}, 2020)
	if e.Beleg.BetragCent != 0 || !strings.Contains(e.Beleg.Hinweis, "Kein Betrag") {
		t.Errorf("Hinweis: %q", e.Beleg.Hinweis)
	}
}

func TestStandardjahrRueckfall(t *testing.T) {
	e := analysiereText([]string{"Irgendein Schreiben", "ohne Datum und Betrag"}, 2023)
	if e.Beleg.Art != ArtSonstiges || e.Beleg.Steuerjahr != 2023 || len(e.Positionen) != 0 {
		t.Errorf("%+v", e)
	}
}

func TestErkenneArt(t *testing.T) {
	tests := map[string]string{
		"Nebenkostenabrechnung 2023\nAbrechnungszeitraum 01.01.-31.12.2023\nVorauszahlungen": ArtNebenkosten,
		"Mietvertrag\nzwischen Vermieter und Mieter\nKaution":                                ArtMietvertrag,
		"Versicherungsschein\nVersicherungsnummer 123\nHaftpflichtversicherung":              ArtVersicherung,
		"Elternbeitrag Kindergarten Sonnenschein\nBetreuungsvertrag":                         ArtKinderbetreuung,
		"Honorarrechnung\nFür meine Beratungsleistung berechne ich ein Honorar":              ArtHonorarAusgang,
		"Rechnung\nRechnungsnummer 5\nRechnungsdatum 1.1.2024\nUSt-IdNr":                     ArtRechnung,
		"Betriebskostenabrechnung für die Wohnung":                                           ArtNebenkosten,
		"Hallo, anbei die Unterlagen von neulich.":                                           ArtSonstiges,
	}
	for text, want := range tests {
		if got := ErkenneArt(text); got != want {
			t.Errorf("ErkenneArt(%q) = %q; erwartet %q", text, got, want)
		}
	}
}

func TestVersicherungKategorie(t *testing.T) {
	e := analysiereText([]string{"Muster Versicherung AG", "Beitragsrechnung Haftpflichtversicherung", "Versicherungsschein 123", "Jahresbeitrag 84,50 EUR", "Datum: 02.01.2024"}, 2020)
	if e.Beleg.Art != ArtVersicherung || posMap(e)["vor.sonstige"].BetragCent != 8450 {
		t.Errorf("%+v / %+v", e.Beleg, e.Positionen)
	}
}
