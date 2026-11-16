// Package modell enthält die gemeinsamen Datentypen. Alle Geldbeträge sind
// ganzzahlige Cent (int64), nie Gleitkommazahlen.
package modell

import "time"

// Belegstatus beschreibt, wie weit ein erfasstes Dokument geprüft ist.
type Belegstatus string

const (
	StatusNeu        Belegstatus = "neu"        // automatisch erfasst, vom Nutzer noch nicht geprüft
	StatusBestaetigt Belegstatus = "bestaetigt" // vom Nutzer geprüft und übernommen
	StatusVerworfen  Belegstatus = "verworfen"  // steuerlich nicht relevant
)

// Beleg ist ein Dokument aus dem Eingangsordner.
type Beleg struct {
	ID         int64       `json:"id"`
	Steuerjahr int         `json:"steuerjahr"`
	Datei      string      `json:"datei"`  // Pfad relativ zum Datenordner
	SHA256     string      `json:"sha256"` // verhindert doppelte Erfassung
	ErfasstAm  time.Time   `json:"erfasst_am"`
	Belegdatum string      `json:"belegdatum"` // JJJJ-MM-TT oder leer
	Aussteller string      `json:"aussteller"`
	Art        string      `json:"art"`         // z.B. "lohnsteuerbescheinigung", "rechnung", "steuerbescheid"
	BetragCent int64       `json:"betrag_cent"` // erkannter Gesamtbetrag, 0 wenn unbekannt
	Text       string      `json:"text"`        // ausgelesener Volltext
	Status     Belegstatus `json:"status"`
	Hinweis    string      `json:"hinweis"` // z.B. "Betrag unsicher erkannt"
}

// Position ist ein steuerlich relevanter Einzelbetrag. Nur bestätigte
// Positionen gehen in die Berechnung ein.
type Position struct {
	ID          int64  `json:"id"`
	Steuerjahr  int    `json:"steuerjahr"`
	BelegID     *int64 `json:"beleg_id"`  // nil = ohne Beleg erfasst
	Person      string `json:"person"`    // "A" oder "B" (Ehegatten); leer = gemeinsam
	Kategorie   string `json:"kategorie"` // Schlüssel aus Kategorien, z.B. "n.arbeitsmittel"
	Bezeichnung string `json:"bezeichnung"`
	BetragCent  int64  `json:"betrag_cent"`
	Datum       string `json:"datum"` // JJJJ-MM-TT oder leer
	Notiz       string `json:"notiz"`
	Bestaetigt  bool   `json:"bestaetigt"`
	Vorschlag   bool   `json:"vorschlag"` // true = automatisch vorgeschlagen
}

// Angabe ist ein Stammdatum je Steuerjahr (z.B. "homeoffice.tage" = "180").
// Die gültigen Schlüssel definiert das Paket rechner.
type Angabe struct {
	Steuerjahr int    `json:"steuerjahr"`
	Schluessel string `json:"schluessel"`
	Wert       string `json:"wert"`
}

// Vorjahreswert ist ein Betrag aus einem früheren Steuerbescheid, für den
// Vorjahresvergleich.
type Vorjahreswert struct {
	Steuerjahr int    `json:"steuerjahr"`
	Schluessel string `json:"schluessel"` // Kategorie oder Ergebnisgröße
	BetragCent int64  `json:"betrag_cent"`
}
