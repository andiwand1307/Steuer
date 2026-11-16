package web

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"steuer/internal/rechner"
)

type zeileDTO struct {
	Formular        string `json:"formular"`
	Zeile           string `json:"zeile"`
	Kennziffer      string `json:"kennziffer"`
	Person          string `json:"person"`
	Bezeichnung     string `json:"bezeichnung"`
	BetragCent      int64  `json:"betrag_cent"`
	Wert            string `json:"wert"`
	Herleitung      string `json:"herleitung"`
	Rechtsgrundlage string `json:"rechtsgrundlage"`
	Sicherheit      string `json:"sicherheit"`
}

type formularDTO struct {
	Name   string     `json:"name"`
	Zeilen []zeileDTO `json:"zeilen"`
}

type varianteDTO struct {
	Titel         string `json:"titel"`
	SteuerCent    int64  `json:"steuer_cent"`
	DifferenzCent int64  `json:"differenz_cent"`
	Beschreibung  string `json:"beschreibung"`
}

func zeilenDTO(e rechner.Ergebnis) []zeileDTO {
	out := make([]zeileDTO, 0, len(e.Zeilen))
	for _, z := range e.Zeilen {
		out = append(out, zeileDTO{z.Formular, z.Zeile, z.Kennziffer, z.Person, z.Bezeichnung, z.BetragCent, z.Wert, z.Herleitung, z.Rechtsgrundlage, z.Sicherheit})
	}
	return out
}

func (s *Server) erklaerung(w http.ResponseWriter, r *http.Request) {
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
	if b.Fehler != "" {
		antwort(w, 200, map[string]any{"jahr": jahr, "fehler": b.Fehler})
		return
	}
	e := b.Ergebnis
	formulare := []formularDTO{}
	idx := map[string]int{}
	for _, z := range zeilenDTO(e) {
		i, ok := idx[z.Formular]
		if !ok {
			i = len(formulare)
			idx[z.Formular] = i
			formulare = append(formulare, formularDTO{Name: z.Formular})
		}
		formulare[i].Zeilen = append(formulare[i].Zeilen, z)
	}
	basis := e.EStCent + e.SoliCent
	varianten := []varianteDTO{}
	for _, v := range e.Varianten {
		varianten = append(varianten, varianteDTO{v.Titel, v.SteuerCent, v.SteuerCent - basis, v.Beschreibung})
	}
	antwort(w, 200, map[string]any{
		"jahr":               jahr,
		"zve_cent":           e.ZvECent,
		"est_cent":           e.EStCent,
		"soli_cent":          e.SoliCent,
		"vorausgezahlt_cent": e.VorausgezahltCent,
		"erstattung_cent":    e.ErstattungCent,
		"kennzahlen":         e.Kennzahlen,
		"formulare":          formulare,
		"varianten":          varianten,
		"hinweise":           sortierteHinweise(e.Hinweise),
	})
}

func deutschCent(c int64) string {
	neg := c < 0
	if neg {
		c = -c
	}
	s := fmt.Sprintf("%d,%02d", c/100, c%100)
	if neg {
		s = "-" + s
	}
	return s
}

func (s *Server) erklaerungCSV(w http.ResponseWriter, r *http.Request) {
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
	if b.Fehler != "" {
		fehler(w, 422, b.Fehler)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="erklaerung-%d.csv"`, jahr))
	w.Write([]byte("\xef\xbb\xbf")) // BOM, damit Excel die Umlaute richtig liest
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cw.Write([]string{"Formular", "Zeile", "Kennziffer", "Person", "Bezeichnung", "Betrag in EUR", "Wert", "Sicherheit", "Herleitung", "Rechtsgrundlage"})
	for _, z := range zeilenDTO(b.Ergebnis) {
		betrag := ""
		if z.BetragCent != 0 || z.Wert == "" {
			betrag = deutschCent(z.BetragCent)
		}
		cw.Write([]string{csvSicher(z.Formular), z.Zeile, z.Kennziffer, z.Person, csvSicher(z.Bezeichnung), betrag, csvSicher(z.Wert), z.Sicherheit, csvSicher(z.Herleitung), csvSicher(z.Rechtsgrundlage)})
	}
	cw.Flush()
}

// csvSicher entschärft Zellen, die eine Tabellenkalkulation als Formel läse.
func csvSicher(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
