package web

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"steuer/internal/erfassung"
	"steuer/internal/modell"
)

var bescheidTitel = map[string]string{
	"bescheid.zve":              "Zu versteuerndes Einkommen",
	"bescheid.est":              "Festgesetzte Einkommensteuer",
	"bescheid.soli":             "Festgesetzter Solidaritätszuschlag",
	"bescheid.werbungskosten":   "Werbungskosten",
	"bescheid.eink_nichtselbst": "Einkünfte aus nichtselbständiger Arbeit",
	"bescheid.eink_selbst":      "Einkünfte aus selbständiger Arbeit / Gewerbe",
	"bescheid.gde":              "Gesamtbetrag der Einkünfte",
	"bescheid.erstattung":       "Erstattung (+) / Nachzahlung (-)",
}

var bescheidReihenfolge = []string{
	"bescheid.gde", "bescheid.eink_nichtselbst", "bescheid.eink_selbst", "bescheid.werbungskosten",
	"bescheid.zve", "bescheid.est", "bescheid.soli", "bescheid.erstattung",
}

type schluesselDTO struct {
	Schluessel string `json:"schluessel"`
	Titel      string `json:"titel"`
}

func vorjahrSchluessel() []schluesselDTO {
	out := []schluesselDTO{}
	for _, k := range bescheidReihenfolge {
		out = append(out, schluesselDTO{k, bescheidTitel[k]})
	}
	for _, k := range modell.Kategorien {
		out = append(out, schluesselDTO{k.Schluessel, k.Titel})
	}
	return out
}

func vorjahrTitel(k string) string {
	if t, ok := bescheidTitel[k]; ok {
		return t
	}
	for _, c := range modell.Kategorien {
		if c.Schluessel == k {
			return c.Titel
		}
	}
	return k
}

func vorjahrBekannt(k string) bool {
	if _, ok := bescheidTitel[k]; ok {
		return true
	}
	return modell.KategorieBekannt(k)
}

// aktuellerWert liefert den passenden Wert des laufenden Jahres für einen
// Vorjahresschlüssel.
func aktuellerWert(b berechnung, k string) (int64, bool) {
	if b.Fehler != "" {
		return 0, false
	}
	e := b.Ergebnis
	kz := e.Kennzahlen
	switch k {
	case "bescheid.zve":
		return e.ZvECent, true
	case "bescheid.est":
		return e.EStCent, true
	case "bescheid.soli":
		return e.SoliCent, true
	case "bescheid.erstattung":
		return e.ErstattungCent, true
	case "bescheid.gde":
		v, ok := kz["gesamtbetrag_einkuenfte"]
		return v, ok
	case "bescheid.eink_nichtselbst":
		return kz["einkuenfte_19_A"] + kz["einkuenfte_19_B"], true
	case "bescheid.eink_selbst":
		return kz["gewinn_nebentaetigkeit"], true
	case "bescheid.werbungskosten":
		return kz["werbungskosten_abziehbar_A"] + kz["werbungskosten_abziehbar_B"], true
	}
	if modell.KategorieBekannt(k) {
		var sum int64
		for _, p := range b.Positionen {
			if p.Bestaetigt && p.Kategorie == k {
				sum += p.BetragCent
			}
		}
		return sum, true
	}
	return 0, false
}

type vorjahrZeile struct {
	Schluessel    string   `json:"schluessel"`
	Titel         string   `json:"titel"`
	BetragCent    int64    `json:"betrag_cent"`
	HatAktuell    bool     `json:"hat_aktuell"`
	AktuellCent   int64    `json:"aktuell_cent"`
	AbweichungPro *float64 `json:"abweichung_prozent"`
	DifferenzCent int64    `json:"differenz_cent"`
	Markierung    string   `json:"markierung,omitempty"`
	Auffaellig    bool     `json:"auffaellig"`
}

type vorschlagWert struct {
	Steuerjahr int    `json:"steuerjahr"`
	Schluessel string `json:"schluessel"`
	Titel      string `json:"titel"`
	BetragCent int64  `json:"betrag_cent"`
	// GespeichertCent ist der schon vorhandene Wert (nil = noch nicht gespeichert).
	GespeichertCent *int64 `json:"gespeichert_cent"`
}

type vorschlagBeleg struct {
	BelegID    int64           `json:"beleg_id"`
	Datei      string          `json:"datei"`
	Aussteller string          `json:"aussteller"`
	Steuerjahr int             `json:"steuerjahr"`
	Werte      []vorschlagWert `json:"werte"`
	Fehler     string          `json:"fehler,omitempty"`
}

