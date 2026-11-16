package rechner

import (
	"fmt"
	"math/big"
	"time"
)

// Ansatzarten für Homeoffice/Arbeitszimmer.
const (
	ansatzTP = "TP" // nur Tagespauschale
	ansatzJP = "JP" // Jahrespauschale (Monate mit Arbeitszimmer) + Tagespauschale davor
	ansatzTK = "TK" // tatsächliche Kosten (Monate mit Arbeitszimmer) + Tagespauschale davor
)

// Mittelpunkt-Status des Arbeitszimmers.
const (
	mpKeinRaum         = "kein Arbeitszimmer"
	mpJa               = "ja"
	mpRisiko           = "nur über einheitlichen Wirkungskreis"
	mpVertretbar       = "vertretbar (Haupttätigkeit zu Hause), Nachweise nötig"
	mpRegelmaessigNein = "regelmäßig nein"
	mpNein             = "nein"
	mpUnklar           = "unklar"
)

// azStatus prüft Raum und Mittelpunkt der gesamten Betätigung
// (Entscheidungsbaum Abschnitt 10 B/C der Arbeitszimmer-Recherche).
func (r *rechnung) azStatus() (string, string) {
	f := r.f
	switch {
	case !f.azVorhanden:
		return mpKeinRaum, "Kein abgeschlossener Raum angegeben (Arbeitsecke genügt nicht, BFH GrS 1/14)."
	case f.azDurchgang:
		return mpKeinRaum, "Durchgangszimmer: Die Rechtsprechung nimmt regelmäßig schädliche private Mitbenutzung an; konservativ kein Arbeitszimmer."
	case !f.azAusschliesslich:
		return mpKeinRaum, "Der Raum wird nicht (nahezu) ausschließlich beruflich genutzt; dann ist er kein häusliches Arbeitszimmer."
	}
	nebenRelevant := r.hatNeben && f.sPerson == f.azPerson
	an, ne := f.mpAnstellung, f.mpNeben
	if !nebenRelevant {
		switch an {
		case optJa:
			return mpJa, "Mittelpunkt der (einzigen) Tätigkeit nach Selbsteinschätzung im Arbeitszimmer."
		case optNein:
			return mpNein, "Mittelpunkt der Tätigkeit liegt nach Selbsteinschätzung außerhalb."
		}
		return mpUnklar, "Mittelpunkt der Tätigkeit ist nicht eingeschätzt."
	}
	switch {
	case an == optJa && ne == optJa:
		return mpJa, "Für Anstellung und Nebentätigkeit liegt der qualitative Schwerpunkt nach Selbsteinschätzung jeweils im Arbeitszimmer (Fallgruppe 1)."
	case an == optNein && ne == optNein:
		return mpNein, "Für beide Tätigkeiten liegt der Schwerpunkt außer Haus (Fallgruppe 2)."
	case an == optUnklar || ne == optUnklar:
		return mpUnklar, "Mittelpunkt mindestens einer Tätigkeit ist nicht eingeschätzt."
	case an == optJa:
		return mpVertretbar, "Das Arbeitszimmer ist Mittelpunkt der Anstellung (angenommen als Haupttätigkeit), nicht aber der Nebentätigkeit außer Haus. Die Verwaltung geht in dieser Fallgruppe regelmäßig davon aus, dass es nicht Mittelpunkt der gesamten Betätigung ist (BFH VI R 27/02; BMF 15.08.2023); der BFH verlangt aber eine Gesamtwürdigung, bei der der Mittelpunkt der Haupttätigkeit den Mittelpunkt der Gesamttätigkeit indiziert (BFH 16.12.2004, IV R 19/03, BStBl II 2005, 212; BFH 09.08.2011, VIII R 5/09). Vertretbar, wenn die Anstellung deutlich überwiegt – " + nachweisListe + "."
	case f.wirkungskreis:
		return mpRisiko, "Das Arbeitszimmer ist nur für eine Tätigkeit Mittelpunkt (Fallgruppe 3). Abzug nur, weil ein einheitlicher Wirkungskreis angegeben ist – das muss nachgewiesen werden; hohes Risiko."
	}
	return mpRegelmaessigNein, "Das Arbeitszimmer ist nur für die Nebentätigkeit Mittelpunkt, nicht für die Haupttätigkeit (Fallgruppe 3). Nach Verwaltungsauffassung ist es dann regelmäßig nicht Mittelpunkt der gesamten Betätigung (BFH VI R 27/02; BMF 15.08.2023)."
}

// azZulaessig meldet, ob ein Abzug nach Nr. 6b überhaupt in Betracht kommt.
func azZulaessig(status string) bool {
	return status == mpJa || status == mpVertretbar || status == mpRisiko || status == mpUnklar
}

