// Package store kapselt die lokale SQLite-Datenbank.
package store

import (
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"

	"steuer/internal/modell"
)

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS beleg (
  id INTEGER PRIMARY KEY, steuerjahr INTEGER NOT NULL, datei TEXT NOT NULL,
  sha256 TEXT NOT NULL UNIQUE, erfasst_am TEXT NOT NULL, belegdatum TEXT NOT NULL DEFAULT '',
  aussteller TEXT NOT NULL DEFAULT '', art TEXT NOT NULL DEFAULT '', betrag_cent INTEGER NOT NULL DEFAULT 0,
  text TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'neu', hinweis TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS position (
  id INTEGER PRIMARY KEY, steuerjahr INTEGER NOT NULL, beleg_id INTEGER REFERENCES beleg(id),
  person TEXT NOT NULL DEFAULT '', kategorie TEXT NOT NULL, bezeichnung TEXT NOT NULL DEFAULT '', betrag_cent INTEGER NOT NULL,
  datum TEXT NOT NULL DEFAULT '', notiz TEXT NOT NULL DEFAULT '',
  bestaetigt INTEGER NOT NULL DEFAULT 0, vorschlag INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS angabe (
  steuerjahr INTEGER NOT NULL, schluessel TEXT NOT NULL, wert TEXT NOT NULL,
  PRIMARY KEY (steuerjahr, schluessel));
CREATE TABLE IF NOT EXISTS vorjahreswert (
  steuerjahr INTEGER NOT NULL, schluessel TEXT NOT NULL, betrag_cent INTEGER NOT NULL,
  PRIMARY KEY (steuerjahr, schluessel));
`

// Open öffnet (oder erzeugt) die Datenbank. ":memory:" für Tests.
func Open(pfad string) (*Store, error) {
	db, err := sql.Open("sqlite", pfad)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON;" + schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ErrDoppelt meldet, dass ein Beleg mit gleichem Inhalt schon erfasst ist.
var ErrDoppelt = errors.New("beleg bereits erfasst")

const belegSpalten = `id, steuerjahr, datei, sha256, erfasst_am, belegdatum, aussteller, art, betrag_cent, text, status, hinweis`

func scanBeleg(r interface{ Scan(...any) error }) (modell.Beleg, error) {
	var b modell.Beleg
	var t string
	err := r.Scan(&b.ID, &b.Steuerjahr, &b.Datei, &b.SHA256, &t, &b.Belegdatum, &b.Aussteller, &b.Art, &b.BetragCent, &b.Text, &b.Status, &b.Hinweis)
	b.ErfasstAm, _ = time.Parse(time.RFC3339, t)
	return b, err
}

// BelegAnlegen speichert einen neuen Beleg und gibt seine ID zurück.
func (s *Store) BelegAnlegen(b modell.Beleg) (int64, error) {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM beleg WHERE sha256 = ?`, b.SHA256).Scan(&n); err != nil {
		return 0, err
	}
	if n > 0 {
		return 0, ErrDoppelt
	}
	if b.Status == "" {
		b.Status = modell.StatusNeu
	}
	if b.ErfasstAm.IsZero() {
		b.ErfasstAm = time.Now()
	}
	res, err := s.db.Exec(`INSERT INTO beleg (steuerjahr, datei, sha256, erfasst_am, belegdatum, aussteller, art, betrag_cent, text, status, hinweis)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, b.Steuerjahr, b.Datei, b.SHA256, b.ErfasstAm.Format(time.RFC3339), b.Belegdatum, b.Aussteller, b.Art, b.BetragCent, b.Text, b.Status, b.Hinweis)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) Beleg(id int64) (modell.Beleg, error) {
	return scanBeleg(s.db.QueryRow(`SELECT `+belegSpalten+` FROM beleg WHERE id = ?`, id))
}

func (s *Store) Belege(jahr int) ([]modell.Beleg, error) {
	rows, err := s.db.Query(`SELECT `+belegSpalten+` FROM beleg WHERE steuerjahr = ? ORDER BY id`, jahr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []modell.Beleg{}
	for rows.Next() {
		b, err := scanBeleg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BelegAendern überschreibt alle veränderbaren Felder (nicht Datei-Hash und Erfassungszeit).
func (s *Store) BelegAendern(b modell.Beleg) error {
	_, err := s.db.Exec(`UPDATE beleg SET steuerjahr=?, datei=?, belegdatum=?, aussteller=?, art=?, betrag_cent=?, status=?, hinweis=? WHERE id=?`,
		b.Steuerjahr, b.Datei, b.Belegdatum, b.Aussteller, b.Art, b.BetragCent, b.Status, b.Hinweis, b.ID)
	return err
}

const posSpalten = `id, steuerjahr, beleg_id, person, kategorie, bezeichnung, betrag_cent, datum, notiz, bestaetigt, vorschlag`

func (s *Store) PositionAnlegen(p modell.Position) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO position (steuerjahr, beleg_id, person, kategorie, bezeichnung, betrag_cent, datum, notiz, bestaetigt, vorschlag)
		VALUES (?,?,?,?,?,?,?,?,?,?)`, p.Steuerjahr, p.BelegID, p.Person, p.Kategorie, p.Bezeichnung, p.BetragCent, p.Datum, p.Notiz, p.Bestaetigt, p.Vorschlag)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) PositionAendern(p modell.Position) error {
	_, err := s.db.Exec(`UPDATE position SET steuerjahr=?, beleg_id=?, person=?, kategorie=?, bezeichnung=?, betrag_cent=?, datum=?, notiz=?, bestaetigt=?, vorschlag=? WHERE id=?`,
		p.Steuerjahr, p.BelegID, p.Person, p.Kategorie, p.Bezeichnung, p.BetragCent, p.Datum, p.Notiz, p.Bestaetigt, p.Vorschlag, p.ID)
	return err
}

