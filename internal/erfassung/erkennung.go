package erfassung

import (
	"regexp"
	"strings"
	"unicode"

	"steuer/internal/modell"
)

// Belegarten.
const (
	ArtLohnsteuerbescheinigung = "lohnsteuerbescheinigung"
	ArtSteuerbescheid          = "steuerbescheid"
	ArtRechnung                = "rechnung"
	ArtHonorarAusgang          = "honorarrechnung_ausgang"
	ArtSpende                  = "spendenbescheinigung"
	ArtVersicherung            = "versicherung"
	ArtNebenkosten             = "nebenkostenabrechnung"
	ArtMietvertrag             = "mietvertrag"
	ArtKinderbetreuung         = "kinderbetreuung"
	ArtHandwerker              = "handwerkerrechnung"
	ArtSonstiges               = "sonstiges"
)

type gewicht struct {
	wort   string
	punkte int
}

// artRegeln: Schlüsselwörter (Kleinschreibung, Wortanfang) mit Punkten.
var artRegeln = map[string][]gewicht{
	ArtLohnsteuerbescheinigung: {{"lohnsteuerbescheinigung", 12}, {"bruttoarbeitslohn", 6}, {"einbehaltene lohnsteuer", 6}, {"etin", 3}, {"steuerklasse", 2}},
	ArtSteuerbescheid: {{"einkommensteuerbescheid", 12}, {"steuerbescheid", 9}, {"festgesetzte einkommensteuer", 6}, {"zu versteuerndes einkommen", 5},
		{"gesamtbetrag der einkünfte", 4}, {"rechtsbehelfsbelehrung", 3}, {"einspruch", 2}, {"finanzamt", 2}, {"veranlagungszeitraum", 3}},
	ArtSpende:          {{"spendenbescheinigung", 12}, {"zuwendungsbestätigung", 12}, {"bestätigung über zuwendungen", 12}, {"freistellungsbescheid", 4}, {"gemeinnützig", 3}, {"mitgliedsbeitrag", 3}, {"spende", 3}},
	ArtMietvertrag:     {{"mietvertrag", 12}, {"vermieter", 3}, {"mieter", 2}, {"kaution", 3}, {"mietbeginn", 3}, {"mietzins", 3}},
	ArtNebenkosten:     {{"nebenkostenabrechnung", 12}, {"betriebskostenabrechnung", 12}, {"heizkostenabrechnung", 10}, {"abrechnungszeitraum", 3}, {"nebenkosten", 3}, {"vorauszahlungen", 3}},
	ArtVersicherung:    {{"versicherungsschein", 9}, {"versicherungsbeitrag", 7}, {"beitragsrechnung", 7}, {"beitragsbescheinigung", 9}, {"versicherungsnummer", 4}, {"versicherungsnehmer", 5}, {"haftpflichtversicherung", 5}, {"krankenversicherung", 4}, {"police", 3}, {"versicherungsbeginn", 3}, {"jahresbeitrag", 3}},
	ArtKinderbetreuung: {{"kinderbetreuung", 9}, {"kindergarten", 7}, {"kita", 6}, {"betreuungsvertrag", 7}, {"tagesmutter", 7}, {"betreuungskosten", 7}, {"elternbeitrag", 7}, {"hort", 3}},
	ArtHandwerker:      {{"handwerkerleistung", 9}, {"handwerker", 6}, {"arbeitskosten", 6}, {"lohnkosten", 5}, {"lohnanteil", 6}, {"fachbetrieb", 3}, {"reparatur", 3}, {"montage", 2}, {"installation", 2}, {"arbeitsstunden", 3}, {"malerarbeiten", 5}, {"§ 35a", 8}},
	ArtHonorarAusgang:  {{"honorarrechnung", 12}, {"honorar", 8}, {"kleinunternehmer", 2}},
	ArtRechnung:        {{"rechnung", 2}, {"rechnungsnummer", 2}, {"rechnungsdatum", 2}, {"rechnungsbetrag", 2}, {"invoice", 3}, {"mwst", 1}, {"umsatzsteuer", 1}, {"ust-idnr", 1}, {"gesamtbetrag", 1}},
}

