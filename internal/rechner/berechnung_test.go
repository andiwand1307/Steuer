package rechner

import (
	"strings"
	"testing"

	"steuer/internal/modell"
)

// Alle Zahlen in diesen Tests sind erfunden.

func pos(person, kat string, cent int64) modell.Position {
	return modell.Position{Steuerjahr: 2026, Person: person, Kategorie: kat, BetragCent: cent, Bestaetigt: true}
}

func posDatum(kat, datum string, cent int64) modell.Position {
	p := pos("", kat, cent)
	p.Datum = datum
	return p
}

func rechne(t *testing.T, angaben map[string]string, positionen []modell.Position) Ergebnis {
	t.Helper()
	e, err := Berechne(2026, angaben, positionen)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func zeileSuchen(e Ergebnis, formular, zeile, person string) (Ergebniszeile, bool) {
	for _, z := range e.Zeilen {
		if z.Formular == formular && z.Zeile == zeile && z.Person == person {
			return z, true
		}
	}
	return Ergebniszeile{}, false
}

func hatHinweis(e Ergebnis, stufe, teil string) bool {
	for _, h := range e.Hinweise {
		if h.Stufe == stufe && (strings.Contains(h.Thema, teil) || strings.Contains(h.Text, teil)) {
			return true
		}
	}
	return false
}

func TestEntfernungspauschaleUmzug(t *testing.T) {
	e := rechne(t, map[string]string{
		"umzug.datum":     "2026-05-15",
		"a.buerotage_vor": "100", "a.km_vor": "20,9", // volle km: 20
		"a.buerotage_nach": "80", "a.km_nach": "10",
	}, []modell.Position{pos("A", "lstb.3", 4000000)})
	// 100 × 20 × 0,38 = 760,00; 80 × 10 × 0,38 = 304,00
	if got := e.Kennzahlen["entfernungspauschale_A"]; got != 106400 {
		t.Errorf("Entfernungspauschale = %d, erwartet 106400", got)
	}
	z, ok := zeileSuchen(e, "Anlage N", "37", "A")
	if !ok || z.BetragCent != 30400 || z.Sicherheit != StarkEvident {
		t.Errorf("Zeile 37 (zweite Angabe nach Umzug) falsch: %+v", z)
	}
	if !hatHinweis(e, "info", "volle Kilometer") {
		t.Error("Hinweis auf abgerundete Kilometer fehlt")
	}
}

func TestEntfernungspauschaleHoechstbetrag(t *testing.T) {
	ang := map[string]string{"a.buerotage_vor": "220", "a.km_vor": "60", "a.verkehrsmittel": optOeffentlich}
	p := []modell.Position{pos("A", "lstb.3", 6000000)}
	// 220 × 60 × 0,38 = 5.016 € → ohne Pkw höchstens 4.500 €
	if got := rechne(t, ang, p).Kennzahlen["entfernungspauschale_A"]; got != 450000 {
		t.Errorf("ohne Pkw = %d, erwartet 450000", got)
	}
	ang["a.oepnv_kosten"] = "4.800,00"
	if got := rechne(t, ang, p).Kennzahlen["entfernungspauschale_A"]; got != 480000 {
		t.Errorf("mit höheren ÖPNV-Kosten = %d, erwartet 480000", got)
	}
	delete(ang, "a.oepnv_kosten")
	ang["a.verkehrsmittel"] = optPkw
	if got := rechne(t, ang, p).Kennzahlen["entfernungspauschale_A"]; got != 501600 {
		t.Errorf("mit Pkw = %d, erwartet 501600", got)
	}
	// Steuerfreie Arbeitgeberleistungen mindern.
	p = append(p, pos("A", "lstb.17", 60000))
	if got := rechne(t, ang, p).Kennzahlen["entfernungspauschale_A"]; got != 441600 {
		t.Errorf("nach AG-Ersatz = %d, erwartet 441600", got)
	}
}

func TestTagespauschaleHoechstbetrag(t *testing.T) {
	e := rechne(t, map[string]string{"a.hotage_vor": "230"}, []modell.Position{pos("A", "lstb.3", 5000000)})
	// 230 Tage, höchstens 210 Tage × 6 € = 1.260 €
	if got := e.Kennzahlen["tagespauschale_N_A"]; got != 126000 {
		t.Errorf("Tagespauschale = %d, erwartet 126000", got)
	}
	z, ok := zeileSuchen(e, "Anlage N", "58", "A")
	if !ok || z.Wert != "210 Tage" {
		t.Errorf("Zeile 58 falsch: %+v", z)
	}
	// Mit Zeile 59 (kein anderer Arbeitsplatz) gemeinsam höchstens 210 Tage.
	e = rechne(t, map[string]string{"a.hotage_vor": "150", "a.anderer_arbeitsplatz": "nein", "a.hotage_mit_fahrt": "100"}, []modell.Position{pos("A", "lstb.3", 5000000)})
	if got := e.Kennzahlen["tagespauschale_N_A"]; got != 126000 {
		t.Errorf("Tagespauschale 58+59 = %d, erwartet 126000", got)
	}
	if z, _ := zeileSuchen(e, "Anlage N", "59", "A"); z.Wert != "60 Tage" {
		t.Errorf("Zeile 59 = %q, erwartet 60 Tage", z.Wert)
	}
}

// Arbeitszimmer ab Umzug am 15.05.: Jahrespauschale gekürzt um 4 volle
// Monate (Januar–April), Tagespauschale nur für Januar–April.
func azAngaben() map[string]string {
	return map[string]string{
		"umzug.datum":                  "2026-05-15",
		"a.hotage_vor":                 "60",
		"a.hotage_umzugsmonat":         "6",
		"a.hotage_nach":                "100",
		"az.vorhanden":                 "ja",
		"az.ausschliesslich_beruflich": "ja",
		"az.mittelpunkt_anstellung":    "ja",
		"az.nachweise":                 "ja",
		"az.wohnflaeche":               "100",
		"az.flaeche":                   "12,5",
	}
}

func TestArbeitszimmerUmzugsjahr(t *testing.T) {
	ang := azAngaben()
	ang["az.ansatz"] = optJahrespauschale
	p := []modell.Position{pos("A", "lstb.3", 5000000)}
	e := rechne(t, ang, p)
	// 1.260 − 4 × 105 = 840 €
	if got := e.Kennzahlen["arbeitszimmer_N_A"]; got != 84000 {
		t.Errorf("Jahrespauschale = %d, erwartet 84000", got)
	}
	// (60 − 6) Tage × 6 € = 324 €
	if got := e.Kennzahlen["tagespauschale_N_A"]; got != 32400 {
		t.Errorf("Tagespauschale vor Umzugsmonat = %d, erwartet 32400", got)
	}

	// Tatsächliche Kosten: nur Kosten ab Umzug, Flächenanteil 12,5 %.
	ang["az.ansatz"] = optTatsaechlich
	p = append(p,
		posDatum("wohnung.miete", "2026-03-01", 90000), // alte Wohnung: nicht
		posDatum("wohnung.miete", "2026-06-01", 800000),
		posDatum("wohnung.nebenkosten", "2026-07-01", 200000),
		posDatum("wohnung.zimmer_direkt", "2026-05-10", 15000),
		posDatum("wohnung.versicherung", "2026-06-01", 10000), // strittig: nicht
	)
	e = rechne(t, ang, p)
	// (8.000 + 2.000) × 12,5 % = 1.250 € + 150 € direkt = 1.400 €
	if got := e.Kennzahlen["arbeitszimmer_N_A"]; got != 140000 {
		t.Errorf("tatsächliche Kosten = %d, erwartet 140000", got)
	}
	if !hatHinweis(e, "pruefen", "Hausratversicherung") {
		t.Error("Hinweis Hausratversicherung fehlt")
	}

	// Automatisch: Mittelpunkt + Nachweise → günstigster Ansatz (hier tatsächliche Kosten).
	ang["az.ansatz"] = optAutomatisch
	e = rechne(t, ang, p)
	if got := e.Kennzahlen["arbeitszimmer_N_A"]; got != 140000 {
		t.Errorf("automatisch = %d, erwartet 140000", got)
	}
	if len(e.Varianten) != 3 {
		t.Errorf("erwartet 3 Varianten, %d", len(e.Varianten))
	}

	// Ohne Nachweise: nur Tagespauschale (160 Tage × 6 = 960 €).
	ang["az.nachweise"] = "nein"
	e = rechne(t, ang, p)
	if e.Kennzahlen["arbeitszimmer_N_A"] != 0 || e.Kennzahlen["tagespauschale_N_A"] != 96000 {
		t.Errorf("ohne Nachweise: AZ %d, TP %d", e.Kennzahlen["arbeitszimmer_N_A"], e.Kennzahlen["tagespauschale_N_A"])
	}
}

func TestArbeitszimmerFallgruppe3(t *testing.T) {
	ang := azAngaben()
	ang["az.mittelpunkt_nebentaetigkeit"] = "nein"
	ang["az.ansatz"] = optJahrespauschale
	p := []modell.Position{pos("A", "lstb.3", 5000000), pos("", "s.einnahmen", 200000)}
	e := rechne(t, ang, p)
	if e.Kennzahlen["arbeitszimmer_N_A"] != 0 || e.Kennzahlen["arbeitszimmer_EUER"] != 0 {
		t.Error("Fallgruppe 3 ohne einheitlichen Wirkungskreis darf kein Arbeitszimmer ansetzen")
	}
	if !hatHinweis(e, "warnung", "nicht zulässig") {
		t.Error("Warnung fehlt")
	}
	// Variante wird trotzdem mit Risikohinweis gezeigt.
	gefunden := false
	for _, v := range e.Varianten {
		if strings.HasPrefix(v.Titel, "Jahrespauschale") && strings.Contains(v.Beschreibung, "regelmäßig NICHT") {
			gefunden = true
		}
	}
	if !gefunden {
		t.Errorf("Variante Jahrespauschale mit Risikohinweis fehlt: %+v", e.Varianten)
	}
	// Kein Raum → keine Arbeitszimmer-Varianten.
	ang["az.vorhanden"] = "nein"
	if e = rechne(t, ang, p); len(e.Varianten) != 1 {
		t.Errorf("ohne Raum nur Tagespauschale erwartet, %d Varianten", len(e.Varianten))
	}
}

// Günstigerprüfung: Entlastung durch 9.756 € Freibeträge gegen 3.108 € Kindergeld.
func TestGuenstigerpruefungKind(t *testing.T) {
	rw := regelwerk2026(t)
	f := &fall{jahr: 2026, kinder: []kindDaten{{nr: 1, monate: 12}}}
	r := &rechnung{rw: rw, f: f, gesehen: map[string]bool{}, e: Ergebnis{Kennzahlen: map[string]int64{}}}

	// zvE 200.000 €: 61.728 − 57.630 = 4.098 € > 3.108 € → Freibeträge
	zvE, kfb, hinzu := r.kinder(20000000)
	if zvE != 20000000-975600 || kfb != 975600 || hinzu != 310800 {
		t.Errorf("hohes Einkommen: zvE %d, kfb %d, hinzu %d", zvE, kfb, hinzu)
	}
	// zvE 60.000 €: 8.434 − 5.766 = 2.668 € < 3.108 € → Kindergeld
	zvE, kfb, hinzu = r.kinder(6000000)
	if zvE != 6000000 || kfb != 975600 || hinzu != 0 {
		t.Errorf("mittleres Einkommen: zvE %d, kfb %d, hinzu %d", zvE, kfb, hinzu)
	}
	// Geburt im Juni: 7 Monate → 9.756 × 7/12 = 5.691 €, Kindergeld 7 × 259 = 1.813 €
	f.kinder = []kindDaten{{nr: 1, monate: 7}}
	_, kfb, _ = r.kinder(20000000)
	if kfb != 569100 || r.e.Kennzahlen["kindergeld_anspruch"] != 181300 {
		t.Errorf("7 Monate: kfb %d, Kindergeld %d", kfb, r.e.Kennzahlen["kindergeld_anspruch"])
	}
}

func TestHaerteausgleich(t *testing.T) {
	p := []modell.Position{pos("A", "lstb.3", 4000000), pos("", "s.einnahmen", 60000)}
	// Nebeneinkünfte 600 € → Abzug 820 − 600 = 220 €
	e := rechne(t, map[string]string{"s.einkunftsart": optParagraph18}, p)
	if got := e.Kennzahlen["haerteausgleich"]; got != 22000 {
		t.Errorf("Härteausgleich = %d, erwartet 22000", got)
	}
	p[1].BetragCent = 30000 // 300 € → voll abgezogen
	if got := rechne(t, nil, p).Kennzahlen["haerteausgleich"]; got != 30000 {
		t.Errorf("Härteausgleich = %d, erwartet 30000", got)
	}
}

func TestUnbestaetigteUndFehler(t *testing.T) {
	offen := pos("A", "n.arbeitsmittel", 50000)
	offen.Bestaetigt = false
	e := rechne(t, nil, []modell.Position{pos("A", "lstb.3", 3000000), offen, pos("", "n.sonstige", 1000)})
	if e.Kennzahlen["werbungskosten_A"] != 0 {
		t.Error("unbestätigte Position wurde gerechnet")
	}
	if !hatHinweis(e, "info", "unbestätigte") || !hatHinweis(e, "warnung", "keinem Ehegatten") {
		t.Errorf("Hinweise fehlen: %+v", e.Hinweise)
	}
	if _, err := Berechne(2026, map[string]string{"a.km_vor": "zwölf"}, nil); err == nil {
		t.Error("ungültige Zahl muss Fehler liefern")
	}
	if _, err := Berechne(2025, nil, nil); err == nil {
		t.Error("Jahr ohne Regelwerk muss Fehler liefern")
	}
}

// Gesamtfall von Ende zu Ende, von Hand nachgerechnet.
func TestGesamtfall(t *testing.T) {
	ang := map[string]string{
		"umzug.datum":     "15.05.2026",
		"a.buerotage_vor": "40", "a.km_vor": "25",
		"a.buerotage_nach": "20", "a.km_nach": "15",
		"a.hotage_vor": "50", "a.hotage_umzugsmonat": "5", "a.hotage_nach": "120",
		"b.buerotage_vor": "80", "b.km_vor": "10",
		"b.buerotage_nach": "120", "b.km_nach": "8",
		"az.vorhanden": "ja", "az.ausschliesslich_beruflich": "ja",
		"az.mittelpunkt_anstellung": "ja", "az.mittelpunkt_nebentaetigkeit": "nein",
		"az.nachweise": "ja", "az.wohnflaeche": "90", "az.flaeche": "12",
		"s.einkunftsart":     optParagraph18,
		"kind1.geburtsdatum": "2020-03-10",
	}
	offen := pos("A", "n.fortbildung", 99900)
	offen.Bestaetigt = false
	p := []modell.Position{
		pos("A", "lstb.3", 5000000), pos("A", "lstb.4", 600000),
		pos("A", "lstb.23a", 465000), pos("A", "lstb.22a", 465000),
		pos("A", "lstb.25", 300000), pos("A", "lstb.26", 90000), pos("A", "lstb.27", 65000),
		pos("B", "lstb.3", 3000000), pos("B", "lstb.4", 300000),
		pos("B", "lstb.23a", 279000), pos("B", "lstb.22a", 279000),
		pos("B", "lstb.25", 180000), pos("B", "lstb.26", 54000), pos("B", "lstb.27", 39000),
		pos("", "s.einnahmen", 300000), pos("", "s.sonstige", 20000), pos("", "s.bewirtung", 10000),
		pos("", "kind.betreuung", 200000), pos("", "sa.spenden", 10000),
		pos("", "h.handwerker", 50000), pos("", "agb.krankheit", 30000),
		offen,
	}
	e := rechne(t, ang, p)

	erwartet := map[string]int64{
		// A: Entfernung 40×25×0,38 = 380 + 20×15×0,38 = 114 → 494 €.
		// Tagespauschale 170 Tage × 6 = 1.020 € – optimiert ganz in die EÜR,
		// weil die übrigen Werbungskosten (494 €) unter dem Pauschbetrag liegen.
		"entfernungspauschale_A":     49400,
		"tagespauschale_N_A":         0,
		"tagespauschale_EUER":        102000,
		"werbungskosten_abziehbar_A": 123000,
		"einkuenfte_19_A":            4877000,
		// B: 80×10×0,38 = 304 + 120×8×0,38 = 364,80 → 668,80 € < 1.230 €
		"entfernungspauschale_B": 66880,
		"einkuenfte_19_B":        2877000,
		// EÜR: 3.000 − 200 − 70 % von 100 − 1.020 = 1.710 €
		"gewinn_nebentaetigkeit": 171000,
		"summe_einkuenfte":       7925000,
		"haerteausgleich":        0,
		// Altersvorsorge 14.880 − 7.440 AG-Anteil = 7.440 €
		"vorsorge_altersvorsorge": 744000,
		// KV/PV: 3.000×0,96 + 900 + 1.800×0,96 + 540 = 6.048 € > 3.800 € Höchstbetrag
		"vorsorge_sonstige_abziehbar": 604800,
		// Spenden 100 + Kinderbetreuung 80 % von 2.000 = 1.600 → 1.700 €
		"sonderausgaben_uebrige": 170000,
		"sonderausgaben":         1518800,
		// zumutbare Belastung 306,80 + 1.073,70 + 1.124,80 = 2.505,30 € > 300 €
		"zumutbare_belastung": 250530,
		"agb_abziehbar":       0,
		"einkommen":           6406200,
		// Splitting(64.062) = 9.592; Splitting(54.306) = 6.856; Entlastung 2.736 < 3.108 → Kindergeld
		"kindergeld_hinzurechnung": 0,
		"zve":                      6406200,
		"tarifliche_est":           959200,
		// Handwerker 20 % von 500 = 100 €
		"ermaessigung_35a": 10000,
		// Soli-Bemessung 6.856 − 100 = 6.756 € < 40.700 €
		"soli_bemessungsgrundlage": 675600,
	}
	for k, v := range erwartet {
		if got := e.Kennzahlen[k]; got != v {
			t.Errorf("%s = %d, erwartet %d", k, got, v)
		}
	}
	if e.ZvECent != 6406200 || e.EStCent != 949200 || e.SoliCent != 0 {
		t.Errorf("zvE %d, ESt %d, Soli %d", e.ZvECent, e.EStCent, e.SoliCent)
	}
	// Lohnsteuer 9.000 € − ESt 9.492 € = Nachzahlung 492 €
	if e.VorausgezahltCent != 900000 || e.ErstattungCent != -49200 {
		t.Errorf("vorausgezahlt %d, Erstattung %d", e.VorausgezahltCent, e.ErstattungCent)
	}
	// Fallgruppe 3: Tagespauschale gerechnet, Arbeitszimmer nur als Variante
	// (tatsächliche Kosten mangels Wohnungskosten nicht berechenbar).
	if len(e.Varianten) != 2 || !hatHinweis(e, "info", "Tatsächliche Arbeitszimmerkosten nicht berechenbar") || !strings.HasSuffix(e.Varianten[0].Titel, "(gerechnet)") {
		t.Errorf("Varianten: %+v", e.Varianten)
	}
	if !hatHinweis(e, "info", "Vordrucken 2025") || !hatHinweis(e, "info", "unbestätigte") {
		t.Error("Pflichthinweise fehlen")
	}
	for _, z := range e.Zeilen {
		if z.Rechtsgrundlage == "" || z.Sicherheit == "" {
			t.Errorf("Zeile ohne Rechtsgrundlage/Sicherheit: %+v", z)
		}
		if z.Sicherheit != Sicher && !hatHinweis(e, "pruefen", z.Bezeichnung) {
			t.Errorf("unsichere Zeile ohne Prüfhinweis: %s %s", z.Formular, z.Zeile)
		}
	}
	if z, ok := zeileSuchen(e, "Anlage S", "", "A"); !ok || z.BetragCent != 171000 {
		t.Errorf("Anlage S Gewinn fehlt: %+v", z)
	}
}

func TestAngabeDefs(t *testing.T) {
	gesehen := map[string]bool{}
	for _, d := range AngabeDefs(2026) {
		if gesehen[d.Schluessel] {
			t.Errorf("doppelt: %s", d.Schluessel)
		}
		gesehen[d.Schluessel] = true
		switch d.Typ {
		case "zahl", "betrag", "datum", "text", "jaNein":
		case "auswahl":
			if len(d.Optionen) == 0 {
				t.Errorf("%s ohne Optionen", d.Schluessel)
			}
		default:
			t.Errorf("%s: unbekannter Typ %s", d.Schluessel, d.Typ)
		}
	}
	if AngabeDefs(2025) != nil {
		t.Error("für 2025 keine Angaben erwartet")
	}
}