// nachweisListe nennt die Nachweise für den Mittelpunkt im Arbeitszimmer.
const nachweisListe = "nötige Nachweise: Tätigkeitsbeschreibung (qualitative Gleichwertigkeit der Arbeit zu Hause und im Büro), Arbeitgeberbestätigung (Homeoffice-Vereinbarung, Anteil der Homeoffice-Tage, Büroarbeitsplatz), Tageskalender 2026 (Ort und Stunden je Tätigkeit), Auftragsliste der Nebentätigkeit mit Zeitaufwand, Grundriss und Fotos des Raums"

// hoErgebnis ist die Wirkung von Tagespauschale und Arbeitszimmer je Person.
type hoErgebnis struct {
	tageN58, tageN59, tageE int64
	tpN, tpE                int64
	azN, azE                int64
	azArt                   string
	azHerleitung            string
	tpHerleitung            string
	azSicherheit            string
	tageGekappt             int64
}

// monateOhneAZ ist die Zahl voller Kalendermonate vor dem Umzugsmonat.
func (f *fall) monateOhneAZ() int64 {
	if !f.hatUmzug {
		return 0
	}
	return int64(f.umzugMonat - 1)
}

// tatsaechlicheKosten berechnet die Arbeitszimmerkosten der neuen Wohnung
// (Flächenanteil) ab dem Umzug. ok=false, wenn nicht berechenbar.
func (r *rechnung) tatsaechlicheKosten() (betrag int64, herleitung string, ok bool) {
	f := r.f
	if f.azWohnflaeche.Sign() <= 0 || f.azFlaeche.Sign() <= 0 {
		return 0, "Wohnfläche oder Arbeitszimmerfläche fehlt.", false
	}
	var flaechenSumme int64
	var ohneDatum int
	for _, kat := range []string{"wohnung.miete", "wohnung.nebenkosten", "wohnung.strom"} {
		for _, p := range r.pos.liste(kat, "*") {
			d, err := time.Parse("2006-01-02", p.Datum)
			if err != nil {
				ohneDatum++
				continue
			}
			if f.hatUmzug && d.Before(f.umzug) {
				continue // alte Wohnung
			}
			if d.Year() != f.jahr {
				continue
			}
			flaechenSumme += p.BetragCent
		}
	}
	direkt := r.pos.summe("wohnung.zimmer_direkt", "*")
	if flaechenSumme == 0 && direkt == 0 {
		return 0, "Keine bestätigten Wohnungskosten der neuen Wohnung mit Datum erfasst.", false
	}
	q := new(big.Rat).Quo(f.azFlaeche, f.azWohnflaeche)
	anteilig := floorRat(new(big.Rat).Mul(new(big.Rat).SetInt64(flaechenSumme), q))
	if ohneDatum > 0 {
		r.hinweis("pruefen", "Arbeitszimmer: Wohnungskosten ohne Datum", fmt.Sprintf("%d Position(en) Miete/Nebenkosten/Strom ohne gültiges Datum sind bei den tatsächlichen Arbeitszimmerkosten nicht berücksichtigt (Zuordnung alte/neue Wohnung unmöglich).", ohneDatum))
	}
	if v := r.pos.summe("wohnung.versicherung", "*"); v > 0 {
		r.hinweis("pruefen", "Arbeitszimmer: Hausratversicherung", fmt.Sprintf("Hausratversicherung (%s) ist als Arbeitszimmerkosten strittig und wird nicht angesetzt.", euro(v)))
	}
	herleitung = fmt.Sprintf("Miete/Nebenkosten/Strom ab Umzug %s × %s m² / %s m² = %s; direkte Raumkosten %s",
		euro(flaechenSumme), formatZahl(f.azFlaeche), formatZahl(f.azWohnflaeche), euro(anteilig), euro(direkt))
	return anteilig + direkt, herleitung, true
}

