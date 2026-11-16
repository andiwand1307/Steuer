package rechner

import (
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"
)

// AngabeDef beschreibt ein Stammdatum, das die Berechnung braucht.
type AngabeDef struct {
	Schluessel string
	Titel      string
	Gruppe     string
	Typ        string // "zahl", "betrag", "datum", "text", "jaNein", "auswahl"
	Hilfe      string
	Optionen   []string
}

// Auswahlwerte.
const (
	optJa     = "ja"
	optNein   = "nein"
	optUnklar = "unklar"

	optPkw         = "Pkw (eigener oder überlassener)"
	optOeffentlich = "öffentliche Verkehrsmittel / sonstige"

	optAutomatisch      = "automatisch"
	optTagespauschale   = "Tagespauschale"
	optJahrespauschale  = "Jahrespauschale"
	optTatsaechlich     = "tatsächliche Kosten"
	optNachAnteil       = "nach Nutzungsanteil"
	optGanzN            = "ganz Anlage N"
	optGanzEUER         = "ganz EÜR"
	optOptimiert        = "optimiert"
	optParagraph18      = "§ 18 freiberuflich (Anlage S)"
	optParagraph15      = "§ 15 gewerblich (Anlage G)"
	maxKinder           = 3
	personA, personB    = "A", "B"
	praefixA, praefixB  = "a.", "b."
	gruppeAllgemein     = "Allgemein"
	gruppeUmzug         = "Umzug"
	gruppeArbeitszimmer = "Arbeitszimmer und Homeoffice"
	gruppeNeben         = "Selbständige Nebentätigkeit"
)

func praefix(person string) string {
	if person == personB {
		return praefixB
	}
	return praefixA
}

