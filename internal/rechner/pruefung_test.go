package rechner

import (
	"strings"
	"testing"

	"steuer/internal/modell"
)

// Tests zu den Befunden der Gegenprüfung. Alle Zahlen erfunden.

func benannt(person, kat, bez string, cent int64) modell.Position {
	p := pos(person, kat, cent)
	p.Bezeichnung = bez
	return p
}

// GWG-Grenze 800 € netto (bis 952 € brutto bei 19 % USt); Computer voll;
// darüber nur AfA-Positionen.
func TestGWGNettoUndComputer(t *testing.T) {
	p := []modell.Position{
		pos("A", "lstb.3", 5000000),
		benannt("A", "n.arbeitsmittel", "Schreibtisch", 90000),     // 900 € brutto → angesetzt, prüfen
		benannt("A", "n.arbeitsmittel", "Regal", 100000),           // 1.000 € brutto → nicht angesetzt
		benannt("A", "n.arbeitsmittel", "AfA Sessel 6/12", 120000), // Jahres-AfA → angesetzt
		benannt("A", "n.computer", "Notebook", 140000),             // Computer → voll
		pos("", "s.einnahmen", 300000),
		benannt("", "s.arbeitsmittel", "Mikrofon", 90000),
		benannt("", "s.computer", "Software", 140000),
	}
	e := rechne(t, nil, p)
	// 900 + 1.200 + 1.400 = 3.500 €
	if got := e.Kennzahlen["werbungskosten_A"]; got != 350000 {
		t.Errorf("Werbungskosten = %d, erwartet 350000", got)
	}
	// 3.000 − 900 − 1.400 = 700 €
	if got := e.Kennzahlen["gewinn_nebentaetigkeit"]; got != 70000 {
		t.Errorf("Gewinn = %d, erwartet 70000", got)
	}
	if !hatHinweis(e, "pruefen", "zwischen 800 € und 952 €") || !hatHinweis(e, "warnung", "„AfA“") {
		t.Errorf("Hinweise fehlen: %+v", e.Hinweise)
	}
	z, _ := zeileSuchen(e, "Anlage N", "54-56", "A")
	if z.BetragCent != 350000 || z.Sicherheit != StarkEvident {
		t.Errorf("Zeile 54-56: %+v", z)
	}
	if z, ok := zeileSuchen(e, "Anlage EÜR", "31-33", "A"); !ok || z.BetragCent != 140000 {
		t.Errorf("EÜR Computer: %+v", z)
	}
	if !hatHinweis(e, "info", "900,00 €") {
		t.Error("Betriebsausgabenpauschale: Höchstbetrag 900 € fehlt im Hinweis")
	}
}

// Gewerkschaftsbeiträge ab 2026 zusätzlich zum Arbeitnehmer-Pauschbetrag,
// Berufsverbände nicht.
func TestGewerkschaftNebenPauschbetrag(t *testing.T) {
	e := rechne(t, nil, []modell.Position{
		pos("A", "lstb.3", 3000000), pos("A", "n.gewerkschaft", 30000), pos("A", "n.berufsverband", 10000),
	})
	// max(100, 1.230) + 300 = 1.530 €
	if got := e.Kennzahlen["werbungskosten_abziehbar_A"]; got != 153000 {
		t.Errorf("abziehbar = %d, erwartet 153000", got)
	}
	// Werbungskosten über dem Pauschbetrag: Gewerkschaft einfach mitgezählt.
	e = rechne(t, map[string]string{"a.buerotage_vor": "200", "a.km_vor": "20"}, []modell.Position{
		pos("A", "lstb.3", 3000000), pos("A", "n.gewerkschaft", 30000),
	})
	// 200 × 20 × 0,38 = 1.520 + 300 = 1.820 €
	if got := e.Kennzahlen["werbungskosten_abziehbar_A"]; got != 182000 {
		t.Errorf("abziehbar = %d, erwartet 182000", got)
	}
}

