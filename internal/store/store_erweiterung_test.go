package store

import (
	"errors"
	"testing"

	"steuer/internal/modell"
)

func TestBelegLoeschen(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id, _ := s.BelegAnlegen(modell.Beleg{Steuerjahr: 2026, Datei: "a.pdf", SHA256: "a"})
	id2, _ := s.BelegAnlegen(modell.Beleg{Steuerjahr: 2025, Datei: "b.pdf", SHA256: "b"})
	s.PositionAnlegen(modell.Position{Steuerjahr: 2026, BelegID: &id, Kategorie: "n.arbeitsmittel", BetragCent: 100, Vorschlag: true})
	pb, _ := s.PositionAnlegen(modell.Position{Steuerjahr: 2026, BelegID: &id, Kategorie: "n.fortbildung", BetragCent: 200, Bestaetigt: true})
	if err := s.BelegLoeschen(id); err != nil {
		t.Fatal(err)
	}
	ps, _ := s.Positionen(2026)
	if len(ps) != 1 || ps[0].ID != pb || ps[0].BelegID != nil {
		t.Fatalf("falsche Restpositionen: %+v", ps)
	}
	if err := s.BelegLoeschen(id); !errors.Is(err, ErrNichtGefunden) {
		t.Fatalf("zweites Löschen: %v", err)
	}
	alle, _ := s.BelegeAlle()
	if len(alle) != 1 || alle[0].ID != id2 {
		t.Fatalf("BelegeAlle: %+v", alle)
	}
	if _, err := s.Position(9999); !errors.Is(err, ErrNichtGefunden) {
		t.Fatalf("Position: %v", err)
	}
	if p, err := s.Position(pb); err != nil || p.BetragCent != 200 {
		t.Fatalf("Position: %+v %v", p, err)
	}
}

func TestVorjahreswerteGesammelt(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	err := s.VorjahreswerteSetzen([]modell.Vorjahreswert{
		{Steuerjahr: 2025, Schluessel: "bescheid.zve", BetragCent: 5000000},
		{Steuerjahr: 2025, Schluessel: "bescheid.est", BetragCent: 900000},
		{Steuerjahr: 2025, Schluessel: "bescheid.est", BetragCent: 910000},
	})
	if err != nil {
		t.Fatal(err)
	}
	m, _ := s.Vorjahreswerte(2025)
	if len(m) != 2 || m["bescheid.est"] != 910000 {
		t.Fatalf("Werte: %v", m)
	}
	s.VorjahreswertLoeschen(2025, "bescheid.est")
	m, _ = s.Vorjahreswerte(2025)
	if len(m) != 1 {
		t.Fatalf("nach Löschen: %v", m)
	}
}

func TestPositionenZuBeleg(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	id, _ := s.BelegAnlegen(modell.Beleg{Steuerjahr: 2026, Datei: "a.pdf", SHA256: "a"})
	s.PositionAnlegen(modell.Position{Steuerjahr: 2026, BelegID: &id, Kategorie: "n.arbeitsmittel", BetragCent: 1})
	s.PositionAnlegen(modell.Position{Steuerjahr: 2026, Kategorie: "n.arbeitsmittel", BetragCent: 2})
	ps, err := s.PositionenZuBeleg(id)
	if err != nil || len(ps) != 1 || ps[0].BetragCent != 1 {
		t.Fatalf("%+v %v", ps, err)
	}
}