// homeoffice berechnet Tagespauschale und Arbeitszimmer einer Person für
// einen Ansatz und eine Zuordnung (Anlage N / EÜR).
func (r *rechnung) homeoffice(p string) hoErgebnis {
	f := r.f
	d := f.person[p]
	rw := r.rw
	var h hoErgebnis
	tag := rw.cent("wk.homeoffice_tag")
	maxTage := rw.cent("wk.homeoffice_hoechst") / tag

	azHier := r.opt.az != ansatzTP && f.azPerson == p
	tage58 := d.hoVor + d.hoNach
	var tage59 int64
	if !d.andererArbeitsplatz {
		tage59 = d.hoMitFahrt
	}
	if azHier {
		// Tagespauschale nur für volle Monate vor dem Umzugsmonat (konservativ:
		// keine Tagespauschale in Monaten, die in den 6b-Abzug eingehen).
		if f.hatUmzug {
			tage58 = d.hoVor - d.hoUmzugsmonat
		} else {
			tage58 = 0
		}
		if tage59 > 0 {
			r.hinweis("pruefen", "Tagespauschale Zeile 59 neben Arbeitszimmer", "Tage mit Homeoffice und Fahrt (kein anderer Arbeitsplatz) sind nicht nach Zeiträumen vor/nach dem Umzug getrennt erfasst und werden bei Ansatz des Arbeitszimmers nicht gerechnet.")
			tage59 = 0
		}
	}
	if tage58+tage59 > maxTage {
		h.tageGekappt = tage58 + tage59 - maxTage
		if tage58 > maxTage {
			tage58, tage59 = maxTage, 0
		} else {
			tage59 = maxTage - tage58
		}
	}

	neben := r.hatNeben && f.sPerson == p
	// Aufteilung des 6b-Betrags (Anleitung Anlage N 2025 Zeile 57: nach
	// Nutzungsumfang oder insgesamt einer Tätigkeit).
	teile := func(n int64) (nN, nE int64) {
		if !neben {
			return n, 0
		}
		switch r.opt.zuordnung {
		case optGanzEUER:
			return 0, n
		case optNachAnteil:
			e := floorRat(new(big.Rat).Mul(new(big.Rat).SetInt64(n), new(big.Rat).Quo(f.anteilNeben, big.NewRat(100, 1))))
			return n - e, e
		}
		return n, 0
	}
	// Tagespauschale: der EÜR nur Tage zuordnen, an denen (auch) die
	// Nebentätigkeit zu Hause ausgeübt wurde (Anleitung Anlage N 2025 Zeile
	// 58/59). In Arbeitszimmer-Varianten ist die Lage dieser Tage vor/nach dem
	// Umzug nicht erfasst – dann konservativ keine Zuordnung zur EÜR.
	h.tageN58, h.tageE = teile(tage58)
	erlaubtE := min(d.hoMitNeben, tage58)
	if azHier {
		erlaubtE = 0
	}
	if h.tageE > erlaubtE {
		h.tageN58 += h.tageE - erlaubtE
		h.tageE = erlaubtE
	}
	h.tageN59 = tage59
	h.tpN = (h.tageN58 + h.tageN59) * tag
	h.tpE = h.tageE * tag
	h.tpHerleitung = fmt.Sprintf("%d Tage × %s", tage58+tage59, euro(tag))
	if h.tageGekappt > 0 {
		h.tpHerleitung += fmt.Sprintf(", gekappt auf %d Tage (Höchstbetrag %s)", maxTage, euro(rw.cent("wk.homeoffice_hoechst")))
	}
	if neben {
		h.tpHerleitung += fmt.Sprintf("; Zuordnung %s: Anlage N %d Tage, EÜR %d Tage", r.opt.zuordnung, h.tageN58+h.tageN59, h.tageE)
	}

	if azHier {
		var betrag int64
		switch r.opt.az {
		case ansatzJP:
			kuerz := f.monateOhneAZ() * rw.cent("wk.arbeitszimmer_monatskuerzung")
			betrag = rw.cent("wk.arbeitszimmer_jahrespauschale") - kuerz
			h.azArt = "Jahrespauschale"
			h.azHerleitung = fmt.Sprintf("%s − %d volle Monate ohne Arbeitszimmer × %s = %s",
				euro(rw.cent("wk.arbeitszimmer_jahrespauschale")), f.monateOhneAZ(), euro(rw.cent("wk.arbeitszimmer_monatskuerzung")), euro(betrag))
			h.azSicherheit = rw.si("wk.arbeitszimmer_jahrespauschale")
		case ansatzTK:
			b, her, ok := r.tatsaechlicheKosten()
			if ok {
				betrag = b
				h.azArt = "tatsächliche Kosten"
				h.azHerleitung = her
				h.azSicherheit = rw.si("wk.arbeitszimmer_flaechenschluessel")
			}
		}
		if f.hatUmzug {
			h.azHerleitung += fmt.Sprintf(" (Arbeitszimmer ab %s)", f.umzug.Format("02.01.2006"))
		}
		h.azN, h.azE = teile(betrag)
		if neben {
			h.azHerleitung += fmt.Sprintf("; Zuordnung %s: Anlage N %s, EÜR %s", r.opt.zuordnung, euro(h.azN), euro(h.azE))
		}
		if betrag+h.tpN+h.tpE > rw.cent("wk.homeoffice_hoechst") {
			r.hinweis("pruefen", "Arbeitszimmer und Tagespauschale zusammen", fmt.Sprintf("Arbeitszimmer (%s) und Tagespauschale (%s) übersteigen zusammen %s. Ob eine gemeinsame Obergrenze gilt, ist nicht geklärt (BMF-Original prüfen).", euro(betrag), euro(h.tpN+h.tpE), euro(rw.cent("wk.homeoffice_hoechst"))))
		}
	}
	return h
}