// LStB Nr. 17 und Nr. 18 mindern die Entfernungspauschale; Zeile 29 ohne Eurobetrag.
func TestArbeitgeberleistungenNr17und18(t *testing.T) {
	e := rechne(t, map[string]string{"a.buerotage_vor": "220", "a.km_vor": "20"}, []modell.Position{
		pos("A", "lstb.3", 4000000), pos("A", "lstb.17", 20000), pos("A", "lstb.18", 10000),
	})
	// 220 × 20 × 0,38 = 1.672 − 200 − 100 = 1.372 €
	if got := e.Kennzahlen["entfernungspauschale_A"]; got != 137200 {
		t.Errorf("Entfernungspauschale = %d, erwartet 137200", got)
	}
	n295 := false
	for _, z := range e.Zeilen {
		if z.Zeile == "51" && z.Kennziffer == "295" && z.BetragCent == 10000 {
			n295 = true
		}
	}
	if !n295 {
		t.Error("Zeile 51 Kz 295 fehlt")
	}
	if z, _ := zeileSuchen(e, "Anlage N", "29", "A"); z.BetragCent != 0 || z.Wert != "220 Tage" {
		t.Errorf("Zeile 29: %+v", z)
	}
	if z, _ := zeileSuchen(e, "Anlage N", "5", "A"); z.Sicherheit != Sicher {
		t.Errorf("Zeile 5 Sicherheit %q, erwartet sicher", z.Sicherheit)
	}
}

// Höchstbeträge je Kind; zumutbare Belastung nur mit Kindern mit Anspruch.
func TestKinderJeKind(t *testing.T) {
	ein := map[string]string{"kind1.geburtsdatum": "2019-04-01"}
	e := rechne(t, ein, []modell.Position{pos("A", "lstb.3", 4000000), pos("", "kind.betreuung", 700000)})
	// ein Kind: 80 % von 7.000 = 5.600 → höchstens 4.800 €
	if got := e.Kennzahlen["sonderausgaben_uebrige"]; got != 480000 {
		t.Errorf("ein Kind: %d, erwartet 480000", got)
	}
	zwei := map[string]string{"kind1.geburtsdatum": "2018-04-01", "kind2.geburtsdatum": "2021-09-01"}
	k2 := benannt("", "kind.betreuung", "Kita Kind 2", 300000)
	e = rechne(t, zwei, []modell.Position{pos("A", "lstb.3", 4000000), pos("", "kind.betreuung", 700000), k2})
	// Kind 2: 2.400 €; 7.000 € ohne Zuordnung im ungünstigsten Fall bei Kind 2:
	// min(80 % × 10.000; 4.800) = 4.800 € (statt 2 × 4.800)
	if got := e.Kennzahlen["sonderausgaben_uebrige"]; got != 480000 {
		t.Errorf("zwei Kinder, nicht zugeordnet: %d, erwartet 480000", got)
	}
	if !hatHinweis(e, "pruefen", "Kinderbetreuung je Kind") {
		t.Error("Prüfhinweis Zuordnung fehlt")
	}
	k1 := benannt("", "kind.betreuung", "Hort Kind 1", 700000)
	e = rechne(t, zwei, []modell.Position{pos("A", "lstb.3", 4000000), k1, k2})
	// 4.800 + 2.400 = 7.200 €
	if got := e.Kennzahlen["sonderausgaben_uebrige"]; got != 720000 {
		t.Errorf("zwei Kinder, zugeordnet: %d, erwartet 720000", got)
	}

	// Zumutbare Belastung: volljähriges Kind ohne Monate zählt nicht.
	// GdE 30.000 − 1.230 = 28.770 €; ohne Kind (Splitting): 4 % × 15.340 + 5 % × 13.430 = 1.285,10 €
	alt := map[string]string{"kind1.geburtsdatum": "2000-01-01"}
	e = rechne(t, alt, []modell.Position{pos("A", "lstb.3", 3000000), pos("", "agb.krankheit", 10000)})
	if got := e.Kennzahlen["zumutbare_belastung"]; got != 128510 {
		t.Errorf("zumutbare Belastung = %d, erwartet 128510", got)
	}
	// mit Kind: 2 % × 15.340 + 3 % × 13.430 = 709,70 €
	e = rechne(t, ein, []modell.Position{pos("A", "lstb.3", 3000000), pos("", "agb.krankheit", 10000)})
	if got := e.Kennzahlen["zumutbare_belastung"]; got != 70970 {
		t.Errorf("zumutbare Belastung mit Kind = %d, erwartet 70970", got)
	}
}

func TestFeldkatalogZeile30Rechtsstand2026(t *testing.T) {
	for _, f := range Feldkatalog() {
		if f.Formular == "Anlage N" && f.Zeile != nil && *f.Zeile == "30" {
			if !strings.Contains(f.Inhalt, "0,38 EUR je vollem Entfernungskilometer ab dem 1. km") {
				t.Errorf("Zeile 30 ohne Hinweis auf Rechtsstand 2026: %s", f.Inhalt)
			}
			return
		}
	}
	t.Error("Anlage N Zeile 30 fehlt")
}
