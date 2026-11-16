package web

import (
	"fmt"
	"net/http"
	"strings"

	"steuer/internal/modell"
	"steuer/internal/rechner"
)

type checkpunkt struct {
	Stufe   string   `json:"stufe"` // "wichtig" oder "hinweis"
	Titel   string   `json:"titel"`
	Text    string   `json:"text"`
	Details []string `json:"details"`
	Seite   string   `json:"seite"` // Ziel in der Oberfläche
}

func (s *Server) checkliste(w http.ResponseWriter, r *http.Request) {
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
	punkte := []checkpunkt{}
	add := func(p checkpunkt) {
		if p.Details == nil {
			p.Details = []string{}
		}
		punkte = append(punkte, p)
	}

	// 1. Unbestätigte Vorschläge und neue Belege
	offen := 0
	for _, p := range b.Positionen {
		if !p.Bestaetigt {
			offen++
		}
	}
	if offen > 0 {
		add(checkpunkt{"wichtig", fmt.Sprintf("%d Vorschläge noch nicht bestätigt", offen),
			"Nur bestätigte Positionen gehen in die Berechnung ein. Prüfe die Vorschläge bei den Belegen und bestätige oder verwirf sie.", nil, "belege"})
	}
	alle, err := s.Store.BelegeAlle()
	if err != nil {
		serverfehler(w, err)
		return
	}
	var neu []string
	for _, bl := range alle {
		if bl.Status == modell.StatusNeu && bl.Steuerjahr == jahr {
			neu = append(neu, fmt.Sprintf("Beleg %d: %s", bl.ID, strings.TrimSpace(bl.Art+" "+bl.Aussteller)))
		}
	}
	if len(neu) > 0 {
		add(checkpunkt{"wichtig", fmt.Sprintf("%d Belege noch nicht geprüft", len(neu)),
			"Öffne jeden neuen Beleg, kontrolliere die erkannten Daten und setze ihn auf bestätigt oder verworfen.", neu, "belege"})
	}

	// 2. Kategorien ohne Positionen
	hat := func(kat, person string) bool {
		for _, p := range b.Positionen {
			if p.Bestaetigt && p.Kategorie == kat && (person == "" || p.Person == person) {
				return true
			}
		}
		return false
	}
	for _, p := range []string{"A", "B"} {
		switch {
		case !hat("lstb.3", p):
			add(checkpunkt{"wichtig", "Keine Lohnsteuerbescheinigung für Person " + p,
				"Es gibt keinen bestätigten Bruttoarbeitslohn (Nr. 3) für Person " + p + ". Lege die Lohnsteuerbescheinigung in den Eingang oder erfasse die Werte unter „Positionen“. Ist Person " + p + " nicht angestellt, kannst Du das ignorieren.", nil, "positionen"})
		default:
			var fehlt []string
			for _, k := range []string{"lstb.4", "lstb.23a", "lstb.25", "lstb.26"} {
				if !hat(k, p) {
					fehlt = append(fehlt, kategorieTitel(k))
				}
			}
			if len(fehlt) > 0 {
				add(checkpunkt{"hinweis", "Lohnsteuerbescheinigung Person " + p + ": Werte fehlen",
					"Diese Nummern der Bescheinigung sind noch nicht erfasst (leer ist in Ordnung, wenn sie auf der Bescheinigung 0 sind):", fehlt, "positionen"})
			}
		}
	}
	if !hat("s.einnahmen", "") && !hat("s.einnahmen", "A") && !hat("s.einnahmen", "B") {
		add(checkpunkt{"wichtig", "Keine Betriebseinnahmen der selbständigen Tätigkeit",
			"Es sind keine Honorare erfasst. Lege Ausgangsrechnungen in den Eingang oder erfasse die Einnahmen unter „Positionen“.", nil, "positionen"})
	}
	if !hat("vor.kranken_pflege", "") && !hat("lstb.25", "") {
		add(checkpunkt{"hinweis", "Keine Kranken- und Pflegeversicherung erfasst",
			"Weder Beiträge aus der Lohnsteuerbescheinigung (Nr. 25/26) noch eigene Beiträge (Basisversorgung) sind erfasst.", nil, "positionen"})
	}
	if b.Angaben["az.vorhanden"] == "ja" && !hat("wohnung.miete", "") {
		add(checkpunkt{"hinweis", "Arbeitszimmer ohne Wohnungskosten",
			"Du hast ein Arbeitszimmer angegeben, aber keine Miete erfasst. Ohne Wohnungskosten ist nur die Pauschale möglich.", nil, "positionen"})
	}

	// 3. Angaben ohne Wert
	gruppen := []string{}
	fehlend := map[string][]string{}
	gesamt := map[string]int{}
	gesetzt := map[string]int{}
	for _, d := range rechner.AngabeDefs(jahr) {
		gesamt[d.Gruppe]++
		if strings.TrimSpace(b.Angaben[d.Schluessel]) != "" {
			gesetzt[d.Gruppe]++
			continue
		}
		if _, ok := fehlend[d.Gruppe]; !ok {
			gruppen = append(gruppen, d.Gruppe)
		}
		fehlend[d.Gruppe] = append(fehlend[d.Gruppe], d.Titel)
	}
	for _, g := range gruppen {
		if (g == "Kind 2" || g == "Kind 3") && gesetzt[g] == 0 {
			continue // Kinder, die es nicht gibt
		}
		add(checkpunkt{"hinweis", fmt.Sprintf("%s: %d von %d Angaben ohne Wert", g, len(fehlend[g]), gesamt[g]),
			"Leer lassen ist erlaubt, wenn die Angabe nicht zutrifft; es gelten dann die Standardwerte.", fehlend[g], "angaben"})
	}

	// 4. Hinweise der Berechnung
	warn := 0
	for _, h := range b.Ergebnis.Hinweise {
		if h.Stufe == "warnung" {
			warn++
		}
	}
	if warn > 0 {
		add(checkpunkt{"wichtig", fmt.Sprintf("%d Warnungen in der Erklärung", warn),
			"Die Berechnung hat Warnungen ausgegeben. Lies sie unter „Erklärung“, bevor Du etwas einträgst.", nil, "erklaerung"})
	}
	if b.Fehler != "" {
		add(checkpunkt{"wichtig", "Berechnung nicht möglich", b.Fehler, nil, "angaben"})
	}
	antwort(w, 200, map[string]any{"jahr": jahr, "punkte": punkte})
}

func kategorieTitel(k string) string {
	for _, c := range modell.Kategorien {
		if c.Schluessel == k {
			return c.Titel
		}
	}
	return k
}
