package web

import (
	"net/http"
	"slices"
	"strings"

	"steuer/internal/modell"
	"steuer/internal/rechner"
)

type angabeDefDTO struct {
	Schluessel string   `json:"schluessel"`
	Titel      string   `json:"titel"`
	Gruppe     string   `json:"gruppe"`
	Typ        string   `json:"typ"`
	Hilfe      string   `json:"hilfe"`
	Optionen   []string `json:"optionen"`
}

func (s *Server) angabenListe(w http.ResponseWriter, r *http.Request) {
	jahr, err := s.jahr(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	werte, err := s.Store.Angaben(jahr)
	if err != nil {
		serverfehler(w, err)
		return
	}
	defs := []angabeDefDTO{}
	for _, d := range rechner.AngabeDefs(jahr) {
		o := d.Optionen
		if o == nil {
			o = []string{}
		}
		defs = append(defs, angabeDefDTO{d.Schluessel, d.Titel, d.Gruppe, d.Typ, d.Hilfe, o})
	}
	antwort(w, 200, map[string]any{"jahr": jahr, "defs": defs, "werte": werte})
}

type angabeEingabe struct {
	Jahr       int    `json:"jahr"`
	Schluessel string `json:"schluessel"`
	Wert       string `json:"wert"`
}

func (s *Server) angabeSetzen(w http.ResponseWriter, r *http.Request) {
	var in angabeEingabe
	if !lies(w, r, &in) {
		return
	}
	var def *rechner.AngabeDef
	for _, d := range rechner.AngabeDefs(in.Jahr) {
		if d.Schluessel == in.Schluessel {
			d := d
			def = &d
			break
		}
	}
	if def == nil {
		fehler(w, 400, "Diese Angabe gibt es für das Steuerjahr nicht.")
		return
	}
	wert := strings.TrimSpace(in.Wert)
	if wert != "" {
		switch def.Typ {
		case "zahl", "betrag":
			n, ok := normZahl(wert)
			if !ok {
				fehler(w, 400, "Bitte eine Zahl eingeben (z. B. 1.234,56).")
				return
			}
			wert = n
		case "datum":
			d, ok := normDatum(wert)
			if !ok {
				fehler(w, 400, "Bitte ein Datum eingeben (TT.MM.JJJJ).")
				return
			}
			wert = d
		case "jaNein":
			switch strings.ToLower(wert) {
			case "ja", "nein":
				wert = strings.ToLower(wert)
			default:
				fehler(w, 400, "Bitte ja oder nein wählen.")
				return
			}
		case "auswahl":
			if !slices.Contains(def.Optionen, wert) {
				fehler(w, 400, "Bitte einen der angebotenen Werte wählen.")
				return
			}
		default:
			if len([]rune(wert)) > 500 {
				fehler(w, 400, "Eingabe zu lang.")
				return
			}
		}
	}
	if err := s.Store.AngabeSetzen(modell.Angabe{Steuerjahr: in.Jahr, Schluessel: in.Schluessel, Wert: wert}); err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"schluessel": in.Schluessel, "wert": wert})
}
