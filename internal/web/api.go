package web

import (
	"errors"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"steuer/internal/erfassung"
	"steuer/internal/modell"
	"steuer/internal/rechner"
	"steuer/internal/store"
)

func (s *Server) routen(m *http.ServeMux) {
	m.HandleFunc("GET /api/meta", s.meta)
	m.HandleFunc("GET /api/uebersicht", s.uebersicht)
	m.HandleFunc("POST /api/eingang/pruefen", s.eingangPruefen)

	m.HandleFunc("GET /api/belege", s.belegeListe)
	m.HandleFunc("GET /api/belege/{id}", s.belegDetail)
	m.HandleFunc("PUT /api/belege/{id}", s.belegAendern)
	m.HandleFunc("DELETE /api/belege/{id}", s.belegLoeschen)
	m.HandleFunc("GET /api/belege/{id}/datei", s.belegDatei)

	m.HandleFunc("GET /api/positionen", s.positionenListe)
	m.HandleFunc("POST /api/positionen", s.positionAnlegen)
	m.HandleFunc("PUT /api/positionen/{id}", s.positionAendern)
	m.HandleFunc("DELETE /api/positionen/{id}", s.positionLoeschen)

	m.HandleFunc("GET /api/angaben", s.angabenListe)
	m.HandleFunc("PUT /api/angaben", s.angabeSetzen)

	m.HandleFunc("GET /api/erklaerung", s.erklaerung)
	m.HandleFunc("GET /api/erklaerung.csv", s.erklaerungCSV)

	m.HandleFunc("GET /api/vorjahre", s.vorjahre)
	m.HandleFunc("POST /api/vorjahre", s.vorjahreSpeichern)
	m.HandleFunc("DELETE /api/vorjahre", s.vorjahrLoeschen)

	m.HandleFunc("GET /api/checkliste", s.checkliste)
}

// ---- Stammdaten der Oberfläche ----

type zuordnungDTO struct {
	Formular   string `json:"formular"`
	Zeile      string `json:"zeile"`
	Kennziffer string `json:"kennziffer"`
	Hinweis    string `json:"hinweis"`
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	zu := map[string]zuordnungDTO{}
	for _, z := range rechner.Zuordnungen() {
		zu[z.Kategorie] = zuordnungDTO{z.Formular, z.Zeile, z.Kennziffer, z.Hinweis}
	}
	antwort(w, 200, map[string]any{
		"standardjahr":  s.StandardJahr,
		"kategorien":    modell.Kategorien,
		"zuordnungen":   zu,
		"belegarten":    belegarten,
		"vorjahrsarten": vorjahrSchluessel(),
	})
}

var belegarten = []string{
	erfassung.ArtLohnsteuerbescheinigung, erfassung.ArtSteuerbescheid, erfassung.ArtRechnung,
	erfassung.ArtHonorarAusgang, erfassung.ArtSpende, erfassung.ArtVersicherung,
	erfassung.ArtNebenkosten, erfassung.ArtMietvertrag, erfassung.ArtKinderbetreuung,
	erfassung.ArtHandwerker, erfassung.ArtSonstiges,
}

// ---- Berechnung ----

type berechnung struct {
	Ergebnis   rechner.Ergebnis
	Positionen []modell.Position
	Angaben    map[string]string
	Fehler     string // Text, wenn nicht berechenbar
}

func (s *Server) berechne(jahr int) (berechnung, error) {
	pos, err := s.Store.Positionen(jahr)
	if err != nil {
		return berechnung{}, err
	}
	ang, err := s.Store.Angaben(jahr)
	if err != nil {
		return berechnung{}, err
	}
	b := berechnung{Positionen: pos, Angaben: ang}
	if len(rechner.AngabeDefs(jahr)) == 0 {
		b.Fehler = "Für das Steuerjahr " + itoa(jahr) + " sind keine Steuerwerte hinterlegt. Bitte wähle ein unterstütztes Jahr (aktuell 2026)."
		return b, nil
	}
	erg, err := rechner.Berechne(jahr, ang, pos)
	if err != nil {
		b.Fehler = err.Error()
		return b, nil
	}
	b.Ergebnis = erg
	return b, nil
}

func itoa(i int) string { return strconv.Itoa(i) }

func stufeRang(s string) int {
	switch s {
	case "warnung":
		return 0
	case "pruefen":
		return 1
	}
	return 2
}

func sortierteHinweise(h []rechner.Hinweis) []hinweisDTO {
	out := make([]hinweisDTO, 0, len(h))
	for _, x := range h {
		out = append(out, hinweisDTO{x.Stufe, x.Thema, x.Text})
	}
	sort.SliceStable(out, func(i, j int) bool { return stufeRang(out[i].Stufe) < stufeRang(out[j].Stufe) })
	return out
}

type hinweisDTO struct {
	Stufe string `json:"stufe"`
	Thema string `json:"thema"`
	Text  string `json:"text"`
}

// ---- Übersicht ----

