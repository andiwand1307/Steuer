package erfassung

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// betragTreffer ist ein in einer Textzeile gefundener Geldbetrag.
type betragTreffer struct {
	Cent    int64
	Sicher  bool   // false bei uneindeutigem Zahlenformat (z.B. "1.234" oder "1,234")
	Roh     string // Fundstelle im Text
	MitEuro bool   // direkt von €/EUR gefolgt
}

// ParseBetrag liest einen Geldbetrag in deutschem (1.234,56), schlichtem
// (1234,56) oder englischem Format (1,234.56) und liefert Cent. sicher ist
// false, wenn das Format mehrdeutig ist (z.B. "1.234" = 1234 Euro oder
// 1,234 Euro). ok ist false, wenn der Text kein Betrag ist.
func ParseBetrag(s string) (cent int64, sicher bool, ok bool) {
	s = strings.TrimSpace(s)
	for _, w := range []string{"€", "EUR", "eur", "Euro", "euro"} {
		s = strings.ReplaceAll(s, w, "")
	}
	r := strings.NewReplacer(" ", "", " ", "", " ", "", "'", "", "’", "")
	s = r.Replace(s)
	neg := false
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "−") || strings.HasPrefix(s, "–") {
		neg = true
		s = strings.TrimLeft(s, "-−–")
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if strings.HasSuffix(s, "-") || strings.HasSuffix(s, "−") {
		neg = true
		s = strings.TrimRight(s, "-−")
	}
	if s == "" || len(s) > 18 {
		return 0, false, false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9') && c != '.' && c != ',' {
			return 0, false, false
		}
	}
	if s[0] < '0' || s[0] > '9' || s[len(s)-1] < '0' || s[len(s)-1] > '9' {
		return 0, false, false
	}
	sicher = true
	var ganz, dez string
	lastDot, lastComma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	switch {
	case lastDot >= 0 && lastComma >= 0:
		// Das letzte Trennzeichen ist das Dezimaltrennzeichen.
		var tsd, dz byte = '.', ','
		pos := lastComma
		if lastDot > lastComma {
			tsd, dz = ',', '.'
			pos = lastDot
		}
		ganz, dez = s[:pos], s[pos+1:]
		if strings.IndexByte(dez, tsd) >= 0 || strings.IndexByte(dez, dz) >= 0 || len(dez) < 1 || len(dez) > 2 {
			return 0, false, false
		}
		if !tausenderOK(ganz, tsd) {
			return 0, false, false
		}
		ganz = strings.ReplaceAll(ganz, string(tsd), "")
	case lastComma >= 0:
		if strings.Count(s, ",") > 1 {
			if !tausenderOK(s, ',') {
				return 0, false, false
			}
			ganz = strings.ReplaceAll(s, ",", "")
			sicher = false
			break
		}
		ganz, dez = s[:lastComma], s[lastComma+1:]
		if len(dez) == 3 && len(ganz) <= 3 {
			ganz += dez // "1,234" - eher Tausendertrennzeichen
			dez = ""
			sicher = false
		} else if len(dez) < 1 || len(dez) > 2 {
			return 0, false, false
		}
	case lastDot >= 0:
		if strings.Count(s, ".") > 1 {
			if !tausenderOK(s, '.') {
				return 0, false, false
			}
			ganz = strings.ReplaceAll(s, ".", "")
			break
		}
		ganz, dez = s[:lastDot], s[lastDot+1:]
		if len(dez) == 3 && len(ganz) <= 3 {
			ganz += dez // "1.234" - deutsches Tausenderformat, aber mehrdeutig
			dez = ""
			sicher = false
		} else if len(dez) < 1 || len(dez) > 2 {
			return 0, false, false
		}
	default:
		ganz = s
	}
	if ganz == "" {
		ganz = "0"
	}
	g, err := strconv.ParseInt(ganz, 10, 64)
	if err != nil {
		return 0, false, false
	}
	var d int64
	if dez != "" {
		if len(dez) == 1 {
			dez += "0"
		}
		d, err = strconv.ParseInt(dez, 10, 64)
		if err != nil {
			return 0, false, false
		}
	}
	cent = g*100 + d
	if neg {
		cent = -cent
	}
	return cent, sicher, true
}

// tausenderOK prüft Gruppen "1.234.567": erste Gruppe 1-3 Ziffern, Rest je 3.
func tausenderOK(s string, sep byte) bool {
	teile := strings.Split(s, string(sep))
	for i, t := range teile {
		if t == "" {
			return false
		}
		if i == 0 && len(t) > 3 && len(teile) > 1 {
			return false
		}
		if i > 0 && len(t) != 3 {
			return false
		}
	}
	return true
}

var zahlToken = regexp.MustCompile(`[-−]?\d[\d.,']*[-−]?`)
var euroNach = regexp.MustCompile(`^\s{0,2}(?:€|EUR\b|Euro\b|eur\b)`)