// AngabeDefs liefert alle Stammdaten, die die Berechnung eines Jahres liest.
func AngabeDefs(jahr int) []AngabeDef {
	if jahr != 2026 {
		return nil
	}
	defs := []AngabeDef{
		{"allg.vorauszahlung_est", "Geleistete Einkommensteuer-Vorauszahlungen", gruppeAllgemein, "betrag", "Laut Vorauszahlungsbescheid tatsächlich gezahlt (nicht die Lohnsteuer).", nil},
		{"allg.vorauszahlung_soli", "Geleistete Vorauszahlungen Solidaritätszuschlag", gruppeAllgemein, "betrag", "", nil},

		{"umzug.datum", "Umzugsdatum (Einzug neue Wohnung)", gruppeUmzug, "datum", "Leer lassen, wenn 2026 kein Umzug. Bestimmt die Aufteilung der Fahrten (alte/neue Entfernung) und die Monate mit Arbeitszimmer.", nil},
		{"umzug.beruflich", "Umzug aus eigenständigem beruflichem Grund", gruppeUmzug, "jaNein", "Nur 'ja' bei z.B. Arbeitgeberwechsel, Versetzung oder Fahrzeitverkürzung um mindestens 1 Stunde täglich. Ein Umzug, um (erstmals) ein Arbeitszimmer einzurichten, ist nicht beruflich (BFH VI R 3/23). Standard: nein.", nil},

		{"az.vorhanden", "Abgeschlossener Raum als Arbeitszimmer (neue Wohnung)", gruppeArbeitszimmer, "jaNein", "Eigener, vom Wohnbereich abgetrennter Raum; keine Arbeitsecke.", nil},
		{"az.person", "Wer nutzt das Arbeitszimmer?", gruppeArbeitszimmer, "auswahl", "", []string{personA, personB}},
		{"az.durchgangszimmer", "Arbeitszimmer ist Durchgangszimmer", gruppeArbeitszimmer, "jaNein", "Wenn man durch den Raum muss, um private Räume zu erreichen: dann wird konservativ kein Arbeitszimmer angesetzt.", nil},
		{"az.ausschliesslich_beruflich", "Arbeitszimmer (nahezu) ausschließlich beruflich genutzt", gruppeArbeitszimmer, "jaNein", "Keine private Mitbenutzung (Gästebett, Kinderspielzeug, Bügeln).", nil},
		{"az.wohnflaeche", "Wohnfläche neue Wohnung in m² (einschließlich Arbeitszimmer, ohne Keller/Abstellräume)", gruppeArbeitszimmer, "zahl", "", nil},
		{"az.flaeche", "Fläche Arbeitszimmer in m²", gruppeArbeitszimmer, "zahl", "", nil},
		{"az.mittelpunkt_anstellung", "Mittelpunkt der Anstellung im Arbeitszimmer (Selbsteinschätzung)", gruppeArbeitszimmer, "auswahl", "'ja' nur, wenn die Arbeit zu Hause und im Büro qualitativ gleichwertig ist und mehr als die Hälfte der Arbeitszeit zu Hause geleistet wird.", []string{optJa, optNein, optUnklar}},
		{"az.mittelpunkt_nebentaetigkeit", "Mittelpunkt der Nebentätigkeit im Arbeitszimmer (Selbsteinschätzung)", gruppeArbeitszimmer, "auswahl", "Bei Moderationen vor Ort beim Auftraggeber in der Regel 'nein' (prägender Teil außer Haus).", []string{optJa, optNein, optUnklar}},
		{"az.einheitlicher_wirkungskreis", "Anstellung und Nebentätigkeit bilden einen einheitlichen Wirkungskreis", gruppeArbeitszimmer, "jaNein", "Nur 'ja', wenn die Nebentätigkeit inhaltlich auf die Anstellung ausgerichtet ist und das nachweisbar ist. Hohes Risiko.", nil},
		{"az.nachweise", "Nachweise für den Mittelpunkt vorhanden", gruppeArbeitszimmer, "jaNein", "Z.B. Arbeitgeberbescheinigung (Homeoffice-Quote, Gleichwertigkeit), Tageskalender, Grundriss, Fotos.", nil},
		{"az.anteil_nebentaetigkeit", "Anteil der Nutzung für die Nebentätigkeit in %", gruppeArbeitszimmer, "zahl", "Zeitlicher Nutzungsanteil; Schlüssel für die Aufteilung auf Anlage N und EÜR.", nil},
		{"az.alte_wohnung", "Auch in der alten Wohnung gab es ein abgeschlossenes Arbeitszimmer", gruppeArbeitszimmer, "jaNein", "Wird nicht gerechnet (nur Hinweis).", nil},
		{"az.ansatz", "Ansatz Arbeitszimmer", gruppeArbeitszimmer, "auswahl", "'automatisch' setzt die Jahrespauschale oder tatsächliche Kosten nur an, wenn der Mittelpunkt eindeutig und belegt ist; sonst die Tagespauschale.", []string{optAutomatisch, optTagespauschale, optJahrespauschale, optTatsaechlich}},
		{"ho.zuordnung", "Zuordnung Tagespauschale/Arbeitszimmer auf Anlage N und EÜR", gruppeArbeitszimmer, "auswahl", "Aufteilung nach Nutzungsanteil oder vollständig einer Tätigkeit (Anleitung Anlage N 2025). 'optimiert' wählt die Variante mit der niedrigsten Steuer.", []string{optOptimiert, optNachAnteil, optGanzN, optGanzEUER}},

		{"s.person", "Wer übt die selbständige Nebentätigkeit aus?", gruppeNeben, "auswahl", "", []string{personA, personB}},
		{"s.einkunftsart", "Einkunftsart der Nebentätigkeit", gruppeNeben, "auswahl", "Moderation ist nach BFH VIII R 5/12 im Zweifel gewerblich; freiberuflich nur bei künstlerischer oder journalistischer Leistung.", []string{optParagraph18, optParagraph15, optUnklar}},
		{"s.kleinunternehmer", "Kleinunternehmer nach § 19 UStG", gruppeNeben, "jaNein", "Standard: ja. Ohne Kleinunternehmerregelung rechnet das Programm die Umsatzsteuer nicht.", nil},
	}
	for _, p := range []string{personA, personB} {
		x := praefix(p)
		g := "Person " + p + ": Arbeitsweg und Homeoffice"
		defs = append(defs,
			AngabeDef{x + "buerotage_vor", "Tage mit Fahrt zur ersten Tätigkeitsstätte vor dem Umzug (ohne Umzug: ganzes Jahr)", g, "zahl", "Ohne Urlaub, Krankheit, Homeoffice und Dienstreisen.", nil},
			AngabeDef{x + "km_vor", "Einfache Entfernung Wohnung – erste Tätigkeitsstätte vor dem Umzug (km)", g, "zahl", "Kürzeste Straßenverbindung; es zählen nur volle Kilometer.", nil},
			AngabeDef{x + "buerotage_nach", "Tage mit Fahrt zur ersten Tätigkeitsstätte ab dem Umzug", g, "zahl", "", nil},
			AngabeDef{x + "km_nach", "Einfache Entfernung ab dem Umzug (km)", g, "zahl", "", nil},
			AngabeDef{x + "verkehrsmittel", "Überwiegendes Verkehrsmittel", g, "auswahl", "Nur mit eigenem oder überlassenem Pkw gilt der Höchstbetrag von 4.500 € nicht.", []string{optPkw, optOeffentlich}},
			AngabeDef{x + "oepnv_kosten", "Tatsächliche Kosten öffentliche Verkehrsmittel (Jahr)", g, "betrag", "Werden angesetzt, soweit sie die Entfernungspauschale übersteigen.", nil},
			AngabeDef{x + "hotage_vor", "Homeoffice-Tage vor dem Umzug (ohne Umzug: ganzes Jahr)", g, "zahl", "Kalendertage, an denen die gesamte berufliche und betriebliche Tätigkeit überwiegend zu Hause ausgeübt und die erste Tätigkeitsstätte nicht aufgesucht wurde.", nil},
			AngabeDef{x + "hotage_umzugsmonat", "davon Homeoffice-Tage im Umzugsmonat vor dem Umzugstag", g, "zahl", "Wird für die Kombination Jahrespauschale/Tagespauschale gebraucht (Monatsprinzip).", nil},
			AngabeDef{x + "hotage_nach", "Homeoffice-Tage ab dem Umzug", g, "zahl", "", nil},
			AngabeDef{x + "hotage_mit_neben", "davon Tage, an denen auch die selbständige Nebentätigkeit zu Hause ausgeübt wurde", g, "zahl", "Nur für diese Tage darf die Tagespauschale der EÜR zugeordnet werden (Anleitung Anlage N 2025, Zeile 58/59). Leer: keine Zuordnung zur EÜR.", nil},
			AngabeDef{x + "anderer_arbeitsplatz", "Beim Arbeitgeber steht dauerhaft ein anderer Arbeitsplatz zur Verfügung", g, "jaNein", "Standard: ja. Nur bei 'nein' zählen auch Tage mit Fahrt zur Tätigkeitsstätte für die Tagespauschale (Anlage N Zeile 59).", nil},
			AngabeDef{x + "hotage_mit_fahrt", "Tage mit Homeoffice UND Fahrt zur Tätigkeitsstätte (nur wenn kein anderer Arbeitsplatz)", g, "zahl", "", nil},
			AngabeDef{x + "verpflegung_8h", "Dienstreisetage über 8 Stunden ohne Übernachtung", g, "zahl", "Nur die ersten 3 Monate an derselben auswärtigen Tätigkeitsstätte.", nil},
			AngabeDef{x + "verpflegung_anabreise", "An- und Abreisetage bei Dienstreisen mit Übernachtung", g, "zahl", "", nil},
			AngabeDef{x + "verpflegung_24h", "Dienstreisetage mit 24 Stunden Abwesenheit", g, "zahl", "", nil},
			AngabeDef{x + "verpflegung_kuerzung", "Kürzung wegen gestellter Mahlzeiten", g, "betrag", "Frühstück 5,60 €, Mittag/Abend je 11,20 €.", nil},
			AngabeDef{x + "verpflegung_ersatz", "Vom Arbeitgeber steuerfrei ersetzte Verpflegung", g, "betrag", "", nil},
			AngabeDef{x + "kv_zuschuss", "Anspruch auf steuerfreie Zuschüsse zur Krankenversicherung oder beitragsfreie Mitversicherung", "Person " + p + ": Vorsorge", "jaNein", "Arbeitnehmer mit Arbeitgeberzuschuss oder familienversichert: ja (Höchstbetrag 1.900 €). Nur Selbstzahler ohne Zuschuss: nein (2.800 €). Standard: ja.", nil},
			AngabeDef{x + "krankengeld", "Gesetzliche Krankenversicherung mit Anspruch auf Krankengeld", "Person " + p + ": Vorsorge", "jaNein", "Standard: ja (Arbeitnehmer). Dann werden die Beiträge um 4 % gekürzt.", nil},
		)
	}
	for i := 1; i <= maxKinder; i++ {
		x := fmt.Sprintf("kind%d.", i)
		g := fmt.Sprintf("Kind %d", i)
		defs = append(defs,
			AngabeDef{x + "geburtsdatum", "Geburtsdatum", g, "datum", "Leer lassen, wenn es dieses Kind nicht gibt.", nil},
			AngabeDef{x + "monate", "Monate 2026 mit Anspruch auf Kindergeld", g, "zahl", "Maßgeblich ist der Anspruch, nicht die Auszahlung. Leer: aus dem Geburtsdatum abgeleitet (nur für Kinder unter 18).", nil},
			AngabeDef{x + "haushalt", "Kind lebt im gemeinsamen Haushalt", g, "jaNein", "Voraussetzung für Kinderbetreuungskosten. Standard: ja.", nil},
		)
	}
	return defs
}

