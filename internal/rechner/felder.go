package rechner

import (
	"encoding/json"
	"sync"

	"steuer/regelwerk"
)

// Feld ist ein Eintrag des Feldkatalogs (Formularzeile, Stand Vordrucke 2025).
type Feld struct {
	Formular     string  `json:"formular"`
	Formularjahr string  `json:"formularjahr"`
	Zeile        *string `json:"zeile"`
	Kennziffer   *string `json:"kennziffer"`
	Bezeichnung  string  `json:"bezeichnung"`
	Inhalt       string  `json:"inhalt"`
	Belegart     string  `json:"belegart"`
	Quelle       string  `json:"quelle"`
	Sicherheit   string  `json:"sicherheit"`
}

var (
	feldkatalogEinmal sync.Once
	feldkatalog       []Feld
)

// Feldkatalog liefert alle Formularfelder aus regelwerk/felder_2025.json.
func Feldkatalog() []Feld {
	feldkatalogEinmal.Do(func() {
		if err := json.Unmarshal(regelwerk.Felder(), &feldkatalog); err != nil {
			panic("rechner: Feldkatalog ungültig: " + err.Error())
		}
	})
	out := make([]Feld, len(feldkatalog))
	copy(out, feldkatalog)
	return out
}

// Zuordnung verknüpft eine Kategorie (modell.Kategorien) mit der
// Formularstelle (Vordrucke 2025). Kennziffern "x / y" = Person A / Person B.
type Zuordnung struct {
	Kategorie  string
	Formular   string
	Zeile      string
	Kennziffer string
	Hinweis    string
}

// Zuordnungen liefert für jede Kategorie die Formularstelle.
func Zuordnungen() []Zuordnung {
	return []Zuordnung{
		{"lstb.3", "Anlage N", "5", "110", "Nummer der Lohnsteuerbescheinigung laut Feldkatalog nicht auf dem Vordruck verifiziert"},
		{"lstb.4", "Anlage N", "6", "140", "Nummer der Lohnsteuerbescheinigung laut Feldkatalog nicht auf dem Vordruck verifiziert"},
		{"lstb.5", "Anlage N", "7", "150", "Nummer der Lohnsteuerbescheinigung laut Feldkatalog nicht auf dem Vordruck verifiziert"},
		{"lstb.17", "Anlage N", "51", "290", "mindert die Entfernungspauschale"},
		{"lstb.22a", "Anlage Vorsorgeaufwand", "9", "304 / 404", ""},
		{"lstb.23a", "Anlage Vorsorgeaufwand", "4", "300 / 400", ""},
		{"lstb.25", "Anlage Vorsorgeaufwand", "11", "320 / 420", ""},
		{"lstb.26", "Anlage Vorsorgeaufwand", "13", "323 / 423", ""},
		{"lstb.27", "Anlage Vorsorgeaufwand", "43", "370 / 470", ""},
		{"n.arbeitsmittel", "Anlage N", "54-56", "320", "Einzelposten über 800 € nicht sofort abziehbar (AfA)"},
		{"n.fortbildung", "Anlage N", "60", "330", ""},
		{"n.reisekosten", "Anlage N", "66-69", "410", "Fahrt-, Übernachtungs-, Nebenkosten; Summe Zeile 69"},
		{"n.telefon_internet", "Anlage N", "62-64", "380", "weitere Werbungskosten"},
		{"n.gewerkschaft", "Anlage N", "53", "310", "Gewerkschaft ab 2026 neben dem Pauschbetrag; Berufsverbände nicht"},
		{"n.umzug", "Anlage N", "62-64", "380", "nur bei beruflicher Veranlassung"},
		{"n.sonstige", "Anlage N", "62-64", "380", "weitere Werbungskosten"},
		{"s.einnahmen", "Anlage EÜR", "12", "", "Kleinunternehmer: vereinnahmte Beträge"},
		{"s.fahrtkosten", "Anlage EÜR", "71", "", "Fahrten mit Privat-Pkw; Wege Wohnung–Betriebsstätte gehören in Zeile 72-73, Reisen in Zeile 44"},
		{"s.arbeitsmittel", "Anlage EÜR", "36", "", "GWG; über 800 € AfA (Zeile 31-35, Anlage AVEÜR)"},
		{"s.fortbildung", "Anlage EÜR", "60", "", "übrige unbeschränkt abziehbare Betriebsausgaben"},
		{"s.telefon_internet", "Anlage EÜR", "60", "", "übrige unbeschränkt abziehbare Betriebsausgaben"},
		{"s.bewirtung", "Anlage EÜR", "63", "", "70 % abziehbar"},
		{"s.sonstige", "Anlage EÜR", "60", "", "übrige unbeschränkt abziehbare Betriebsausgaben"},
		{"wohnung.miete", "Anlage N / Anlage EÜR", "57 / 65", "325", "nur über tatsächliche Arbeitszimmerkosten (Flächenanteil)"},
		{"wohnung.nebenkosten", "Anlage N / Anlage EÜR", "57 / 65", "325", "nur über tatsächliche Arbeitszimmerkosten (Flächenanteil)"},
		{"wohnung.strom", "Anlage N / Anlage EÜR", "57 / 65", "325", "nur über tatsächliche Arbeitszimmerkosten (Flächenanteil)"},
		{"wohnung.versicherung", "Anlage N / Anlage EÜR", "57 / 65", "325", "Hausrat als Arbeitszimmerkosten strittig: nicht gerechnet"},
		{"wohnung.zimmer_direkt", "Anlage N / Anlage EÜR", "57 / 65", "325", "voll, nur über tatsächliche Arbeitszimmerkosten"},
		{"vor.kranken_pflege", "Anlage Vorsorgeaufwand", "16-27", "", "Zeile je nach Versicherungsart (gesetzlich Selbstzahler 16-22, privat 23-27)"},
		{"vor.altersvorsorge", "Anlage Vorsorgeaufwand", "5 / 8", "301 / 401 bzw. 303 / 403", "Versorgungswerk Zeile 5, Rürup Zeile 8"},
		{"vor.sonstige", "Anlage Vorsorgeaufwand", "44-46", "500 / 501 / 502", "Zeile je nach Versicherungsart"},
		{"sa.spenden", "Anlage Sonderausgaben", "5", "123 / 124", ""},
		{"sa.parteispenden", "Anlage Sonderausgaben", "7", "127 / 128", ""},
		{"kind.betreuung", "Anlage Kind", "67", "51", "je Kind eine Anlage Kind"},
		{"kind.schulgeld", "Anlage Kind", "57", "56", "je Kind eine Anlage Kind"},
		{"agb.krankheit", "Anlage Außergewöhnliche Belastungen", "23-25", "302", ""},
		{"agb.sonstige", "Anlage Außergewöhnliche Belastungen", "36-38", "312", ""},
		{"h.minijob", "Anlage Haushaltsnahe Aufwendungen", "4", "202", ""},
		{"h.dienstleistung", "Anlage Haushaltsnahe Aufwendungen", "5", "212", ""},
		{"h.handwerker", "Anlage Haushaltsnahe Aufwendungen", "6-9", "214", "nur Lohn-, Maschinen- und Fahrtkosten"},
	}
}

// zuordnung liefert die Formularstelle einer Kategorie.
func zuordnung(kategorie string) Zuordnung {
	for _, z := range Zuordnungen() {
		if z.Kategorie == kategorie {
			return z
		}
	}
	return Zuordnung{Kategorie: kategorie}
}