// artReihenfolge entscheidet bei Gleichstand (spezifisch vor allgemein).
var artReihenfolge = []string{ArtLohnsteuerbescheinigung, ArtSteuerbescheid, ArtSpende, ArtMietvertrag, ArtNebenkosten, ArtKinderbetreuung, ArtHandwerker, ArtHonorarAusgang, ArtVersicherung, ArtRechnung}

const artSchwelle = 4

func enthaeltWortanfang(text, wort string) bool {
	from := 0
	for {
		i := strings.Index(text[from:], wort)
		if i < 0 {
			return false
		}
		i += from
		if i == 0 {
			return true
		}
		r := []rune(text[:i])
		if !unicode.IsLetter(r[len(r)-1]) {
			return true
		}
		from = i + len(wort)
	}
}

// ErkenneArt bestimmt die Belegart aus dem Text.
func ErkenneArt(text string) string {
	low := strings.ToLower(text)
	best, bestP := ArtSonstiges, artSchwelle-1
	for _, art := range artReihenfolge {
		p := 0
		for _, g := range artRegeln[art] {
			if enthaeltWortanfang(low, g.wort) {
				p += g.punkte
			}
		}
		if p > bestP {
			best, bestP = art, p
		}
	}
	return best
}

func zeilenVon(text string) []string {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", " ", " ", " ", " ", "\t", " ").Replace(text)
	z := strings.Split(text, "\n")
	for i := range z {
		z[i] = strings.TrimSpace(z[i])
	}
	return z
}

// ---- Datum ----

var (
	datumLabelStark = regexp.MustCompile(`(?i)(rechnungsdatum|belegdatum|ausstellungsdatum|ausgestellt am|bescheiddatum|bescheid vom|datum des bescheids|datum der ausstellung|datum der bestätigung|rechnung vom|beleg vom)`)
	datumLabelDatum = regexp.MustCompile(`(?i)\bdatum\b`)
	datumAusschluss = regexp.MustCompile(`(?i)(geburt|fällig|zahlbar|zahlungsziel|zahlungsfrist|leistungs|liefer|zeitraum|gültig|vertragsbeginn|laufzeit|mietbeginn|versicherungsbeginn|\bbis\b|\bab\b|\bvom\b.*\bbis\b|verbrauch|ablesung)`)
)

// erkenneBelegdatum liefert JJJJ-MM-TT oder "". mitRueckfall erlaubt das erste
// beliebige Datum im Text.
func erkenneBelegdatum(zeilen []string, mitRueckfall bool) string {
	suche := func(re *regexp.Regexp) string {
		for i, z := range zeilen {
			if !re.MatchString(z) {
				continue
			}
			if d := findeDaten(z); len(d) > 0 {
				loc := re.FindStringIndex(z)
				for _, t := range d { // Datum nach der Beschriftung bevorzugen
					if t.Pos >= loc[0] {
						return t.ISO
					}
				}
				return d[0].ISO
			}
			if i+1 < len(zeilen) {
				if d := findeDaten(zeilen[i+1]); len(d) > 0 {
					return d[0].ISO
				}
			}
		}
		return ""
	}
	if d := suche(datumLabelStark); d != "" {
		return d
	}
	// "Datum" allein, aber nicht in ausgeschlossenen Zeilen
	for i, z := range zeilen {
		if !datumLabelDatum.MatchString(z) || datumAusschluss.MatchString(z) {
			continue
		}
		if d := findeDaten(z); len(d) > 0 {
			return d[0].ISO
		}
		if i+1 < len(zeilen) && !datumAusschluss.MatchString(zeilen[i+1]) {
			if d := findeDaten(zeilen[i+1]); len(d) > 0 {
				return d[0].ISO
			}
		}
	}
	if !mitRueckfall {
		return ""
	}
	for _, z := range zeilen {
		if datumAusschluss.MatchString(z) {
			continue
		}
		if d := findeDaten(z); len(d) > 0 {
			return d[0].ISO
		}
	}
	return ""
}