func (s *Server) vorjahre(w http.ResponseWriter, r *http.Request) {
	jahr, err := s.jahr(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	vj, err := jahrParam(r, "vorjahr", jahr-1)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	b, err := s.berechne(jahr)
	if err != nil {
		serverfehler(w, err)
		return
	}
	gespeichert, err := s.Store.Vorjahreswerte(vj)
	if err != nil {
		serverfehler(w, err)
		return
	}
	zeilen := []vorjahrZeile{}
	keys := make([]string, 0, len(gespeichert))
	for k := range gespeichert {
		keys = append(keys, k)
	}
	rang := func(k string) int {
		for i, x := range bescheidReihenfolge {
			if x == k {
				return i
			}
		}
		return 100
	}
	sort.Slice(keys, func(i, j int) bool {
		if rang(keys[i]) != rang(keys[j]) {
			return rang(keys[i]) < rang(keys[j])
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		z := vorjahrZeile{Schluessel: k, Titel: vorjahrTitel(k), BetragCent: gespeichert[k]}
		if a, ok := aktuellerWert(b, k); ok {
			z.HatAktuell, z.AktuellCent = true, a
			z.AbweichungPro, z.Auffaellig, z.Markierung = abweichung(z.BetragCent, a)
			z.DifferenzCent = a - z.BetragCent
		}
		zeilen = append(zeilen, z)
	}

	// Vorschläge aus erkannten Steuerbescheiden
	alle, err := s.Store.BelegeAlle()
	if err != nil {
		serverfehler(w, err)
		return
	}
	vorschlaege := []vorschlagBeleg{}
	for _, bl := range alle {
		if bl.Art != erfassung.ArtSteuerbescheid || bl.Status == modell.StatusVerworfen {
			continue
		}
		vb := vorschlagBeleg{BelegID: bl.ID, Datei: filepath.Base(filepath.FromSlash(bl.Datei)), Aussteller: bl.Aussteller, Steuerjahr: bl.Steuerjahr, Werte: []vorschlagWert{}}
		pfad, err := s.sichererPfad(bl.Datei)
		if err != nil {
			vb.Fehler = "Die Datei des Belegs ist nicht lesbar."
			vorschlaege = append(vorschlaege, vb)
			continue
		}
		erg, err := erfassung.Analysiere(pfad, bl.Steuerjahr)
		if err != nil {
			vb.Fehler = "Die Datei konnte nicht ausgewertet werden."
			vorschlaege = append(vorschlaege, vb)
			continue
		}
		bekannt, _ := s.Store.Vorjahreswerte(bl.Steuerjahr)
		for _, v := range erg.Vorjahreswerte {
			wv := vorschlagWert{Steuerjahr: bl.Steuerjahr, Schluessel: v.Schluessel, Titel: vorjahrTitel(v.Schluessel), BetragCent: v.BetragCent}
			if c, ok := bekannt[v.Schluessel]; ok {
				c := c
				wv.GespeichertCent = &c
			}
			vb.Werte = append(vb.Werte, wv)
		}
		vorschlaege = append(vorschlaege, vb)
	}
	antwort(w, 200, map[string]any{
		"jahr": jahr, "vorjahr": vj, "werte": zeilen, "vorschlaege": vorschlaege,
		"berechnungshinweis": b.Fehler,
	})
}

// abweichung liefert die prozentuale Abweichung des aktuellen vom früheren
// Wert; auffällig ab mehr als 25 %. Bei Vorjahreswert 0 oder Vorzeichenwechsel
// (z.B. Erstattung ↔ Nachzahlung) ist eine Prozentangabe sinnlos: dann nur
// Markierung, die absolute Differenz steht im Aufrufer.
func abweichung(frueher, aktuell int64) (*float64, bool, string) {
	switch {
	case frueher == 0 && aktuell == 0:
		return nil, false, ""
	case frueher == 0:
		return nil, true, "Vorjahr 0 €"
	case (frueher < 0 && aktuell > 0) || (frueher > 0 && aktuell < 0):
		return nil, true, "Vorzeichenwechsel"
	}
	d := float64(aktuell-frueher) / abs(float64(frueher)) * 100
	return &d, d > 25 || d < -25, ""
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

type vorjahrEingabe struct {
	Werte []modell.Vorjahreswert `json:"werte"`
}

func (s *Server) vorjahreSpeichern(w http.ResponseWriter, r *http.Request) {
	var in vorjahrEingabe
	if !lies(w, r, &in) {
		return
	}
	if len(in.Werte) == 0 || len(in.Werte) > 200 {
		fehler(w, 400, "Keine Werte übergeben.")
		return
	}
	for _, v := range in.Werte {
		if v.Steuerjahr < 2000 || v.Steuerjahr > 2100 || !vorjahrBekannt(v.Schluessel) {
			fehler(w, 400, "Ungültiger Vorjahreswert ("+strings.TrimSpace(v.Schluessel)+").")
			return
		}
		if v.BetragCent < -1_000_000_000_000 || v.BetragCent > 1_000_000_000_000 {
			fehler(w, 400, "Betrag außerhalb des zulässigen Bereichs.")
			return
		}
	}
	if err := s.Store.VorjahreswerteSetzen(in.Werte); err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"gespeichert": len(in.Werte)})
}

func (s *Server) vorjahrLoeschen(w http.ResponseWriter, r *http.Request) {
	j, err := jahrParam(r, "jahr", 0)
	if err != nil || j == 0 {
		fehler(w, 400, "Steuerjahr fehlt.")
		return
	}
	k := r.URL.Query().Get("schluessel")
	if !vorjahrBekannt(k) {
		fehler(w, 400, "Unbekannter Schlüssel.")
		return
	}
	if err := s.Store.VorjahreswertLoeschen(j, k); err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"ok": true})
}
