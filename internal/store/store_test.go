package store

import (
	"errors"
	"testing"

	"steuer/internal/modell"
)

func TestGrundfunktionen(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, err := s.BelegAnlegen(modell.Beleg{Steuerjahr: 2026, Datei: "a.pdf", SHA256: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BelegAnlegen(modell.Beleg{Steuerjahr: 2026, Datei: "b.pdf", SHA256: "x"}); !errors.Is(err, ErrDoppelt) {
		t.Fatalf("Doppelter Beleg nicht erkannt: %v", err)
	}
	pid, err := s.PositionAnlegen(modell.Position{Steuerjahr: 2026, BelegID: &id, Kategorie: "n.arbeitsmittel", BetragCent: 12345})
	if err != nil {
		t.Fatal(err)
	}
	ps, _ := s.Positionen(2026)
	if len(ps) != 1 || ps[0].ID != pid || ps[0].BetragCent != 12345 || *ps[0].BelegID != id {
		t.Fatalf("Position falsch: %+v", ps)
	}
	s.AngabeSetzen(modell.Angabe{Steuerjahr: 2026, Schluessel: "k", Wert: "1"})
	s.AngabeSetzen(modell.Angabe{Steuerjahr: 2026, Schluessel: "k", Wert: "2"})
	if a, _ := s.Angaben(2026); a["k"] != "2" {
		t.Fatalf("Angabe falsch: %v", a)
	}
	b, _ := s.Beleg(id)
	b.Status = modell.StatusBestaetigt
	if err := s.BelegAendern(b); err != nil {
		t.Fatal(err)
	}
	if b2, _ := s.Beleg(id); b2.Status != modell.StatusBestaetigt {
		t.Fatal("Status nicht gespeichert")
	}
}