// angaben liest die Stammdaten typgerecht und sammelt Eingabefehler.
type angaben struct {
	werte  map[string]string
	defs   map[string]AngabeDef
	fehler []string
}

func neueAngaben(jahr int, werte map[string]string) *angaben {
	a := &angaben{werte: werte, defs: map[string]AngabeDef{}}
	for _, d := range AngabeDefs(jahr) {
		a.defs[d.Schluessel] = d
	}
	return a
}

func (a *angaben) roh(k string) string {
	if _, ok := a.defs[k]; !ok {
		panic("rechner: Angabe nicht definiert: " + k)
	}
	return strings.TrimSpace(a.werte[k])
}

func (a *angaben) gesetzt(k string) bool { return a.roh(k) != "" }

func (a *angaben) fehl(k, text string) {
	a.fehler = append(a.fehler, fmt.Sprintf("%s (%s): %s", a.defs[k].Titel, k, text))
}

// normDezimal wandelt deutsche oder englische Schreibweise in "1234.56".
func normDezimal(s string) (string, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	s = strings.TrimSuffix(strings.TrimSuffix(s, "€"), "EUR")
	if s == "" {
		return "", false
	}
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		if strings.Count(s, ",") > 1 {
			return "", false
		}
		s = strings.Replace(s, ",", ".", 1)
	} else if strings.Count(s, ".") > 1 {
		s = strings.ReplaceAll(s, ".", "")
	} else if i := strings.LastIndex(s, "."); i >= 0 && len(s)-i-1 == 3 {
		// "1.234" ist Tausenderpunkt
		s = s[:i] + s[i+1:]
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9') && c != '.' && c != '-' {
			return "", false
		}
	}
	return s, true
}

