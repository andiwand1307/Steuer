package web

import (
	"errors"
	"net/http"
	"strings"

	"steuer/internal/modell"
	"steuer/internal/store"
)

func (s *Server) positionenListe(w http.ResponseWriter, r *http.Request) {
	jahr, err := s.jahr(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	ps, err := s.Store.Positionen(jahr)
	if err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"jahr": jahr, "positionen": ps})
}

type positionEingabe struct {
	Steuerjahr  int    `json:"steuerjahr"`
	BelegID     *int64 `json:"beleg_id"`
	Person      string `json:"person"`
	Kategorie   string `json:"kategorie"`
	Bezeichnung string `json:"bezeichnung"`
	BetragCent  int64  `json:"betrag_cent"`
	Datum       string `json:"datum"`
	Notiz       string `json:"notiz"`
	Bestaetigt  bool   `json:"bestaetigt"`
}

// pruefe validiert die Eingabe und füllt p.
func (in positionEingabe) uebernehmeIn(p *modell.Position) error {
	if in.Steuerjahr < 2000 || in.Steuerjahr > 2100 {
		return errors.New("Steuerjahr ungültig.")
	}
	if !modell.KategorieBekannt(in.Kategorie) {
		return errors.New("Unbekannte Kategorie.")
	}
	if in.Person != "" && in.Person != "A" && in.Person != "B" {
		return errors.New("Person muss A, B oder leer (gemeinsam) sein.")
	}
	if in.BetragCent < -1_000_000_000_000 || in.BetragCent > 1_000_000_000_000 {
		return errors.New("Betrag außerhalb des zulässigen Bereichs.")
	}
	d, ok := normDatum(in.Datum)
	if !ok {
		return errors.New("Datum ungültig (erwartet TT.MM.JJJJ).")
	}
	if len([]rune(in.Bezeichnung)) > 300 || len([]rune(in.Notiz)) > 2000 {
		return errors.New("Eingabe zu lang.")
	}
	p.Steuerjahr, p.Person, p.Kategorie = in.Steuerjahr, in.Person, in.Kategorie
	p.Bezeichnung, p.BetragCent, p.Datum = strings.TrimSpace(in.Bezeichnung), in.BetragCent, d
	p.Notiz, p.Bestaetigt = strings.TrimSpace(in.Notiz), in.Bestaetigt
	p.Vorschlag = !in.Bestaetigt && p.Vorschlag
	return nil
}

func (s *Server) positionAnlegen(w http.ResponseWriter, r *http.Request) {
	var in positionEingabe
	if !lies(w, r, &in) {
		return
	}
	var p modell.Position
	if err := in.uebernehmeIn(&p); err != nil {
		fehler(w, 400, err.Error())
		return
	}
	if in.BelegID != nil {
		if _, err := s.Store.Beleg(*in.BelegID); err != nil {
			fehler(w, 400, "Der Beleg existiert nicht.")
			return
		}
		p.BelegID = in.BelegID
	}
	id, err := s.Store.PositionAnlegen(p)
	if err != nil {
		serverfehler(w, err)
		return
	}
	p.ID = id
	antwort(w, 201, map[string]any{"position": p})
}

func (s *Server) positionAendern(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	var in positionEingabe
	if !lies(w, r, &in) {
		return
	}
	p, err := s.Store.Position(id)
	if err != nil {
		fehler(w, 404, "Position nicht gefunden.")
		return
	}
	if err := in.uebernehmeIn(&p); err != nil {
		fehler(w, 400, err.Error())
		return
	}
	if err := s.Store.PositionAendern(p); err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"position": p})
}

func (s *Server) positionLoeschen(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	if _, err := s.Store.Position(id); errors.Is(err, store.ErrNichtGefunden) {
		fehler(w, 404, "Position nicht gefunden.")
		return
	}
	if err := s.Store.PositionLoeschen(id); err != nil {
		serverfehler(w, err)
		return
	}
	antwort(w, 200, map[string]any{"ok": true})
}
