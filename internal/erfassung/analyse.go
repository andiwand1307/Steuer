package erfassung

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"steuer/internal/modell"
)

// Ergebnis ist das Resultat der Analyse einer Datei. Alle Positionen und
// Vorjahreswerte sind Vorschläge und gehen erst nach Bestätigung in eine
// Berechnung ein.
type Ergebnis struct {
	Beleg          modell.Beleg
	Positionen     []modell.Position
	Vorjahreswerte []modell.Vorjahreswert
}

func sha256Datei(pfad string) (string, error) {
	f, err := os.Open(pfad)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Analysiere liest eine Datei und erkennt Belegart, Datum, Aussteller, Betrag
// und Steuerjahr. Es gibt keine Datenbank- oder Dateisystemänderung. Beleg.ID
// und Beleg.Datei (nur der Dateiname) werden vom Aufrufer vervollständigt.
func Analysiere(pfad string, standardJahr int) (Ergebnis, error) {
	st, err := os.Stat(pfad)
	if err != nil {
		return Ergebnis{}, err
	}
	if st.IsDir() {
		return Ergebnis{}, fmt.Errorf("%s ist ein Ordner", pfad)
	}
	hash, err := sha256Datei(pfad)
	if err != nil {
		return Ergebnis{}, err
	}
	te := textGewinnen(pfad)
	return auswerten(filepath.Base(pfad), hash, te, standardJahr), nil
}

// auswerten ist die reine Textauswertung (ohne Dateizugriff).
func auswerten(dateiname, hash string, te textErgebnis, standardJahr int) Ergebnis {
	text := te.Text
	if r := []rune(text); len(r) > maxTextZeichen {
		text = string(r[:maxTextZeichen])
	}
	zeilen := zeilenVon(text)
	var hinweise []string
	if te.Hinweis != "" {
		hinweise = append(hinweise, te.Hinweis)
	}

	art := ErkenneArt(text)
	b := modell.Beleg{
		SHA256: hash, Datei: dateiname, ErfasstAm: time.Now(), Status: modell.StatusNeu,
		Art: art, Text: text,
	}
	var res Ergebnis

	mitRueckfall := art != ArtLohnsteuerbescheinigung
	b.Belegdatum = erkenneBelegdatum(zeilen, mitRueckfall)
	b.Aussteller = erkenneAussteller(zeilen, text, art)

	// Steuerjahr
	jahr := 0
	if art == ArtLohnsteuerbescheinigung || art == ArtSteuerbescheid {
		jahr = jahrAusDokument(text)
	}
	if jahr == 0 && b.Belegdatum != "" {
		jahr = atoi(b.Belegdatum[:4])
	}
	if jahr == 0 {
		jahr = standardJahr
		if strings.TrimSpace(text) != "" {
			hinweise = append(hinweise, fmt.Sprintf("Steuerjahr nicht erkannt - Standardjahr %d verwendet", standardJahr))
		}
	}
	b.Steuerjahr = jahr
	if art == ArtSonstiges && strings.TrimSpace(text) != "" {
		hinweise = append(hinweise, "Belegart nicht erkannt")
	}
	if art == ArtHonorarAusgang {
		hinweise = append(hinweise, "Als Ausgangsrechnung (Honorar) erkannt - bitte prüfen")
	}

	pos := func(kat, bez string, cent int64, notiz string) modell.Position {
		return modell.Position{
			Steuerjahr: jahr, Kategorie: kat, Bezeichnung: bez, BetragCent: cent,
			Datum: b.Belegdatum, Notiz: notiz, Bestaetigt: false, Vorschlag: true,
		}
	}

	switch art {
	case ArtLohnsteuerbescheinigung:
		w := lohnsteuerWerte(zeilen)
		for _, nr := range lstbReihenfolge {
			if c, ok := w[nr]; ok && c != 0 {
				kat := "lstb." + nr
				res.Positionen = append(res.Positionen, pos(kat, kategorieTitel(kat), c, ""))
			}
		}
		b.BetragCent = w["3"]
		if len(w) == 0 {
			hinweise = append(hinweise, "Keine Werte der Lohnsteuerbescheinigung erkannt - bitte manuell erfassen")
		} else if _, ok := w["3"]; !ok {
			hinweise = append(hinweise, "Bruttoarbeitslohn (Nr. 3) nicht erkannt")
		}
	case ArtSteuerbescheid:
		res.Vorjahreswerte = bescheidWerte(zeilen, jahr)
		for _, v := range res.Vorjahreswerte {
			if v.Schluessel == "bescheid.est" {
				b.BetragCent = v.BetragCent
			}
		}
		if len(res.Vorjahreswerte) == 0 {
			hinweise = append(hinweise, "Keine Werte des Steuerbescheids erkannt")
		}
	case ArtMietvertrag:
		// kein Vorschlag: Beträge je Monat, nicht je Jahr
	default:
		cent, h := erkenneGesamtbetrag(zeilen, art)
		b.BetragCent = cent
		if h != "" && strings.TrimSpace(text) != "" {
			hinweise = append(hinweise, h)
		}
		kat, notiz := vorschlagKategorie(art, text)
		if kat != "" && cent > 0 {
			bez := artTitel(art)
			if b.Aussteller != "" {
				bez += " " + b.Aussteller
			}
			if art == ArtHandwerker {
				if ak, ok := arbeitskosten(zeilen); ok {
					cent, notiz = ak, "Nur Arbeitskosten (ohne Material) ansetzbar"
				} else {
					notiz = "Gesamtbetrag - nur Arbeitskosten (ohne Material) ansetzbar, Betrag bitte kürzen"
					hinweise = append(hinweise, "Arbeitskosten nicht separat erkannt")
				}
			}
			res.Positionen = append(res.Positionen, pos(kat, bez, cent, notiz))
		}
	}
	b.Hinweis = strings.Join(hinweise, "; ")
	res.Beleg = b
	return res
}

func arbeitskosten(zeilen []string) (int64, bool) {
	for i, z := range zeilen {
		low := strings.ToLower(z)
		if betragAusschluss.MatchString(low) || !enthaeltEines(low, betragTier0[ArtHandwerker]) {
			continue
		}
		if b := betraegeZeileOderNaechste(zeilen, i, false); len(b) > 0 && b[len(b)-1].Sicher {
			return b[len(b)-1].Cent, true
		}
	}
	return 0, false
}

func artTitel(art string) string {
	switch art {
	case ArtHandwerker:
		return "Handwerkerrechnung"
	case ArtSpende:
		return "Spendenbescheinigung"
	case ArtKinderbetreuung:
		return "Kinderbetreuung"
	case ArtHonorarAusgang:
		return "Honorarrechnung"
	case ArtNebenkosten:
		return "Nebenkostenabrechnung"
	case ArtVersicherung:
		return "Versicherung"
	case ArtRechnung:
		return "Rechnung"
	}
	return "Beleg"
}