func (s *Server) uebersicht(w http.ResponseWriter, r *http.Request) {
	jahr, err := s.jahr(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	b, err := s.berechne(jahr)
	if err != nil {
		serverfehler(w, err)
		return
	}
	alle, err := s.Store.BelegeAlle()
	if err != nil {
		serverfehler(w, err)
		return
	}
	neu, andere := 0, 0
	for _, x := range alle {
		if x.Status != modell.StatusNeu {
			continue
		}
		if x.Steuerjahr == jahr {
			neu++
		} else {
			andere++
		}
	}
	vorschlaege := 0
	for _, p := range b.Positionen {
		if !p.Bestaetigt {
			vorschlaege++
		}
	}
	offen, warn := 0, 0
	for _, h := range b.Ergebnis.Hinweise {
		if h.Stufe == "warnung" || h.Stufe == "pruefen" {
			offen++
		}
		if h.Stufe == "warnung" {
			warn++
		}
	}
	antwort(w, 200, map[string]any{
		"jahr":                jahr,
		"berechnet":           b.Fehler == "",
		"fehler":              b.Fehler,
		"zve_cent":            b.Ergebnis.ZvECent,
		"est_cent":            b.Ergebnis.EStCent,
		"soli_cent":           b.Ergebnis.SoliCent,
		"vorausgezahlt_cent":  b.Ergebnis.VorausgezahltCent,
		"erstattung_cent":     b.Ergebnis.ErstattungCent,
		"belege_neu":          neu,
		"belege_neu_andere":   andere,
		"vorschlaege_offen":   vorschlaege,
		"hinweise_offen":      offen,
		"hinweise_warnungen":  warn,
		"eingang":             filepath.Join(s.Datenordner, "Eingang"),
		"texterkennung":       erfassung.TexterkennungVerfuegbar(),
		"unterstuetzte_jahre": []int{2026},
	})
}

func (s *Server) eingangPruefen(w http.ResponseWriter, r *http.Request) {
	if s.Dienst == nil {
		fehler(w, 503, "Der Erfassungsdienst läuft nicht.")
		return
	}
	neu, errs := s.Dienst.Durchlauf()
	texte := []string{}
	for _, e := range errs {
		texte = append(texte, e.Error())
	}
	antwort(w, 200, map[string]any{"neu": neu, "fehler": texte, "zeit": time.Now().Format(time.RFC3339)})
}

// ---- Belege ----

type belegDTO struct {
	modell.Beleg
	OffeneVorschlaege int `json:"offene_vorschlaege"`
	AnzahlPositionen  int `json:"anzahl_positionen"`
}

func (s *Server) belegeListe(w http.ResponseWriter, r *http.Request) {
	alle, err := s.Store.BelegeAlle()
	if err != nil {
		serverfehler(w, err)
		return
	}
	jahrText := r.URL.Query().Get("jahr")
	status := r.URL.Query().Get("status")
	jahre := map[int]bool{}
	for _, b := range alle {
		jahre[b.Steuerjahr] = true
	}
	offen := map[int64]int{}
	anzahl := map[int64]int{}
	for j := range jahre {
		ps, err := s.Store.Positionen(j)
		if err != nil {
			serverfehler(w, err)
			return
		}
		for _, p := range ps {
			if p.BelegID == nil {
				continue
			}
			anzahl[*p.BelegID]++
			if !p.Bestaetigt {
				offen[*p.BelegID]++
			}
		}
	}
	out := []belegDTO{}
	for _, b := range alle {
		if jahrText != "" && jahrText != "alle" && itoa(b.Steuerjahr) != jahrText {
			continue
		}
		if status != "" && status != "alle" && string(b.Status) != status {
			continue
		}
		b.Text = "" // Volltext nur in der Detailansicht
		out = append(out, belegDTO{b, offen[b.ID], anzahl[b.ID]})
	}
	antwort(w, 200, map[string]any{"belege": out})
}

func (s *Server) belegDetail(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	b, err := s.Store.Beleg(id)
	if err != nil {
		fehler(w, 404, "Beleg nicht gefunden.")
		return
	}
	ps, err := s.Store.PositionenZuBeleg(id)
	if err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"beleg": b, "positionen": ps})
}

type belegEingabe struct {
	Steuerjahr int    `json:"steuerjahr"`
	Belegdatum string `json:"belegdatum"`
	Aussteller string `json:"aussteller"`
	Art        string `json:"art"`
	BetragCent int64  `json:"betrag_cent"`
	Status     string `json:"status"`
}

func (s *Server) belegAendern(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	var in belegEingabe
	if !lies(w, r, &in) {
		return
	}
	b, err := s.Store.Beleg(id)
	if err != nil {
		fehler(w, 404, "Beleg nicht gefunden.")
		return
	}
	if in.Steuerjahr < 2000 || in.Steuerjahr > 2100 {
		fehler(w, 400, "Steuerjahr ungültig.")
		return
	}
	switch modell.Belegstatus(in.Status) {
	case modell.StatusNeu, modell.StatusBestaetigt, modell.StatusVerworfen:
	default:
		fehler(w, 400, "Status ungültig.")
		return
	}
	datum, ok := normDatum(in.Belegdatum)
	if !ok {
		fehler(w, 400, "Belegdatum ungültig (erwartet TT.MM.JJJJ).")
		return
	}
	if len([]rune(in.Aussteller)) > 200 || len([]rune(in.Art)) > 60 {
		fehler(w, 400, "Eingabe zu lang.")
		return
	}
	b.Steuerjahr, b.Belegdatum, b.Aussteller = in.Steuerjahr, datum, strings.TrimSpace(in.Aussteller)
	b.Art, b.BetragCent, b.Status = strings.TrimSpace(in.Art), in.BetragCent, modell.Belegstatus(in.Status)
	if err := s.Store.BelegAendern(b); err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"beleg": b})
}

func (s *Server) belegLoeschen(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	if err := s.Store.BelegLoeschen(id); err != nil {
		if errors.Is(err, store.ErrNichtGefunden) {
			fehler(w, 404, "Beleg nicht gefunden.")
			return
		}
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"ok": true})
}