// ---- Steuerjahr aus Bescheid / Bescheinigung ----

var (
	jahrZeitraum = regexp.MustCompile(`(\d{1,2}\.\d{1,2}\.)\s*(?:-|–|bis)\s*\d{1,2}\.\d{1,2}\.((?:19|20)\d\d)`)
	jahrLabel    = regexp.MustCompile(`(?i)(?:veranlagungszeitraum|steuerjahr|kalenderjahr|bescheinigungszeitraum|für\s+das\s+jahr|veranlagung\s+für|bescheinigung\s+für|bescheid\s+für)\D{0,15}?((?:19|20)\d\d)\b`)
	jahrTitel    = regexp.MustCompile(`(?i)(?:einkommensteuer(?:bescheid)?|lohnsteuerbescheinigung|steuerbescheid)\s+(?:für\s+)?((?:19|20)\d\d)\b`)
)

func jahrAusDokument(text string) int {
	for _, re := range []*regexp.Regexp{jahrZeitraum, jahrLabel, jahrTitel} {
		if m := re.FindStringSubmatch(text); m != nil {
			return atoi(m[len(m)-1])
		}
	}
	return 0
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ---- Aussteller ----

var (
	ausstellerLabel  = regexp.MustCompile(`(?i)^(?:name des )?(aussteller|absender|arbeitgeber|vermieter|versicherer|versicherungsgesellschaft|zuwendungsempfänger|zuwendungsempfaenger|einrichtung|rechnungssteller|leistungserbringer|träger)(?:\s+und anschrift)?\s*:?\s*(.*)$`)
	finanzamtName    = regexp.MustCompile(`Finanzamt\s+[\p{Lu}][\p{L}\-]+`)
	firmenForm       = regexp.MustCompile(`(?i)(\bgmbh\b|\bag\b|\bkg\b|\bohg\b|\bug\b|\be\.\s?v\.|\bgbr\b|\bmbh\b|\bse\b|stiftung|\bverein\b|versicherung|\bbank\b|sparkasse|krankenkasse|stadtwerke|\bkita\b|kindergarten|& co)`)
	ausstellerGeneri = regexp.MustCompile(`(?i)^(rechnung|datum|seite|invoice|quittung|beleg|bescheid|kundennummer|rechnungsnummer|an:|von:|betreff|ihr zeichen|\d)`)
)

func kuerzeAussteller(s string) string {
	s = strings.TrimSpace(s)
	for _, trenner := range []string{" · ", " | ", " • ", ", ", " - ", "  "} {
		if i := strings.Index(s, trenner); i > 2 {
			s = s[:i]
		}
	}
	s = strings.Trim(s, " ,;:-–|·")
	r := []rune(s)
	if len(r) > 60 {
		s = strings.TrimSpace(string(r[:60]))
	}
	return s
}

func erkenneAussteller(zeilen []string, text, art string) string {
	if art == ArtSteuerbescheid {
		if m := finanzamtName.FindString(text); m != "" {
			return strings.Join(strings.Fields(m), " ")
		}
	}
	for i, z := range zeilen {
		m := ausstellerLabel.FindStringSubmatch(z)
		if m == nil {
			continue
		}
		wert := strings.TrimSpace(m[2])
		if wert == "" {
			for k := i + 1; k < len(zeilen) && k <= i+2; k++ {
				if zeilen[k] != "" {
					wert = zeilen[k]
					break
				}
			}
		}
		if a := kuerzeAussteller(wert); len([]rune(a)) >= 3 && !ausstellerGeneri.MatchString(a) {
			return a
		}
	}
	for i, z := range zeilen {
		if i >= 25 {
			break
		}
		if len(z) <= 80 && firmenForm.MatchString(z) && !ausstellerGeneri.MatchString(z) {
			if a := kuerzeAussteller(z); len([]rune(a)) >= 3 {
				return a
			}
		}
	}
	n := 0
	for _, z := range zeilen {
		if z == "" {
			continue
		}
		if n++; n > 5 {
			break
		}
		r := []rune(z)
		if len(r) >= 3 && len(r) <= 60 && unicode.IsLetter(r[0]) && !ausstellerGeneri.MatchString(z) && len(zahlenInZeile(z, true)) == 0 && len(findeDaten(z)) == 0 {
			return kuerzeAussteller(z)
		}
	}
	return ""
}

// ---- Gesamtbetrag ----

type betragKandidat struct {
	tier   int
	cent   int64
	sicher bool
}

var (
	betragAusschluss = regexp.MustCompile(`(?i)(netto|zwischensumme|zzgl|enthaltene|^\s*(mwst|ust|umsatzsteuer|mehrwertsteuer)|skonto|anzahlung|rabatt)`)
	betragTier0      = map[string][]string{
		ArtHandwerker: {"arbeitskosten", "lohnkosten", "arbeitsleistung", "lohnanteil", "arbeitsanteil", "arbeitslohn"},
	}
	betragTier1 = []string{"zu zahlender betrag", "zahlbetrag", "gesamtbetrag", "rechnungsbetrag", "endbetrag", "gesamtsumme", "summe brutto", "bruttobetrag", "rechnungssumme", "zu zahlen", "gesamt brutto", "brutto gesamt", "spendenbetrag", "betrag der zuwendung", "zuwendung", "jahresbeitrag", "gesamtbeitrag", "elternbeitrag", "betreuungskosten"}
	betragTier2 = []string{"brutto", "summe", "total", "gesamt", "betrag", "beitrag", "spende"}
)

func enthaeltEines(low string, woerter []string) bool {
	for _, w := range woerter {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

func betraegeZeileOderNaechste(zeilen []string, i int, ganzzahlOk bool) []betragTreffer {
	if b := zahlenInZeile(zeilen[i], ganzzahlOk); len(b) > 0 {
		return b
	}
	if i+1 < len(zeilen) {
		n := zeilen[i+1]
		buchst := 0
		for _, r := range n {
			if unicode.IsLetter(r) {
				buchst++
			}
		}
		if buchst <= 4 {
			return zahlenInZeile(n, ganzzahlOk)
		}
	}
	return nil
}

// erkenneGesamtbetrag liefert den Betrag und gegebenenfalls einen Hinweis.
func erkenneGesamtbetrag(zeilen []string, art string) (int64, string) {
	var kand []betragKandidat
	for i, z := range zeilen {
		low := strings.ToLower(z)
		if low == "" || betragAusschluss.MatchString(low) {
			continue
		}
		tier := 0
		switch {
		case enthaeltEines(low, betragTier1):
			tier = 2
		case enthaeltEines(low, betragTier2):
			tier = 3
		default:
			continue
		}
		b := betraegeZeileOderNaechste(zeilen, i, false)
		if len(b) == 0 {
			continue
		}
		l := b[len(b)-1]
		kand = append(kand, betragKandidat{tier, l.Cent, l.Sicher})
	}
	if len(kand) > 0 {
		bestTier := 99
		for _, k := range kand {
			if k.tier < bestTier {
				bestTier = k.tier
			}
		}
		var sel []betragKandidat
		for _, k := range kand {
			if k.tier == bestTier {
				sel = append(sel, k)
			}
		}
		verschieden := map[int64]bool{}
		for _, k := range sel {
			verschieden[k.cent] = true
		}
		if bestTier == 3 { // nur schwache Stichwörter: größter Betrag, unsicher
			g := sel[0]
			for _, k := range sel {
				if k.cent > g.cent {
					g = k
				}
			}
			return g.cent, "Betrag unsicher erkannt - bitte prüfen"
		}
		g := sel[len(sel)-1]
		h := ""
		if len(verschieden) > 1 {
			h = "Mehrere mögliche Gesamtbeträge gefunden (verwendet: letzter) - bitte prüfen"
		} else if !g.sicher {
			h = "Betrag unsicher erkannt (Zahlenformat uneindeutig) - bitte prüfen"
		}
		return g.cent, h
	}
	// Rückfall: größter Betrag mit Dezimalstellen
	var max int64
	for _, z := range zeilen {
		for _, b := range zahlenInZeile(z, false) {
			if b.Cent > max {
				max = b.Cent
			}
		}
	}
	if max > 0 {
		return max, "Betrag unsicher erkannt (höchster Betrag im Dokument) - bitte prüfen"
	}
	return 0, "Kein Betrag erkannt - bitte manuell erfassen"
}

// ---- Lohnsteuerbescheinigung ----

// lstbNummer ordnet eine Zeile anhand ihrer Beschriftung einer Nummer zu.
func lstbNummer(low string) string {
	has := func(w ...string) bool {
		for _, x := range w {
			if !strings.Contains(low, x) {
				return false
			}
		}
		return true
	}
	switch {
	case strings.Contains(low, "knappschaft"):
		return ""
	case has("einbehaltene lohnsteuer"):
		return "4"
	case has("solidarit") && !strings.Contains(low, "kirchensteuer"):
		return "5"
	case has("bruttoarbeitslohn") && !strings.Contains(low, "einbehalten"):
		return "3"
	case has("arbeitgeberanteil", "rentenversicherung"):
		return "22a"
	case has("arbeitnehmeranteil", "rentenversicherung"):
		return "23a"
	case has("arbeitslosenversicherung"):
		return "27"
	case has("pflegeversicherung"):
		return "26"
	case has("krankenversicherung") && !strings.Contains(low, "arbeitgeber") && !strings.Contains(low, "privat"):
		return "25"
	case has("arbeitgeberleistung") || (has("steuerfrei", "fahrten")):
		return "17"
	}
	return ""
}

// lohnsteuerWerte liest die Nummern 3, 4, 5, 17, 22a, 23a, 25, 26, 27.
func lohnsteuerWerte(zeilen []string) map[string]int64 {
	out := map[string]int64{}
	for i, z := range zeilen {
		nr := lstbNummer(strings.ToLower(z))
		if nr == "" {
			continue
		}
		if _, da := out[nr]; da {
			continue
		}
		b := zahlenInZeile(z, false)
		for k := i + 1; len(b) == 0 && k <= i+2 && k < len(zeilen); k++ {
			if lstbNummer(strings.ToLower(zeilen[k])) != "" {
				break
			}
			b = zahlenInZeile(zeilen[k], false)
		}
		if len(b) == 0 {
			continue
		}
		out[nr] = b[len(b)-1].Cent
	}
	return out
}

var lstbReihenfolge = []string{"3", "4", "5", "17", "22a", "23a", "25", "26", "27"}

// ---- Steuerbescheid ----

type bescheidRegel struct {
	schluessel string
	passt      func(low string) bool
	vorzeichen int64 // 0 = Betrag wie gelesen, -1 = negativ (Nachzahlung)
}

func beginntMit(low, w string) bool {
	return strings.HasPrefix(strings.TrimLeft(low, "0123456789.) "), w)
}

var bescheidRegeln = []bescheidRegel{
	{"bescheid.zve", func(l string) bool { return strings.Contains(l, "zu versteuerndes einkommen") }, 1},
	{"bescheid.est", func(l string) bool {
		return !strings.Contains(l, "bescheid") && !strings.Contains(l, "vorauszahlung") && !strings.Contains(l, "abzüglich") &&
			(strings.Contains(l, "festgesetzte einkommensteuer") || beginntMit(l, "einkommensteuer"))
	}, 1},
	{"bescheid.soli", func(l string) bool {
		return strings.Contains(l, "solidaritätszuschlag") && !strings.Contains(l, "bescheid") && !strings.Contains(l, "vorauszahlung") && !strings.Contains(l, "einbehalten") &&
			(strings.Contains(l, "festgesetzt") || beginntMit(l, "solidaritätszuschlag"))
	}, 1},
	{"bescheid.werbungskosten", func(l string) bool {
		return strings.Contains(l, "werbungskosten") && !strings.Contains(l, "pauschbetrag")
	}, 1},
	{"bescheid.eink_nichtselbst", func(l string) bool { return strings.Contains(l, "einkünfte aus nichtselbst") }, 1},
	{"bescheid.eink_selbst", func(l string) bool {
		return strings.Contains(l, "einkünfte aus selbst") && !strings.Contains(l, "nichtselbst")
	}, 1},
	{"bescheid.gde", func(l string) bool { return strings.Contains(l, "gesamtbetrag der einkünfte") }, 1},
	{"bescheid.erstattung", func(l string) bool {
		return strings.Contains(l, "erstattung") && !strings.Contains(l, "bescheid") && !strings.Contains(l, "kostenerstattung")
	}, 1},
	{"bescheid.erstattung", func(l string) bool {
		return strings.Contains(l, "nachzahlung") || strings.Contains(l, "abschlusszahlung") || strings.Contains(l, "nachzuzahlen")
	}, -1},
}

// bescheidWerte liest die wichtigen Größen eines Steuerbescheids (je Schlüssel
// der erste Treffer). Erstattung ist positiv, Nachzahlung negativ.
func bescheidWerte(zeilen []string, jahr int) []modell.Vorjahreswert {
	var out []modell.Vorjahreswert
	gesehen := map[string]bool{}
	for i, z := range zeilen {
		low := strings.ToLower(z)
		for _, r := range bescheidRegeln {
			if gesehen[r.schluessel] || !r.passt(low) {
				continue
			}
			b := betraegeZeileOderNaechste(zeilen, i, true)
			if len(b) == 0 {
				continue
			}
			c := b[len(b)-1].Cent
			if r.vorzeichen < 0 {
				if c > 0 {
					c = -c
				}
			}
			out = append(out, modell.Vorjahreswert{Steuerjahr: jahr, Schluessel: r.schluessel, BetragCent: c})
			gesehen[r.schluessel] = true
			break
		}
	}
	return out
}

// ---- Kategorie je Beleg ----

var kategorieStichworte = []struct {
	kategorie string
	woerter   []string
}{
	{"agb.krankheit", []string{"apotheke", "zahnarzt", "arzt ", "arztrechnung", "rezept", "brille", "krankenhaus", "physiotherapie"}},
	{"n.fortbildung", []string{"seminar", "fortbildung", "lehrgang", "fachbuch", "kursgebühr", "webinar"}},
	{"n.telefon_internet", []string{"mobilfunk", "internetanschluss", "telefonrechnung", "dsl"}},
	{"n.arbeitsmittel", []string{"laptop", "notebook", "monitor", "tastatur", "schreibtisch", "bürostuhl", "drucker", "headset", "arbeitsmittel"}},
}

// vorschlagKategorie wählt die Kategorie für Belege ohne eigene Auswertung.
func vorschlagKategorie(art, text string) (kat, notiz string) {
	low := strings.ToLower(text)
	switch art {
	case ArtHandwerker:
		return "h.handwerker", ""
	case ArtSpende:
		return "sa.spenden", ""
	case ArtKinderbetreuung:
		return "kind.betreuung", ""
	case ArtHonorarAusgang:
		return "s.einnahmen", ""
	case ArtNebenkosten:
		return "wohnung.nebenkosten", ""
	case ArtVersicherung:
		if strings.Contains(low, "krankenversicherung") || strings.Contains(low, "pflegeversicherung") {
			return "vor.kranken_pflege", "Art der Versicherung bitte prüfen"
		}
		if strings.Contains(low, "hausrat") {
			return "wohnung.versicherung", "Art der Versicherung bitte prüfen"
		}
		return "vor.sonstige", "Art der Versicherung bitte prüfen"
	case ArtRechnung, ArtSonstiges:
		for _, k := range kategorieStichworte {
			if enthaeltEines(low, k.woerter) {
				return k.kategorie, "Kategorie aus Stichworten vermutet - bitte prüfen"
			}
		}
	}
	return "", ""
}

func kategorieTitel(schluessel string) string {
	for _, k := range modell.Kategorien {
		if k.Schluessel == schluessel {
			return k.Titel
		}
	}
	return schluessel
}