func (s *Store) PositionLoeschen(id int64) error {
	_, err := s.db.Exec(`DELETE FROM position WHERE id = ?`, id)
	return err
}

func (s *Store) Positionen(jahr int) ([]modell.Position, error) {
	rows, err := s.db.Query(`SELECT `+posSpalten+` FROM position WHERE steuerjahr = ? ORDER BY kategorie, datum, id`, jahr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []modell.Position{}
	for rows.Next() {
		var p modell.Position
		if err := rows.Scan(&p.ID, &p.Steuerjahr, &p.BelegID, &p.Person, &p.Kategorie, &p.Bezeichnung, &p.BetragCent, &p.Datum, &p.Notiz, &p.Bestaetigt, &p.Vorschlag); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) AngabeSetzen(a modell.Angabe) error {
	_, err := s.db.Exec(`INSERT INTO angabe (steuerjahr, schluessel, wert) VALUES (?,?,?)
		ON CONFLICT(steuerjahr, schluessel) DO UPDATE SET wert = excluded.wert`, a.Steuerjahr, a.Schluessel, a.Wert)
	return err
}

// Angaben liefert alle Stammdaten eines Jahres als Schlüssel-Wert-Tabelle.
func (s *Store) Angaben(jahr int) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT schluessel, wert FROM angabe WHERE steuerjahr = ?`, jahr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *Store) VorjahreswertSetzen(v modell.Vorjahreswert) error {
	_, err := s.db.Exec(`INSERT INTO vorjahreswert (steuerjahr, schluessel, betrag_cent) VALUES (?,?,?)
		ON CONFLICT(steuerjahr, schluessel) DO UPDATE SET betrag_cent = excluded.betrag_cent`, v.Steuerjahr, v.Schluessel, v.BetragCent)
	return err
}

func (s *Store) Vorjahreswerte(jahr int) (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT schluessel, betrag_cent FROM vorjahreswert WHERE steuerjahr = ?`, jahr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