// zahl liest eine nicht-negative Dezimalzahl.
func (a *angaben) zahl(k string) *big.Rat {
	s := a.roh(k)
	if s == "" {
		return new(big.Rat)
	}
	n, ok := normDezimal(s)
	r, ok2 := new(big.Rat).SetString(n)
	if !ok || !ok2 {
		a.fehl(k, fmt.Sprintf("%q ist keine Zahl", s))
		return new(big.Rat)
	}
	if r.Sign() < 0 {
		a.fehl(k, "darf nicht negativ sein")
		return new(big.Rat)
	}
	return r
}

// ganzzahl liest eine nicht-negative ganze Zahl (Tage, Monate).
func (a *angaben) ganzzahl(k string) int64 {
	r := a.zahl(k)
	if !r.IsInt() {
		a.fehl(k, "muss eine ganze Zahl sein")
		return 0
	}
	return r.Num().Int64()
}

// volleKm liest Kilometer und rundet auf volle km ab.
func (a *angaben) volleKm(k string) int64 { return floorRat(a.zahl(k)) }

// betrag liest einen nicht-negativen Eurobetrag in Cent.
func (a *angaben) betrag(k string) int64 {
	r := a.zahl(k)
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() {
		a.fehl(k, "mehr als zwei Nachkommastellen")
		return 0
	}
	return r.Num().Int64()
}

func (a *angaben) datum(k string) (time.Time, bool) {
	s := a.roh(k)
	if s == "" {
		return time.Time{}, false
	}
	for _, f := range []string{"2006-01-02", "02.01.2006", "2.1.2006"} {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	a.fehl(k, fmt.Sprintf("%q ist kein Datum (JJJJ-MM-TT oder TT.MM.JJJJ)", s))
	return time.Time{}, false
}

func (a *angaben) jaNein(k string, standard bool) bool {
	switch strings.ToLower(a.roh(k)) {
	case "":
		return standard
	case "ja", "j", "true", "1", "wahr":
		return true
	case "nein", "n", "false", "0", "falsch":
		return false
	}
	a.fehl(k, "nur ja oder nein")
	return standard
}

// auswahl liest eine Auswahl und prüft sie gegen die Optionen.
func (a *angaben) auswahl(k, standard string) string {
	s := a.roh(k)
	if s == "" {
		return standard
	}
	opts := a.defs[k].Optionen
	for _, o := range opts {
		if strings.EqualFold(o, s) {
			return o
		}
	}
	a.fehl(k, fmt.Sprintf("%q ist keine der Optionen %s", s, strings.Join(opts, ", ")))
	return standard
}

// unbekannte liefert gesetzte Schlüssel, die nicht definiert sind.
func (a *angaben) unbekannte() []string {
	var out []string
	for k, v := range a.werte {
		if _, ok := a.defs[k]; !ok && strings.TrimSpace(v) != "" {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

func formatZahl(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	return strings.Replace(strings.TrimRight(strings.TrimRight(r.FloatString(2), "0"), "."), ".", ",", 1)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