// zahlenInZeile findet Geldbeträge in einer Zeile. Ganzzahlen ohne
// Tausenderpunkt und ohne Währung gelten nur mit ganzzahlOk als Betrag
// (Jahreszahlen 1990-2100 nie).
func zahlenInZeile(zeile string, ganzzahlOk bool) []betragTreffer {
	var out []betragTreffer
	for _, loc := range zahlToken.FindAllStringIndex(zeile, -1) {
		roh := zeile[loc[0]:loc[1]]
		// Vorher: kein Buchstabe/Ziffer direkt davor ("A4", "Nr.22a"), kein § / Nr.
		if loc[0] > 0 {
			vorher := []rune(zeile[:loc[0]])
			lr := vorher[len(vorher)-1]
			if unicode.IsLetter(lr) || unicode.IsDigit(lr) {
				continue
			}
			vt := strings.TrimRight(strings.ToLower(zeile[:loc[0]]), "  ")
			if strings.HasSuffix(vt, "§") || strings.HasSuffix(vt, "nr.") || strings.HasSuffix(vt, "nr") || strings.HasSuffix(vt, "nummer") || strings.HasSuffix(vt, "nummern") || strings.HasSuffix(vt, "ziffer") || strings.HasSuffix(vt, "zeile") {
				continue
			}
		}
		rest := zeile[loc[1]:]
		if rest != "" {
			nr, _ := firstRune(rest)
			if nr == '%' || (unicode.IsLetter(nr) && !euroNach.MatchString(rest)) {
				continue
			}
		}
		rohT := strings.TrimRight(roh, ".,'")
		mitEuro := euroNach.MatchString(zeile[loc[0]+len(rohT):])
		c, sicher, ok := ParseBetrag(rohT)
		if !ok {
			continue
		}
		if !strings.ContainsAny(rohT, ".,") && !mitEuro {
			if !ganzzahlOk {
				continue
			}
			if c/100 >= 1990 && c/100 <= 2100 {
				continue
			}
		}
		out = append(out, betragTreffer{Cent: c, Sicher: sicher, Roh: roh, MitEuro: mitEuro})
	}
	return out
}

func firstRune(s string) (rune, int) {
	for i, r := range s {
		return r, i
	}
	return 0, 0
}

// ---- Datum ----

var monatsNamen = map[string]int{
	"januar": 1, "jan": 1, "jänner": 1, "februar": 2, "feb": 2, "märz": 3, "maerz": 3, "mär": 3, "mrz": 3, "mar": 3,
	"april": 4, "apr": 4, "mai": 5, "juni": 6, "jun": 6, "juli": 7, "jul": 7, "august": 8, "aug": 8,
	"september": 9, "sep": 9, "sept": 9, "oktober": 10, "okt": 10, "oct": 10, "november": 11, "nov": 11,
	"dezember": 12, "dez": 12, "dec": 12,
}

var (
	datumNum   = regexp.MustCompile(`(?:^|[^\d.])(\d{1,2})[./](\d{1,2})[./](\d{4}|\d{2})(?:[^\d]|$)`)
	datumIso   = regexp.MustCompile(`(?:^|[^\d])(\d{4})-(\d{2})-(\d{2})(?:[^\d]|$)`)
	datumWort  = regexp.MustCompile(`(?i)(?:^|[^\d])(\d{1,2})\.?\s*(januar|jänner|februar|märz|maerz|april|mai|juni|juli|august|september|oktober|november|dezember|jan|feb|mär|mrz|apr|jun|jul|aug|sept|sep|okt|nov|dez)\.?\s*(\d{4})(?:[^\d]|$)`)
	jahrZweier = regexp.MustCompile(`^\d{2}$`)
)

func datumGueltig(j, m, t int) (string, bool) {
	if j < 1990 || j > 2100 || m < 1 || m > 12 || t < 1 || t > 31 {
		return "", false
	}
	d := time.Date(j, time.Month(m), t, 0, 0, 0, 0, time.UTC)
	if d.Day() != t || int(d.Month()) != m {
		return "", false
	}
	return d.Format("2006-01-02"), true
}

// datumTreffer ist ein Datum mit Position im Text.
type datumTreffer struct {
	ISO string
	Pos int
}

// findeDaten liefert alle Datumsangaben einer Zeile in Textreihenfolge.
func findeDaten(zeile string) []datumTreffer {
	var out []datumTreffer
	for _, m := range datumNum.FindAllStringSubmatchIndex(zeile, -1) {
		t, _ := strconv.Atoi(zeile[m[2]:m[3]])
		mo, _ := strconv.Atoi(zeile[m[4]:m[5]])
		js := zeile[m[6]:m[7]]
		j, _ := strconv.Atoi(js)
		if jahrZweier.MatchString(js) {
			if j > 69 {
				j += 1900
			} else {
				j += 2000
			}
		}
		if iso, ok := datumGueltig(j, mo, t); ok {
			out = append(out, datumTreffer{iso, m[2]})
		}
	}
	for _, m := range datumIso.FindAllStringSubmatchIndex(zeile, -1) {
		j, _ := strconv.Atoi(zeile[m[2]:m[3]])
		mo, _ := strconv.Atoi(zeile[m[4]:m[5]])
		t, _ := strconv.Atoi(zeile[m[6]:m[7]])
		if iso, ok := datumGueltig(j, mo, t); ok {
			out = append(out, datumTreffer{iso, m[2]})
		}
	}
	for _, m := range datumWort.FindAllStringSubmatchIndex(zeile, -1) {
		t, _ := strconv.Atoi(zeile[m[2]:m[3]])
		mo := monatsNamen[strings.ToLower(zeile[m[4]:m[5]])]
		j, _ := strconv.Atoi(zeile[m[6]:m[7]])
		if iso, ok := datumGueltig(j, mo, t); ok {
			out = append(out, datumTreffer{iso, m[2]})
		}
	}
	// nach Position sortieren (kleine Listen)
	for i := 1; i < len(out); i++ {
		for k := i; k > 0 && out[k].Pos < out[k-1].Pos; k-- {
			out[k], out[k-1] = out[k-1], out[k]
		}
	}
	return out
}

// ParseDatum liest ein einzelnes Datum (TT.MM.JJJJ, TT.MM.JJ, JJJJ-MM-TT,
// "12. März 2024") und liefert JJJJ-MM-TT.
func ParseDatum(s string) (string, bool) {
	d := findeDaten(strings.TrimSpace(s))
	if len(d) == 0 {
		return "", false
	}
	return d[0].ISO, true
}
