package rechner

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"steuer/internal/modell"
)

// Ergebniszeile sagt, was in welches Formularfeld einzutragen ist.
type Ergebniszeile struct {
	Formular        string
	Zeile           string
	Kennziffer      string
	Person          string
	Bezeichnung     string
	BetragCent      int64
	Wert            string // für Nicht-Geld-Einträge (Tage, km, Auswahl)
	Herleitung      string
	Rechtsgrundlage string
	Sicherheit      string
}

// Hinweis ist eine Information, Prüfbitte oder Warnung an den Nutzer.
type Hinweis struct {
	Stufe string // "info", "pruefen", "warnung"
	Thema string
	Text  string
}

// Variante ist eine alternative Berechnung mit Steuerwirkung.
type Variante struct {
	Titel        string
	SteuerCent   int64 // Einkommensteuer + Solidaritätszuschlag
	Beschreibung string
}

// Ergebnis ist das vollständige Rechenergebnis eines Jahres.
type Ergebnis struct {
	Zeilen            []Ergebniszeile
	Hinweise          []Hinweis
	Varianten         []Variante
	ZvECent           int64
	EStCent           int64
	SoliCent          int64
	VorausgezahltCent int64
	ErstattungCent    int64 // positiv = Erstattung, negativ = Nachzahlung
	Kennzahlen        map[string]int64
}

type optionen struct {
	az        string
	zuordnung string
}

// rechnung ist ein einzelner Rechendurchlauf mit festen Optionen.
type rechnung struct {
	rw       *Regelwerk
	f        *fall
	pos      posSammlung
	opt      optionen
	hatNeben bool
	e        Ergebnis
	gesehen  map[string]bool
}

func (r *rechnung) hinweis(stufe, thema, text string) {
	k := stufe + "|" + thema + "|" + text
	if r.gesehen[k] {
		return
	}
	r.gesehen[k] = true
	r.e.Hinweise = append(r.e.Hinweise, Hinweis{stufe, thema, text})
}

func (r *rechnung) zeile(z Ergebniszeile) {
	if z.Sicherheit == "" {
		z.Sicherheit = Sicher
	}
	r.e.Zeilen = append(r.e.Zeilen, z)
}

func (r *rechnung) kz(k string, v int64) { r.e.Kennzahlen[k] = v }

// Berechne rechnet die Einkommensteuer eines Jahres (Zusammenveranlagung).
// Nur bestätigte Positionen gehen ein.
func Berechne(jahr int, angabenWerte map[string]string, positionen []modell.Position) (Ergebnis, error) {
	rw, err := LadeRegelwerk(jahr)
	if err != nil {
		return Ergebnis{}, err
	}
	a := neueAngaben(jahr, angabenWerte)
	f := leseFall(a, jahr)
	if len(a.fehler) > 0 {
		return Ergebnis{}, errors.New("Eingaben fehlerhaft: " + strings.Join(a.fehler, "; "))
	}
	pos, posHinweise := sammle(positionen, jahr)

	neu := func(o optionen) *rechnung {
		r := &rechnung{rw: rw, f: f, pos: pos, opt: o, gesehen: map[string]bool{}}
		r.hatNeben = pos.vorhanden("s.", "*")
		return r
	}
	probe := neu(optionen{az: ansatzTP, zuordnung: optGanzN})
	status, statusText := probe.azStatus()

	ansaetze := []string{ansatzTP}
	tkGrund := ""
	if azZulaessig(status) || status == mpRegelmaessigNein {
		ansaetze = append(ansaetze, ansatzJP)
		if _, grund, ok := probe.tatsaechlicheKosten(); ok {
			ansaetze = append(ansaetze, ansatzTK)
		} else {
			tkGrund = grund
		}
	}
	zuordnungen := []string{f.zuordnung}
	if f.zuordnung == optOptimiert {
		zuordnungen = []string{optNachAnteil, optGanzN, optGanzEUER}
	}

	ergebnisse := map[string]*rechnung{}
	for _, az := range ansaetze {
		var best *rechnung
		for _, zu := range zuordnungen {
			r := neu(optionen{az: az, zuordnung: zu})
			r.rechne()
			if best == nil || steuer(r.e) < steuer(best.e) {
				best = r
			}
		}
		ergebnisse[az] = best
	}

	// Hauptansatz bestimmen.
	haupt := ansatzTP
	var wahlHinweise []Hinweis
	gewuenscht := map[string]string{optJahrespauschale: ansatzJP, optTatsaechlich: ansatzTK, optTagespauschale: ansatzTP}[f.azAnsatz]
	switch {
	case f.azAnsatz == optAutomatisch:
		if status == mpJa && f.nachweise {
			for _, az := range ansaetze {
				if steuer(ergebnisse[az].e) < steuer(ergebnisse[haupt].e) {
					haupt = az
				}
			}
		} else if status == mpJa && len(ansaetze) > 1 {
			wahlHinweise = append(wahlHinweise, Hinweis{"pruefen", "Arbeitszimmer: Nachweise fehlen", "Der Mittelpunkt ist nach Ihrer Selbsteinschätzung erfüllt, aber Nachweise sind nicht als vorhanden angegeben. Gerechnet wird deshalb nur die Tagespauschale; die Arbeitszimmer-Varianten stehen unter Varianten."})
		}
	case gewuenscht == ansatzTP:
	case !azZulaessig(status):
		wahlHinweise = append(wahlHinweise, Hinweis{"warnung", "Arbeitszimmer nicht angesetzt", fmt.Sprintf("Gewählt: %s. Nach Ihren Angaben ist das nicht zulässig (%s): %s Gerechnet wird die Tagespauschale.", f.azAnsatz, status, statusText)})
	case ergebnisse[gewuenscht] == nil:
		wahlHinweise = append(wahlHinweise, Hinweis{"warnung", "Arbeitszimmer nicht angesetzt", "Tatsächliche Kosten sind nicht berechenbar (Flächen oder datierte Wohnungskosten fehlen). Gerechnet wird die Tagespauschale."})
	default:
		haupt = gewuenscht
		if status != mpJa || !f.nachweise {
			wahlHinweise = append(wahlHinweise, Hinweis{"warnung", "Arbeitszimmer auf eigenes Risiko", fmt.Sprintf("Sie haben %s gewählt. Mittelpunkt: %s. %s Nachweise vorhanden: %s. Das Finanzamt kann den Abzug versagen.", f.azAnsatz, status, statusText, jaNeinText(f.nachweise))})
		}
	}

	h := ergebnisse[haupt]
	e := h.e
	e.Hinweise = append(posHinweise, e.Hinweise...)
	e.Hinweise = append(e.Hinweise, wahlHinweise...)
	if f.azVorhanden || f.azAnsatz != optAutomatisch {
		e.Hinweise = append(e.Hinweise, Hinweis{"info", "Arbeitszimmer: Mittelpunkt", fmt.Sprintf("Bewertung: %s. %s", status, statusText)})
	}
	if f.zuordnung == optOptimiert && h.hatNeben {
		e.Hinweise = append(e.Hinweise, Hinweis{"info", "Zuordnung Anlage N / EÜR", fmt.Sprintf("Tagespauschale und Arbeitszimmer sind %q zugeordnet, weil das die niedrigste Steuer ergibt. Zulässig ist die Aufteilung nach Nutzungsanteil oder die Zuordnung zu einer Tätigkeit (Anleitung Anlage N 2025, Zeilen 57-59). Werbungskosten wirken nur über dem Arbeitnehmer-Pauschbetrag, Betriebsausgaben ab dem ersten Euro.", h.opt.zuordnung)})
	}
	if tkGrund != "" {
		e.Hinweise = append(e.Hinweise, Hinweis{"info", "Tatsächliche Arbeitszimmerkosten nicht berechenbar", tkGrund + " Die Variante „tatsächliche Kosten“ fehlt deshalb."})
	}
	if f.alteWohnung {
		e.Hinweise = append(e.Hinweise, Hinweis{"pruefen", "Arbeitszimmer alte Wohnung", "Ein Arbeitszimmer in der alten Wohnung wird nicht gerechnet. Für die Monate vor dem Umzug setzt das Programm nur die Tagespauschale an."})
	}

	// Varianten.
	basis := steuer(ergebnisse[ansatzTP].e)
	for _, az := range ansaetze {
		r := ergebnisse[az]
		st := steuer(r.e)
		v := Variante{SteuerCent: st}
		wirkung := fmt.Sprintf("Steuer (ESt + Soli) %s", euro(st))
		if az != ansatzTP {
			wirkung += fmt.Sprintf(", Unterschied zur Tagespauschale %s", euro(st-basis))
		}
		zuo := ""
		if r.hatNeben {
			zuo = fmt.Sprintf(" Zuordnung N/EÜR: %s.", r.opt.zuordnung)
		}
		switch az {
		case ansatzTP:
			v.Titel = "Nur Tagespauschale"
			v.Beschreibung = wirkung + ". 6 € je Homeoffice-Tag, höchstens 1.260 €; kein Arbeitszimmer erforderlich. Risiko gering, Tage aufzeichnen." + zuo
		case ansatzJP:
			v.Titel = "Jahrespauschale für Arbeitszimmer" + abMonat(f)
			v.Beschreibung = wirkung + ". " + risikoText(status, statusText, f.nachweise) + " Tagespauschale nur für volle Monate vor dem Umzugsmonat." + zuo
		case ansatzTK:
			v.Titel = "Tatsächliche Arbeitszimmerkosten" + abMonat(f)
			v.Beschreibung = wirkung + ". " + risikoText(status, statusText, f.nachweise) + " Kosten nach Flächenanteil; gesonderte Aufzeichnung (§ 4 Abs. 7 EStG) für den EÜR-Anteil nötig." + zuo
		}
		if az == haupt {
			v.Titel += " (gerechnet)"
		}
		e.Varianten = append(e.Varianten, v)
	}

	e.Hinweise = append(e.Hinweise, Hinweis{"info", "Formularstand", fmt.Sprintf("Zeilen und Kennziffern stammen aus den Vordrucken %d; die Vordrucke %d waren am %s noch nicht veröffentlicht. Vor der Abgabe gegen die aktuellen Formulare prüfen.", rw.Formularjahr, jahr, rw.Stand)})
	e.Hinweise = append(e.Hinweise, Hinweis{"info", "Amtlicher Rechner", "Ein Abgleich der Tarifwerte mit www.bmf-steuerrechner.de war bei der Entwicklung nicht möglich (Zugriff gesperrt). Einzelwerte bitte stichprobenartig dort prüfen."})
	return e, nil
}

func steuer(e Ergebnis) int64 { return e.EStCent + e.SoliCent }

func jaNeinText(b bool) string {
	if b {
		return "ja"
	}
	return "nein"
}

func abMonat(f *fall) string {
	if !f.hatUmzug {
		return ""
	}
	return " ab " + monatsname(f.umzugMonat)
}

func monatsname(m int) string {
	return []string{"", "Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}[m]
}

func risikoText(status, text string, nachweise bool) string {
	switch status {
	case mpJa:
		if nachweise {
			return "Risiko mittel: Mittelpunkt nach Selbsteinschätzung erfüllt und Nachweise vorhanden."
		}
		return "Risiko erhöht: Mittelpunkt nach Selbsteinschätzung erfüllt, aber ohne Nachweise."
	case mpVertretbar:
		return "Risiko mittel: vertretbar, wenn die Anstellung als Haupttätigkeit deutlich überwiegt; das Finanzamt folgt nach Verwaltungsauffassung eher nicht – " + nachweisListe + "."
	case mpRegelmaessigNein:
		return "Risiko hoch / nach Verwaltungsauffassung regelmäßig NICHT abziehbar: " + text + " Nur mit Steuerberater."
	}
	return "Risiko hoch (" + status + "): " + text
}

// rechne führt die Berechnung mit festen Optionen durch.
func (r *rechnung) rechne() {
	rw, f := r.rw, r.f
	r.e.Kennzahlen = map[string]int64{}

	r.zeile(Ergebniszeile{Formular: "ESt 1 A", Zeile: "19", Bezeichnung: "Veranlagungsart", Wert: "Zusammenveranlagung", Herleitung: "Profil: verheiratet, Zusammenveranlagung", Rechtsgrundlage: "§§ 26, 26b EStG"})

	// Homeoffice/Arbeitszimmer je Person.
	ho := map[string]hoErgebnis{}
	for _, p := range []string{personA, personB} {
		ho[p] = r.homeoffice(p)
	}

	// Gewinn Nebentätigkeit (EÜR).
	gewinn := r.euer(ho[f.sPerson])

	// Einkünfte § 19.
	var summeEinkuenfte int64
	var arbeitnehmer bool
	var lohnsteuer, soliEinbehalten int64
	for _, p := range []string{personA, personB} {
		ein, aktiv := r.anlageN(p, ho[p])
		if aktiv {
			summeEinkuenfte += ein
			if r.pos.summe("lstb.3", p) > 0 {
				arbeitnehmer = true
			}
		}
		lohnsteuer += r.pos.summe("lstb.4", p)
		soliEinbehalten += r.pos.summe("lstb.5", p)
	}
	summeEinkuenfte += gewinn
	gde := summeEinkuenfte
	r.kz("summe_einkuenfte", summeEinkuenfte)
	r.kz("gesamtbetrag_einkuenfte", gde)

	// Pflichtveranlagung / Härteausgleich.
	grenze := rw.cent("haerteausgleich.grenze")
	ober := rw.cent("haerteausgleich.obergrenze")
	var haerte int64
	if r.hatNeben && arbeitnehmer && gewinn > 0 {
		switch {
		case gewinn <= grenze:
			haerte = gewinn
		case gewinn < ober:
			haerte = ober - gewinn
		}
		if haerte > 0 {
			r.hinweis("pruefen", "Härteausgleich", fmt.Sprintf("Härteausgleich %s vom Einkommen abgezogen (Nebeneinkünfte %s). Grenzen 410/820 € bei Zusammenveranlagung gemeinsam, nicht verdoppelt – nur sekundär belegt.", euro(haerte), euro(gewinn)))
		}
	}
	if r.hatNeben && gewinn > rw.cent("veranlagung.grenze_nebeneinkuenfte") {
		r.hinweis("info", "Pflichtveranlagung", "Einkünfte ohne Lohnsteuerabzug über 410 €: Abgabe der Erklärung ist Pflicht (§ 46 Abs. 2 Nr. 1 EStG), elektronisch mit Anlage EÜR.")
	}
	r.kz("haerteausgleich", haerte)

	// Sonderausgaben.
	sa := r.sonderausgaben(gde)
	// Außergewöhnliche Belastungen.
	agb := r.agb(gde)

	einkommen := gde - sa - agb
	r.kz("einkommen", einkommen)

	// Kinder und zvE.
	zvEOhneKFB := einkommen - haerte
	zvE, kfbAlle, hinzu := r.kinder(zvEOhneKFB)
	zvEE := zvEEuro(zvE)
	r.e.ZvECent = zvEE * 100
	r.kz("zve", zvEE*100)

	tarif := rw.SplittingEuro(zvEE) * 100
	r.hinweis("pruefen", "Rundung Splittingverfahren", fmt.Sprintf("Tarifliche ESt %s nach Splitting: halbes zvE auf volle Euro abgerundet, Tarif, abgerundet, verdoppelt. Die Rundungsreihenfolge folgt der Praxis der amtlichen Tabellen (Sicherheit %q); Abweichung höchstens wenige Euro.", euro(tarif), rw.si("tarif.splitting_rundung")))
	r.kz("tarifliche_est", tarif)
	erm34g, erm35a := r.ermaessigungen(tarif)
	est := tarif - erm34g - erm35a + hinzu
	if est < 0 {
		est = 0
	}
	r.e.EStCent = est

	// Soli: Bemessungsgrundlage immer mit Kinderfreibeträgen.
	tarifKFB := rw.SplittingEuro(zvEEuro(zvEOhneKFB-kfbAlle)) * 100
	e34, e35 := r.ermaessigungenOhneHinweis(tarifKFB)
	bmg := max(tarifKFB-e34-e35, 0)
	r.kz("soli_bemessungsgrundlage", bmg)
	r.e.SoliCent = rw.SoliCent(bmg)

	r.e.VorausgezahltCent = lohnsteuer + soliEinbehalten + f.vzESt + f.vzSoli
	r.e.ErstattungCent = r.e.VorausgezahltCent - r.e.EStCent - r.e.SoliCent
	r.kz("lohnsteuer", lohnsteuer)
	r.kz("soli_einbehalten", soliEinbehalten)
	r.kz("vorauszahlungen", f.vzESt+f.vzSoli)
	r.kz("est_festzusetzen", est)
	r.kz("soli", r.e.SoliCent)

	// Jede unsichere Zeile erzeugt einen Prüfhinweis.
	for _, z := range r.e.Zeilen {
		if z.Sicherheit != Sicher {
			ort := strings.TrimSpace(z.Formular + " Zeile " + z.Zeile)
			if z.Zeile == "" {
				ort = z.Formular
			}
			if z.Person != "" {
				ort += " (Person " + z.Person + ")"
			}
			r.hinweis("pruefen", ort+": "+z.Bezeichnung, fmt.Sprintf("Sicherheit %q. Herleitung: %s. Rechtsgrundlage: %s.", z.Sicherheit, strings.TrimSuffix(z.Herleitung, "."), z.Rechtsgrundlage))
		}
	}
}

// gwgFilter summiert Arbeitsmittel bis zur GWG-Grenze von 800 € netto
// (ohne Umsatzsteuer, auch ohne Vorsteuerabzug). Positionen sind
// Bruttobeträge: bis 800 € sicher, bis 952 € (Brutto bei 19 % USt) mit
// Prüfhinweis, darüber nicht gerechnet (AfA nötig). Positionen, deren
// Bezeichnung mit "AfA" beginnt, sind bereits Jahres-AfA-Beträge.
func (r *rechnung) gwgFilter(kat, person, ort string) (summe int64, n int, si string) {
	rw := r.rw
	netto := rw.cent("wk.gwg_grenze")
	brutto := rw.cent("wk.gwg_grenze_brutto_19")
	si = rw.si("wk.gwg_grenze")
	for _, p := range r.pos.liste(kat, person) {
		afa := strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.Bezeichnung)), "afa")
		switch {
		case p.BetragCent <= netto:
		case afa:
			si = schwaechste(si, StarkEvident)
			r.hinweis("pruefen", "AfA-Position", fmt.Sprintf("%s: %q (%s) ist als Jahres-AfA angesetzt. Bitte Anschaffungskosten, Nutzungsdauer und Monatsanteil im Anschaffungsjahr belegen.", ort, p.Bezeichnung, euro(p.BetragCent)))
		case p.BetragCent <= brutto:
			si = schwaechste(si, rw.si("wk.gwg_grenze_brutto_19"))
			r.hinweis("pruefen", "Arbeitsmittel zwischen 800 € und 952 € brutto", fmt.Sprintf("%s: %q (%s) ist sofort angesetzt, weil die GWG-Grenze 800 € ohne Umsatzsteuer beträgt (bei 19 %% USt bis 952 € brutto). Bitte den Nettobetrag der Rechnung prüfen: bei 7 %% USt liegt die Grenze bei 856 € brutto, ohne ausgewiesene USt bei 800 €. Liegt der Nettobetrag über 800 €, ist nur die Abschreibung abziehbar.", ort, p.Bezeichnung, euro(p.BetragCent)))
		default:
			r.hinweis("warnung", "Wirtschaftsgut über der GWG-Grenze", fmt.Sprintf("%s: %q (%s) übersteigt 800 € netto und ist NICHT gerechnet. Abziehbar ist nur die Abschreibung: Anschaffungskosten ÷ betriebsgewöhnliche Nutzungsdauer, im Anschaffungsjahr zeitanteilig ab dem Anschaffungsmonat. Bitte den Jahresbetrag als eigene Position mit einer Bezeichnung, die mit „AfA“ beginnt (z.B. „AfA Schreibtisch, 8 von 12 Monaten“), erfassen und diese Position verwerfen. Computerhardware und Software bitte in der Kategorie „Computerhardware und Software“ erfassen – sie sind im Anschaffungsjahr voll abziehbar.", ort, p.Bezeichnung, euro(p.BetragCent)))
			continue
		}
		summe += p.BetragCent
		n++
	}
	return summe, n, si
}

// anlageN rechnet die Einkünfte § 19 einer Person.
func (r *rechnung) anlageN(p string, h hoErgebnis) (einkuenfte int64, aktiv bool) {
	rw, f := r.rw, r.f
	d := f.person[p]
	brutto := r.pos.summe("lstb.3", p)
	aktiv = brutto > 0 || r.pos.vorhanden("n.", p) || r.pos.vorhanden("lstb.", p) || d.buerotageVor+d.buerotageNach+d.hoVor+d.hoNach > 0
	if !aktiv {
		return 0, false
	}
	const fN = "Anlage N"
	// Nr. 3/4/5 der Lohnsteuerbescheinigung = Bruttoarbeitslohn/Lohnsteuer/Soli
	// (BMF-Schreiben zur Ausstellung der Lohnsteuerbescheinigung, über
	// Sekundärquellen und ELSTER-Forum bestätigt; Zeilen laut Vordruck 2025).
	lstbSi := Sicher
	r.zeile(Ergebniszeile{Formular: fN, Zeile: "5", Kennziffer: "110", Person: p, Bezeichnung: "Bruttoarbeitslohn", BetragCent: brutto, Herleitung: "Summe Lohnsteuerbescheinigung(en) Nr. 3", Rechtsgrundlage: "§ 19 EStG", Sicherheit: lstbSi})
	r.zeile(Ergebniszeile{Formular: fN, Zeile: "6", Kennziffer: "140", Person: p, Bezeichnung: "Lohnsteuer", BetragCent: r.pos.summe("lstb.4", p), Herleitung: "Lohnsteuerbescheinigung Nr. 4", Rechtsgrundlage: "§ 36 Abs. 2 Nr. 2 EStG", Sicherheit: lstbSi})
	if s := r.pos.summe("lstb.5", p); s > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "7", Kennziffer: "150", Person: p, Bezeichnung: "Solidaritätszuschlag", BetragCent: s, Herleitung: "Lohnsteuerbescheinigung Nr. 5", Rechtsgrundlage: "§ 51a EStG; SolZG", Sicherheit: lstbSi})
	}

	// Entfernungspauschale.
	satz := rw.cent("wk.entfernung_satz")
	epVor := d.buerotageVor * d.kmVor * satz
	epNach := d.buerotageNach * d.kmNach * satz
	ep := epVor + epNach
	epSi := rw.si("wk.entfernung_satz")
	if f.hatUmzug && d.buerotageNach > 0 {
		epSi = schwaechste(epSi, rw.si("wk.entfernung_umzug_aufteilung"))
	}
	her := fmt.Sprintf("%d Tage × %d km × %s = %s", d.buerotageVor, d.kmVor, euro(satz), euro(epVor))
	if d.buerotageNach > 0 || f.hatUmzug {
		her += fmt.Sprintf(" (vor Umzug); %d Tage × %d km × %s = %s (ab Umzug)", d.buerotageNach, d.kmNach, euro(satz), euro(epNach))
	}
	if !d.pkw && ep > rw.cent("wk.entfernung_hoechst") {
		ep = rw.cent("wk.entfernung_hoechst")
		her += fmt.Sprintf("; Höchstbetrag ohne Pkw %s", euro(ep))
	}
	if d.oepnvKosten > ep {
		her += fmt.Sprintf("; tatsächliche Kosten öffentliche Verkehrsmittel %s höher", euro(d.oepnvKosten))
		ep = d.oepnvKosten
	}
	ag17 := r.pos.summe("lstb.17", p)
	ag18 := r.pos.summe("lstb.18", p)
	if ag17+ag18 > 0 {
		her += fmt.Sprintf("; abzüglich steuerfreie (%s) und pauschal besteuerte (%s) Arbeitgeberleistungen, höchstens bis 0 €", euro(ag17), euro(ag18))
		epSi = schwaechste(epSi, rw.si("wk.ag_fahrtkostenersatz_mindert"))
	}
	epNetto := max(ep-ag17-ag18, 0)
	if (d.kmVorRoh.Sign() > 0 && !d.kmVorRoh.IsInt()) || (d.kmNachRoh.Sign() > 0 && !d.kmNachRoh.IsInt()) {
		r.hinweis("info", "Entfernung abgerundet", "Für die Entfernungspauschale zählen nur volle Kilometer; Nachkommastellen wurden abgerundet.")
	}
	if d.buerotageVor > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "29", Kennziffer: "110", Person: p, Bezeichnung: "Erste Tätigkeitsstätte aufgesucht an Tagen" + umzugZusatz(f, true), Wert: itoa(d.buerotageVor) + " Tage", Herleitung: "Nur die Anzahl der Tage eintragen (kein Eurobetrag). Rechnung: " + her, Rechtsgrundlage: rw.rg("wk.entfernung_satz"), Sicherheit: epSi})
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "30", Kennziffer: "111", Person: p, Bezeichnung: "Einfache Entfernung" + umzugZusatz(f, true), Wert: itoa(d.kmVor) + " km", Herleitung: "volle Kilometer", Rechtsgrundlage: rw.rg("wk.entfernung_satz"), Sicherheit: epSi})
		if d.pkw {
			r.zeile(Ergebniszeile{Formular: fN, Zeile: "31", Kennziffer: "112", Person: p, Bezeichnung: "davon mit eigenem/überlassenem Pkw", Wert: itoa(d.kmVor) + " km", Herleitung: "Angabe Verkehrsmittel", Rechtsgrundlage: rw.rg("wk.entfernung_hoechst"), Sicherheit: epSi})
		}
	}
	if d.buerotageNach > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "37", Kennziffer: "130", Person: p, Bezeichnung: "2. Angabe: Tätigkeitsstätte aufgesucht an Tagen" + umzugZusatz(f, false), Wert: itoa(d.buerotageNach) + " Tage", Herleitung: "Nur die Anzahl der Tage eintragen (kein Eurobetrag). Rechnung: " + her + ". Zweite Angabe für den Zeitraum ab Umzug (gleiche Tätigkeitsstätte, neue Entfernung).", Rechtsgrundlage: rw.rg("wk.entfernung_umzug_aufteilung"), Sicherheit: schwaechste(epSi, StarkEvident)})
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "38", Kennziffer: "131", Person: p, Bezeichnung: "2. Angabe: einfache Entfernung" + umzugZusatz(f, false), Wert: itoa(d.kmNach) + " km", Herleitung: "volle Kilometer", Rechtsgrundlage: rw.rg("wk.entfernung_umzug_aufteilung"), Sicherheit: schwaechste(epSi, StarkEvident)})
		if d.pkw {
			r.zeile(Ergebniszeile{Formular: fN, Zeile: "39", Kennziffer: "132", Person: p, Bezeichnung: "2. Angabe: davon mit eigenem/überlassenem Pkw", Wert: itoa(d.kmNach) + " km", Herleitung: "Angabe Verkehrsmittel", Rechtsgrundlage: rw.rg("wk.entfernung_hoechst"), Sicherheit: schwaechste(epSi, StarkEvident)})
		}
	}
	if d.oepnvKosten > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "34", Kennziffer: "114", Person: p, Bezeichnung: "Aufwendungen für öffentliche Verkehrsmittel", BetragCent: d.oepnvKosten, Herleitung: "Angabe; wirkt nur, soweit höher als die Entfernungspauschale", Rechtsgrundlage: "§ 9 Abs. 2 Satz 2 EStG"})
	}
	if ag17 > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "51", Kennziffer: "290", Person: p, Bezeichnung: "Steuerfreie Arbeitgeberleistungen für Fahrten", BetragCent: ag17, Herleitung: "Lohnsteuerbescheinigung Nr. 17; mindert die Entfernungspauschale", Rechtsgrundlage: rw.rg("wk.ag_fahrtkostenersatz_mindert"), Sicherheit: rw.si("wk.ag_fahrtkostenersatz_mindert")})
	}
	if ag18 > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "51", Kennziffer: "295", Person: p, Bezeichnung: "Pauschal besteuerte Arbeitgeberleistungen für Fahrten", BetragCent: ag18, Herleitung: "Lohnsteuerbescheinigung Nr. 18; mindert die Entfernungspauschale", Rechtsgrundlage: rw.rg("wk.ag_fahrtkostenersatz_mindert"), Sicherheit: rw.si("wk.ag_fahrtkostenersatz_mindert")})
	}
	if bt := d.buerotageVor + d.buerotageNach + d.hoVor + d.hoNach; bt > rw.ganz("kalender.arbeitstage_sachsen") {
		r.hinweis("pruefen", "Arbeitstage Person "+p, fmt.Sprintf("Büro- und Homeoffice-Tage (%d) übersteigen die %d möglichen Arbeitstage Mo–Fr 2026 in Sachsen. Bitte prüfen (Wochenendarbeit belegen).", bt, rw.ganz("kalender.arbeitstage_sachsen")))
	}

	// Gewerkschaft (neben dem Pauschbetrag) und Berufsverbände.
	gew := r.pos.summe("n.gewerkschaft", p)
	bv := r.pos.summe("n.berufsverband", p)
	if gew+bv > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "53", Kennziffer: "310", Person: p, Bezeichnung: "Beiträge zu Berufsverbänden (einschließlich Gewerkschaft)", BetragCent: gew + bv, Herleitung: fmt.Sprintf("Gewerkschaft %s (zusätzlich zum Arbeitnehmer-Pauschbetrag) + Berufsverbände %s", euro(gew), euro(bv)), Rechtsgrundlage: "§ 9 Abs. 1 Satz 3 Nr. 3 EStG; " + rw.rg("wk.gewerkschaft_neben_pauschbetrag"), Sicherheit: rw.si("wk.gewerkschaft_neben_pauschbetrag")})
	}
	if gew > 0 {
		r.hinweis("pruefen", "Gewerkschaftsbeiträge", "Seit 2026 werden Gewerkschaftsbeiträge zusätzlich zum Arbeitnehmer-Pauschbetrag berücksichtigt (§ 9a Satz 3 EStG). Bitte unter „Gewerkschaft“ nur echte Gewerkschaftsbeiträge erfassen; Kammer- und sonstige Verbandsbeiträge unter „Berufsverbände“ (ob Kammerbeiträge ebenfalls begünstigt sind, ist strittig).")
	}
	// Arbeitsmittel und Computer.
	am, amN, amSi := r.gwgFilter("n.arbeitsmittel", p, "Anlage N Person "+p)
	comp := r.pos.summe("n.computer", p)
	if am+comp > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "54-56", Kennziffer: "320", Person: p, Bezeichnung: "Arbeitsmittel", BetragCent: am + comp, Herleitung: fmt.Sprintf("%d Arbeitsmittel bis 800 € netto (geringwertige Wirtschaftsgüter) = %s; Computerhardware/Software (Nutzungsdauer 1 Jahr, voll im Anschaffungsjahr) = %s; Art und Betrag je Zeile, Summe Zeile 56", amN, euro(am), euro(comp)), Rechtsgrundlage: rw.rg("wk.gwg_grenze") + "; " + rw.rg("wk.computer_nutzungsdauer_jahre"), Sicherheit: schwaechste(amSi, rw.si("wk.computer_nutzungsdauer_jahre"))})
		r.hinweis("pruefen", "Arbeitsmittel Nutzungsanteil", "Arbeitsmittel sind nur mit dem beruflichen Nutzungsanteil abziehbar. Bitte nur diesen Anteil als Position erfassen (keine Raumausstattung wie Tapete/Teppich – die gehört zum Arbeitszimmer).")
	}
	// Arbeitszimmer.
	if h.azN > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "57", Kennziffer: "325", Person: p, Bezeichnung: "Häusliches Arbeitszimmer (" + h.azArt + ")", BetragCent: h.azN, Herleitung: h.azHerleitung, Rechtsgrundlage: "§ 4 Abs. 5 Satz 1 Nr. 6b EStG i.V.m. § 9 Abs. 5 EStG", Sicherheit: h.azSicherheit})
	}
	// Tagespauschale.
	if h.tageN58 > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "58", Kennziffer: "335", Person: p, Bezeichnung: "Tagespauschale: Tage überwiegend in der häuslichen Wohnung (anderer Arbeitsplatz vorhanden)", Wert: itoa(h.tageN58) + " Tage", BetragCent: h.tageN58 * rw.cent("wk.homeoffice_tag"), Herleitung: h.tpHerleitung, Rechtsgrundlage: rw.rg("wk.homeoffice_tag"), Sicherheit: rw.si("wk.homeoffice_tag")})
	}
	if h.tageN59 > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "59", Kennziffer: "336", Person: p, Bezeichnung: "Tagespauschale: dauerhaft kein anderer Arbeitsplatz", Wert: itoa(h.tageN59) + " Tage", BetragCent: h.tageN59 * rw.cent("wk.homeoffice_tag"), Herleitung: h.tpHerleitung, Rechtsgrundlage: "§ 4 Abs. 5 Satz 1 Nr. 6c Satz 2 EStG", Sicherheit: rw.si("wk.homeoffice_tag")})
		r.hinweis("pruefen", "Kein anderer Arbeitsplatz", "Für Zeile 59 muss dargelegt werden, dass dauerhaft kein anderer Arbeitsplatz zur Verfügung steht (Arbeitgeberbescheinigung als Indiz).")
	}
	if h.tageGekappt > 0 {
		r.hinweis("info", "Tagespauschale gekappt Person "+p, fmt.Sprintf("%d Homeoffice-Tage liegen über dem Höchstbetrag von 210 Tagen und wirken nicht.", h.tageGekappt))
	}
	// Fortbildung.
	fb := r.pos.summe("n.fortbildung", p)
	if fb > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "60", Kennziffer: "330", Person: p, Bezeichnung: "Fortbildungskosten", BetragCent: fb, Herleitung: "Summe Positionen n.fortbildung (abzüglich Erstattungen erfassen)", Rechtsgrundlage: "§ 9 Abs. 6 EStG; § 9 Abs. 1 Satz 1 EStG"})
	}
	// Weitere Werbungskosten.
	tel := r.pos.summe("n.telefon_internet", p)
	sonst := r.pos.summe("n.sonstige", p)
	umz := r.pos.summe("n.umzug", p)
	if umz > 0 && !f.umzugBeruflich {
		r.hinweis("warnung", "Umzugskosten nicht angesetzt", fmt.Sprintf("Umzugskosten Person %s (%s) sind nicht gerechnet, weil kein eigenständiger beruflicher Grund angegeben ist. Ein Umzug, um ein Arbeitszimmer einzurichten, ist nicht beruflich veranlasst (BFH 05.02.2025, VI R 3/23).", p, euro(umz)))
		umz = 0
	}
	if umz > 0 {
		r.hinweis("pruefen", "Umzugskosten", "Umzugskosten sind mit beruflicher Veranlassung angesetzt (Einzelnachweis). Die Pauschale für sonstige Umzugsauslagen 2026 ist widersprüchlich belegt (964/643 € oder 1.022/681 €) und wird nicht gerechnet. Bei überwiegendem Homeoffice ist die 1-Stunden-Fahrzeitverkürzung kaum erreichbar.")
	}
	weitere := tel + sonst + umz
	if weitere > 0 {
		si := Sicher
		if umz > 0 {
			si = StarkEvident
		}
		if tel > 0 {
			r.hinweis("pruefen", "Telefon/Internet", "Ohne Einzelnachweis werden pauschal 20 % der Rechnung, höchstens 20 € monatlich anerkannt (R 9.1 Abs. 5 LStR, nicht im Original geprüft). Bitte nur den beruflichen Anteil erfassen.")
			si = schwaechste(si, StarkEvident)
		}
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "62-64", Kennziffer: "380", Person: p, Bezeichnung: "Weitere Werbungskosten", BetragCent: weitere, Herleitung: fmt.Sprintf("Telefon/Internet %s + Sonstige %s + Umzug %s", euro(tel), euro(sonst), euro(umz)), Rechtsgrundlage: "§ 9 Abs. 1 Satz 1 EStG", Sicherheit: si})
	}
	// Reisekosten.
	reise := r.pos.summe("n.reisekosten", p)
	if reise > 0 {
		r.zeile(Ergebniszeile{Formular: fN, Zeile: "66-69", Kennziffer: "410", Person: p, Bezeichnung: "Reisekosten (Fahrt, Übernachtung, Nebenkosten)", BetragCent: reise, Herleitung: "Summe Positionen n.reisekosten; Pkw pauschal 0,30 €/km", Rechtsgrundlage: "§ 9 Abs. 1 Satz 3 Nr. 4a und 5a EStG", Sicherheit: StarkEvident})
	}
	vpf := d.vpf8*rw.cent("wk.verpflegung_8h") + d.vpfAnAb*rw.cent("wk.verpflegung_anabreise") + d.vpf24*rw.cent("wk.verpflegung_24h")
	vpfNetto := max(vpf-d.vpfKuerzung-d.vpfErsatz, 0)
	if vpf > 0 {
		vh := fmt.Sprintf("%d × %s + %d × %s + %d × %s = %s; Kürzung %s; Ersatz %s",
			d.vpf8, euro(rw.cent("wk.verpflegung_8h")), d.vpfAnAb, euro(rw.cent("wk.verpflegung_anabreise")), d.vpf24, euro(rw.cent("wk.verpflegung_24h")), euro(vpf), euro(d.vpfKuerzung), euro(d.vpfErsatz))
		for _, t := range []struct {
			z, kz, b string
			n        int64
		}{{"72", "470", "Verpflegung: Tage über 8 Stunden", d.vpf8}, {"73", "471", "Verpflegung: An- und Abreisetage", d.vpfAnAb}, {"74", "472", "Verpflegung: Tage mit 24 Stunden", d.vpf24}} {
			if t.n > 0 {
				r.zeile(Ergebniszeile{Formular: fN, Zeile: t.z, Kennziffer: t.kz, Person: p, Bezeichnung: t.b, Wert: itoa(t.n) + " Tage", Herleitung: vh, Rechtsgrundlage: "§ 9 Abs. 4a EStG"})
			}
		}
		if d.vpfKuerzung > 0 {
			r.zeile(Ergebniszeile{Formular: fN, Zeile: "75", Kennziffer: "473", Person: p, Bezeichnung: "Kürzung wegen Mahlzeitengestellung", BetragCent: d.vpfKuerzung, Herleitung: "Angabe", Rechtsgrundlage: "§ 9 Abs. 4a Satz 8 EStG"})
		}
		if d.vpfErsatz > 0 {
			r.zeile(Ergebniszeile{Formular: fN, Zeile: "77", Kennziffer: "490", Person: p, Bezeichnung: "Verpflegung vom Arbeitgeber steuerfrei ersetzt", BetragCent: d.vpfErsatz, Herleitung: "Angabe", Rechtsgrundlage: "§ 3c EStG"})
		}
		r.hinweis("pruefen", "Verpflegungspauschalen", "Nur für die ersten drei Monate an derselben auswärtigen Tätigkeitsstätte (§ 9 Abs. 4a Satz 6 EStG).")
	}

	wkOhneGew := epNetto + h.tpN + h.azN + bv + am + comp + fb + weitere + reise + vpfNetto
	wk := wkOhneGew + gew
	pausch := min(rw.cent("wk.arbeitnehmer_pauschbetrag"), brutto)
	abziehbar := max(wkOhneGew, pausch) + gew
	einkuenfte = brutto - abziehbar
	r.kz("bruttolohn_"+p, brutto)
	r.kz("entfernungspauschale_"+p, epNetto)
	r.kz("tagespauschale_N_"+p, h.tpN)
	r.kz("arbeitszimmer_N_"+p, h.azN)
	r.kz("werbungskosten_"+p, wk)
	r.kz("werbungskosten_abziehbar_"+p, abziehbar)
	r.kz("einkuenfte_19_"+p, einkuenfte)
	if wkOhneGew <= pausch {
		t := fmt.Sprintf("Werbungskosten %s liegen nicht über dem Pauschbetrag; angesetzt %s", euro(wkOhneGew), euro(pausch))
		if gew > 0 {
			t += fmt.Sprintf(" zuzüglich Gewerkschaftsbeiträge %s", euro(gew))
		}
		r.hinweis("info", "Arbeitnehmer-Pauschbetrag Person "+p, t+".")
	}
	return einkuenfte, true
}

func umzugZusatz(f *fall, vor bool) string {
	if !f.hatUmzug {
		return ""
	}
	if vor {
		return " (bis " + f.umzug.AddDate(0, 0, -1).Format("02.01.2006") + ")"
	}
	return " (ab " + f.umzug.Format("02.01.2006") + ")"
}

// euer rechnet den Gewinn der Nebentätigkeit (Anlage EÜR).
func (r *rechnung) euer(h hoErgebnis) int64 {
	if !r.hatNeben {
		return 0
	}
	rw, f := r.rw, r.f
	const fE = "Anlage EÜR"
	p := f.sPerson
	si := Sicher
	if !f.kleinunternehmer {
		si = Unsicher
		r.hinweis("warnung", "Umsatzsteuer nicht gerechnet", "Ohne Kleinunternehmerregelung müssen Umsatzsteuer, Vorsteuer und Zeilen 15-17/57 der EÜR berücksichtigt werden. Das Programm rechnet das nicht; der Gewinn ist unsicher.")
	}
	art := "unklar"
	anlage := "Anlage S oder G"
	switch f.sArt {
	case optParagraph18:
		art, anlage = "Selbständige Arbeit", "Anlage S"
	case optParagraph15:
		art, anlage = "Gewerbebetrieb", "Anlage G"
	default:
		r.hinweis("pruefen", "Einkunftsart Nebentätigkeit", "§ 15 oder § 18 ist nicht festgelegt. Moderation ist nach BFH VIII R 5/12 im Zweifel gewerblich (Anlage G), freiberuflich nur bei künstlerischer/journalistischer Leistung (Anlage S). Die Einkommensteuer ist in beiden Fällen gleich, solange keine Gewerbesteuer anfällt.")
	}
	r.zeile(Ergebniszeile{Formular: fE, Zeile: "8", Person: p, Bezeichnung: "Einkunftsart", Wert: art, Herleitung: "Angabe Einkunftsart", Rechtsgrundlage: "§ 15 / § 18 EStG", Sicherheit: map[bool]string{true: StarkEvident, false: Unsicher}[f.sArt != optUnklar]})
	r.zeile(Ergebniszeile{Formular: fE, Zeile: "9", Person: p, Bezeichnung: "Betriebsinhaber", Wert: "Person " + p, Herleitung: "Angabe", Rechtsgrundlage: "§ 60 Abs. 4 EStDV"})

	einnahmen := r.pos.summe("s.einnahmen", "*")
	r.zeile(Ergebniszeile{Formular: fE, Zeile: "12", Person: p, Bezeichnung: "Betriebseinnahmen als umsatzsteuerlicher Kleinunternehmer", BetragCent: einnahmen, Herleitung: "Summe vereinnahmter Honorare (s.einnahmen)", Rechtsgrundlage: "§ 4 Abs. 3 EStG; § 19 UStG", Sicherheit: si})

	gwg, gwgN, gwgSi := r.gwgFilter("s.arbeitsmittel", "*", "EÜR")
	if gwg > 0 {
		r.zeile(Ergebniszeile{Formular: fE, Zeile: "36", Person: p, Bezeichnung: "Geringwertige Wirtschaftsgüter / Arbeitsmittel", BetragCent: gwg, Herleitung: fmt.Sprintf("%d Position(en) bis 800 € ohne Umsatzsteuer (Kleinunternehmer: Grenze gilt für den Nettobetrag, abgezogen wird der gezahlte Bruttobetrag)", gwgN), Rechtsgrundlage: rw.rg("wk.gwg_grenze"), Sicherheit: schwaechste(si, gwgSi)})
	}
	scomp := r.pos.summe("s.computer", "*")
	if scomp > 0 {
		r.zeile(Ergebniszeile{Formular: fE, Zeile: "31-33", Person: p, Bezeichnung: "Absetzung für Abnutzung: Computerhardware/Software (Nutzungsdauer 1 Jahr)", BetragCent: scomp, Herleitung: "Summe s.computer, im Anschaffungsjahr voll abgezogen; genaue AfA-Zeile im Formular prüfen", Rechtsgrundlage: rw.rg("wk.computer_nutzungsdauer_jahre"), Sicherheit: schwaechste(si, StarkEvident)})
		r.hinweis("pruefen", "EÜR Computer", "Computerhardware und Software sind trotz Sofortabzug in der Anlage AVEÜR zu erfassen (Anleitung EÜR 2025, Zeilen 31-38). Nur den betrieblichen Nutzungsanteil erfassen.")
	}
	fahrt := r.pos.summe("s.fahrtkosten", "*")
	if fahrt > 0 {
		r.zeile(Ergebniszeile{Formular: fE, Zeile: "71", Person: p, Bezeichnung: "Fahrtkosten für nicht zum Betriebsvermögen gehörende Fahrzeuge", BetragCent: fahrt, Herleitung: "Summe s.fahrtkosten (Pkw pauschal 0,30 €/km)", Rechtsgrundlage: "§ 4 Abs. 4 EStG; H 4.12 EStH", Sicherheit: StarkEvident})
		r.hinweis("pruefen", "EÜR Fahrtkosten", "Fahrten zu Auftraggebern/Veranstaltungsorten gehören in Zeile 71 (bzw. Reisekosten Zeile 44); Wege Wohnung – Betriebsstätte nur mit Entfernungspauschale in Zeile 72-73.")
	}
	uebrige := r.pos.summe("s.fortbildung", "*") + r.pos.summe("s.telefon_internet", "*") + r.pos.summe("s.sonstige", "*")
	if uebrige > 0 {
		r.zeile(Ergebniszeile{Formular: fE, Zeile: "60", Person: p, Bezeichnung: "Übrige unbeschränkt abziehbare Betriebsausgaben", BetragCent: uebrige, Herleitung: "Fortbildung + Telefon/Internet + Sonstige", Rechtsgrundlage: "§ 4 Abs. 4 EStG", Sicherheit: si})
	}
	bew := r.pos.summe("s.bewirtung", "*")
	bewAbz := prozentAb(bew, rw.rat("ba.bewirtung_abziehbar"))
	if bew > 0 {
		r.zeile(Ergebniszeile{Formular: fE, Zeile: "63", Person: p, Bezeichnung: "Bewirtungsaufwendungen (abziehbarer Teil)", BetragCent: bewAbz, Herleitung: fmt.Sprintf("70 %% von %s; nicht abziehbar %s", euro(bew), euro(bew-bewAbz)), Rechtsgrundlage: rw.rg("ba.bewirtung_abziehbar"), Sicherheit: rw.si("ba.bewirtung_abziehbar")})
		r.hinweis("pruefen", "Bewirtung", "Bewirtungsbelege brauchen Ort, Tag, Teilnehmer, Anlass und Höhe; bei Gaststätten die Rechnung.")
	}
	if h.azE > 0 {
		r.zeile(Ergebniszeile{Formular: fE, Zeile: "65", Person: p, Bezeichnung: "Häusliches Arbeitszimmer (" + h.azArt + ")", BetragCent: h.azE, Herleitung: h.azHerleitung, Rechtsgrundlage: "§ 4 Abs. 5 Satz 1 Nr. 6b EStG; § 4 Abs. 7 EStG", Sicherheit: h.azSicherheit})
		r.hinweis("pruefen", "EÜR Arbeitszimmer", "Arbeitszimmerkosten müssen in der EÜR einzeln und getrennt aufgezeichnet sein (§ 4 Abs. 7 EStG), sonst nicht abziehbar.")
	}
	if h.tpE > 0 {
		r.zeile(Ergebniszeile{Formular: fE, Zeile: "66", Person: p, Bezeichnung: "Tagespauschale häusliche Wohnung", Wert: itoa(h.tageE) + " Tage", BetragCent: h.tpE, Herleitung: h.tpHerleitung, Rechtsgrundlage: rw.rg("wk.homeoffice_tag"), Sicherheit: rw.si("wk.homeoffice_tag")})
	}
	ausgaben := gwg + scomp + fahrt + uebrige + bewAbz + h.azE + h.tpE
	gewinn := einnahmen - ausgaben
	r.zeile(Ergebniszeile{Formular: fE, Zeile: "95", Person: p, Bezeichnung: "Gewinn/Verlust", BetragCent: gewinn, Herleitung: fmt.Sprintf("Einnahmen %s − Ausgaben %s", euro(einnahmen), euro(ausgaben)), Rechtsgrundlage: "§ 4 Abs. 3 EStG", Sicherheit: si})
	r.zeile(Ergebniszeile{Formular: anlage, Person: p, Bezeichnung: "Feld „Gewinn laut Anlage EÜR“ (keine Zeilennummer verfügbar)", BetragCent: gewinn, Herleitung: "Übernahme aus Anlage EÜR Zeile 95", Rechtsgrundlage: map[bool]string{true: "§ 18 EStG", false: "§ 15 / § 18 EStG"}[anlage == "Anlage S"], Sicherheit: Unsicher})
	r.kz("betriebseinnahmen", einnahmen)
	r.kz("betriebsausgaben", ausgaben)
	r.kz("gewinn_nebentaetigkeit", gewinn)
	r.kz("tagespauschale_EUER", h.tpE)
	r.kz("arbeitszimmer_EUER", h.azE)

	r.hinweis("info", "Betriebsausgabenpauschale", fmt.Sprintf("Die Betriebsausgabenpauschale für Nebentätigkeiten (%s %%, höchstens %s; Anleitung EÜR 2025 Zeile 24, BMF 06.04.2023) gilt nur für wissenschaftliche, künstlerische oder schriftstellerische Nebentätigkeit sowie nebenamtliche Lehr- und Prüfungstätigkeit, nicht für gewerbliche Moderation; sie wird nicht gerechnet.", rw.wert("ba.pauschale_nebenberuflich_prozent").Wert, euro(rw.cent("ba.pauschale_nebenberuflich_hoechst"))))
	if f.sArt == optParagraph15 && gewinn > rw.cent("gew.freibetrag") {
		r.hinweis("warnung", "Gewerbesteuer", "Gewinn über dem Gewerbesteuer-Freibetrag von 24.500 €: Gewerbesteuer und Ermäßigung nach § 35 EStG werden nicht gerechnet.")
	}
	if einnahmen > rw.cent("ust.kleinunternehmer_vorjahr") {
		r.hinweis("pruefen", "Kleinunternehmergrenze", "Einnahmen über 25.000 €: Die Kleinunternehmerregelung entfällt für das Folgejahr; prüfen, ob 2026 die 100.000-€-Grenze berührt ist.")
	}
	if gewinn < 0 {
		r.hinweis("pruefen", "Verlust Nebentätigkeit", "Der Verlust wird mit den übrigen Einkünften verrechnet. Bei dauerhaften Verlusten prüft das Finanzamt die Gewinnerzielungsabsicht.")
	}
	return gewinn
}

// sonderausgaben rechnet Vorsorgeaufwand und übrige Sonderausgaben.
func (r *rechnung) sonderausgaben(gde int64) int64 {
	rw, f := r.rw, r.f
	const fV = "Anlage Vorsorgeaufwand"
	var rvGesamt, agAnteil, basisKV, sonstige, hoechstSonst int64
	kvKuerzungen := []string{}
	for _, p := range []string{personA, personB} {
		d := f.person[p]
		an := r.pos.summe("lstb.23a", p)
		ag := r.pos.summe("lstb.22a", p)
		kv := r.pos.summe("lstb.25", p)
		pv := r.pos.summe("lstb.26", p)
		al := r.pos.summe("lstb.27", p)
		rvGesamt += an + ag
		agAnteil += ag
		kvAbz := kv
		if d.krankengeld && kv > 0 {
			k := anteilAuf(kv, rw.ganz("vorsorge.krankengeld_kuerzung"), 100)
			kvAbz = kv - k
			kvKuerzungen = append(kvKuerzungen, fmt.Sprintf("Person %s: %s − 4 %% = %s", p, euro(kv), euro(kvAbz)))
		}
		basisKV += kvAbz + pv
		sonstige += al
		zeile := func(z, kza, kzb, bez string, b int64, quelle string) {
			if b > 0 {
				kzx := kza
				if p == personB {
					kzx = kzb
				}
				r.zeile(Ergebniszeile{Formular: fV, Zeile: z, Kennziffer: kzx, Person: p, Bezeichnung: bez, BetragCent: b, Herleitung: quelle, Rechtsgrundlage: "§ 10 Abs. 1 Nr. 2 und 3 EStG"})
			}
		}
		zeile("4", "300", "400", "Arbeitnehmeranteil Rentenversicherung", an, "Lohnsteuerbescheinigung Nr. 23a")
		zeile("9", "304", "404", "Arbeitgeberanteil Rentenversicherung", ag, "Lohnsteuerbescheinigung Nr. 22a")
		zeile("11", "320", "420", "Arbeitnehmerbeiträge Krankenversicherung", kv, "Lohnsteuerbescheinigung Nr. 25")
		zeile("13", "323", "423", "Arbeitnehmerbeiträge soziale Pflegeversicherung", pv, "Lohnsteuerbescheinigung Nr. 26")
		zeile("43", "370", "470", "Arbeitnehmerbeiträge Arbeitslosenversicherung", al, "Lohnsteuerbescheinigung Nr. 27")
		if d.kvZuschuss {
			hoechstSonst += rw.cent("vorsorge.sonstige_hoechst_zuschuss")
		} else {
			hoechstSonst += rw.cent("vorsorge.sonstige_hoechst_selbstzahler")
		}
	}
	// Eigene Vorsorgepositionen.
	eigenAV := r.pos.summe("vor.altersvorsorge", "*")
	eigenKV := r.pos.summe("vor.kranken_pflege", "*")
	eigenSonst := r.pos.summe("vor.sonstige", "*")
	for _, t := range []struct {
		kat string
		b   int64
	}{{"vor.altersvorsorge", eigenAV}, {"vor.kranken_pflege", eigenKV}, {"vor.sonstige", eigenSonst}} {
		if t.b > 0 {
			zu := zuordnung(t.kat)
			r.zeile(Ergebniszeile{Formular: zu.Formular, Zeile: zu.Zeile, Kennziffer: zu.Kennziffer, Bezeichnung: modellTitel(t.kat), BetragCent: t.b, Herleitung: "Summe Positionen " + t.kat + "; " + zu.Hinweis, Rechtsgrundlage: "§ 10 Abs. 1 Nr. 2, 3, 3a EStG", Sicherheit: StarkEvident})
		}
	}
	if eigenKV > 0 {
		r.hinweis("pruefen", "Eigene Kranken-/Pflegeversicherung", "Positionen vor.kranken_pflege sind voll als Basisabsicherung gerechnet. Wahlleistungen (Chefarzt, Einbettzimmer) und Beiträge mit Krankengeldanspruch (4 % Kürzung) sind herauszurechnen.")
	}
	rvGesamt += eigenAV
	basisKV += eigenKV
	sonstige += eigenSonst

	hoechstAV := 2 * rw.cent("vorsorge.altersvorsorge_hoechst")
	av := max(min(rvGesamt, hoechstAV)-agAnteil, 0)
	if rvGesamt > hoechstAV {
		r.hinweis("pruefen", "Altersvorsorge-Höchstbetrag", "Der Höchstbetrag für Basis-Altersvorsorge (61.652 € zusammen) ist erreicht; der Betrag ist nur sekundär belegt (BBG KnRV × 24,7 %).")
	}
	sonstAbz := max(basisKV, min(hoechstSonst, basisKV+sonstige))
	vorsorge := av + sonstAbz
	r.kz("vorsorge_altersvorsorge", av)
	r.kz("vorsorge_basis_kv_pv", basisKV)
	r.kz("vorsorge_sonstige_abziehbar", sonstAbz)
	r.kz("vorsorgeaufwand", vorsorge)
	if len(kvKuerzungen) > 0 {
		r.hinweis("info", "Krankenversicherung", "Beiträge mit Anspruch auf Krankengeld um 4 % gekürzt: "+strings.Join(kvKuerzungen, "; ")+".")
	}
	if sonstige > 0 && basisKV >= hoechstSonst {
		r.hinweis("info", "Sonstige Vorsorgeaufwendungen", fmt.Sprintf("Die Basis-Kranken-/Pflegeversicherung (%s) erreicht den Höchstbetrag (%s); sonstige Vorsorgeaufwendungen (%s, z.B. Arbeitslosen-, Haftpflichtversicherung) wirken sich nicht aus.", euro(basisKV), euro(hoechstSonst), euro(sonstige)))
	}
	r.hinweis("info", "Günstigerprüfung Vorsorge", "Die Günstigerprüfung nach § 10 Abs. 4a EStG (altes Recht) wird nicht gerechnet; sie wirkt bei Arbeitnehmern mit gesetzlicher Rentenversicherung heute praktisch nicht mehr. Riester (§ 10a EStG) wird nicht gerechnet.")

	// Übrige Sonderausgaben.
	const fS = "Anlage Sonderausgaben"
	sp := r.pos.summe("sa.spenden", "*")
	spAbz := sp
	if grenze := prozentAb(max(gde, 0), rw.rat("sa.spenden_prozent")); spAbz > grenze {
		spAbz = grenze
		r.hinweis("pruefen", "Spenden über Höchstbetrag", fmt.Sprintf("Spenden %s übersteigen 20 %% des Gesamtbetrags der Einkünfte (%s); der Rest ist vortragsfähig (§ 10b Abs. 1 Satz 9 EStG) und wird nicht gerechnet.", euro(sp), euro(grenze)))
	}
	if sp > 0 {
		r.zeile(Ergebniszeile{Formular: fS, Zeile: "5", Kennziffer: "123 / 124", Bezeichnung: "Spenden und Mitgliedsbeiträge (steuerbegünstigte Zwecke)", BetragCent: sp, Herleitung: fmt.Sprintf("Summe sa.spenden; abziehbar %s", euro(spAbz)), Rechtsgrundlage: rw.rg("sa.spenden_prozent"), Sicherheit: rw.si("sa.spenden_prozent")})
		r.hinweis("pruefen", "Spendenbescheinigungen", "Zuwendungsbestätigungen vorhalten; vereinfachter Nachweis (Kontoauszug) nur bis 300 € je Zuwendung.")
	}
	ps := r.pos.summe("sa.parteispenden", "*")
	parteiSA := int64(0)
	if ps > 0 {
		verbraucht := min(ps, 2*rw.cent("sa.parteispenden_ermaessigung_hoechst_zusammen"))
		parteiSA = min(ps-verbraucht, rw.cent("sa.parteispenden_sonderausgaben_hoechst_zusammen"))
		r.zeile(Ergebniszeile{Formular: fS, Zeile: "7", Kennziffer: "127 / 128", Bezeichnung: "Spenden an politische Parteien", BetragCent: ps, Herleitung: fmt.Sprintf("50 %% Steuerermäßigung (höchstens %s), darüber hinaus Sonderausgaben %s", euro(rw.cent("sa.parteispenden_ermaessigung_hoechst_zusammen")), euro(parteiSA)), Rechtsgrundlage: "§ 34g Satz 2 EStG; § 10b Abs. 2 EStG"})
	}

	// Kinderbetreuung und Schulgeld (Höchstbeträge je Kind; nur Kinder mit
	// Anspruch auf Kindergeld/Freibetrag).
	const fK = "Anlage Kind"
	stichtagJahresanfang := time.Date(f.jahr, 1, 1, 0, 0, 0, 0, time.UTC)
	var betreuungsKinder, schulKinder []kindDaten
	for _, k := range f.kinder {
		if r.kindMonate(k) == 0 {
			continue
		}
		schulKinder = append(schulKinder, k)
		alter14 := k.geburt.AddDate(int(rw.ganz("kind.betreuung_alter")), 0, 0)
		if !k.haushalt || !alter14.After(stichtagJahresanfang) {
			continue
		}
		betreuungsKinder = append(betreuungsKinder, k)
		if alter14.Year() == f.jahr {
			r.hinweis("pruefen", fmt.Sprintf("Kind %d wird 14", k.nr), "Kinderbetreuungskosten sind nur bis zum 14. Geburtstag abziehbar; bitte nur Aufwendungen bis dahin erfassen.")
		}
	}
	kbAbz := int64(0)
	if kb := r.pos.summe("kind.betreuung", "*"); kb > 0 {
		if len(betreuungsKinder) == 0 {
			r.hinweis("warnung", "Kinderbetreuung nicht angesetzt", "Kinderbetreuungskosten erfasst, aber kein Kind unter 14 Jahren im Haushalt mit Anspruch auf Kindergeld/Freibetrag angegeben (Ausnahme Behinderung wird nicht gerechnet).")
		} else {
			var her string
			kbAbz, her = r.jeKind("kind.betreuung", betreuungsKinder, rw.rat("kind.betreuung_prozent"), rw.cent("kind.betreuung_hoechst"), "Kinderbetreuung")
			r.zeile(Ergebniszeile{Formular: fK, Zeile: "67", Kennziffer: "51", Bezeichnung: "Kinderbetreuungskosten (Gesamtaufwendungen der Eltern, je Kind eigene Anlage Kind)", BetragCent: kb, Herleitung: her, Rechtsgrundlage: rw.rg("kind.betreuung_prozent"), Sicherheit: rw.si("kind.betreuung_prozent")})
			r.hinweis("pruefen", "Kinderbetreuung Nachweis", "Rechnung und unbare Zahlung nötig; keine Verpflegung, kein Unterricht, keine Freizeitbetätigung. Steuerfreie Arbeitgeberzuschüsse (Anlage Kind Zeile 68) mindern den Betrag.")
		}
	}
	sgAbz := int64(0)
	if sg := r.pos.summe("kind.schulgeld", "*"); sg > 0 {
		if len(schulKinder) == 0 {
			r.hinweis("warnung", "Schulgeld nicht angesetzt", "Schulgeld erfasst, aber kein Kind mit Anspruch auf Kindergeld/Freibetrag angegeben.")
		} else {
			var her string
			sgAbz, her = r.jeKind("kind.schulgeld", schulKinder, rw.rat("kind.schulgeld_prozent"), rw.cent("kind.schulgeld_hoechst"), "Schulgeld")
			r.zeile(Ergebniszeile{Formular: fK, Zeile: "57", Kennziffer: "56", Bezeichnung: "Schulgeld (ohne Beherbergung, Betreuung, Verpflegung; je Kind eigene Anlage Kind)", BetragCent: sg, Herleitung: her, Rechtsgrundlage: rw.rg("kind.schulgeld_prozent"), Sicherheit: rw.si("kind.schulgeld_prozent")})
		}
	}

	besondere := spAbz + parteiSA + kbAbz + sgAbz
	pausch := rw.cent("sa.pauschbetrag_zusammen")
	uebrige := max(besondere, pausch)
	if besondere < pausch {
		r.hinweis("info", "Sonderausgaben-Pauschbetrag", fmt.Sprintf("Spenden, Kinderbetreuung und Schulgeld (%s) liegen unter dem Pauschbetrag; angesetzt %s.", euro(besondere), euro(pausch)))
	}
	r.kz("sonderausgaben_uebrige", uebrige)
	r.kz("sonderausgaben", vorsorge+uebrige)
	return vorsorge + uebrige
}

func modellTitel(kat string) string {
	for _, k := range modell.Kategorien {
		if k.Schluessel == kat {
			return k.Titel
		}
	}
	return kat
}

// agb rechnet außergewöhnliche Belastungen nach Abzug der zumutbaren Belastung.
func (r *rechnung) agb(gde int64) int64 {
	rw := r.rw
	kr := r.pos.summe("agb.krankheit", "*")
	so := r.pos.summe("agb.sonstige", "*")
	summe := kr + so
	if summe == 0 {
		return 0
	}
	var proz [3]int64
	var n int // nur Kinder mit Anspruch auf Kindergeld/Freibetrag (§ 33 Abs. 3 EStG)
	for _, k := range r.f.kinder {
		if r.kindMonate(k) > 0 {
			n++
		}
	}
	switch {
	case n == 0:
		proz = rw.stufen("agb.prozent_ohne_kind_splitting")
	case n <= 2:
		proz = rw.stufen("agb.prozent_1_2_kinder")
	default:
		proz = rw.stufen("agb.prozent_3_kinder")
	}
	g := max(gde, 0)
	s1, s2 := rw.cent("agb.stufe1_bis"), rw.cent("agb.stufe2_bis")
	teil1 := min(g, s1)
	teil2 := max(min(g, s2)-s1, 0)
	teil3 := max(g-s2, 0)
	zumutbar := anteilAuf(teil1*proz[0]+teil2*proz[1]+teil3*proz[2], 1, 100)
	abz := max(summe-zumutbar, 0)
	her := fmt.Sprintf("zumutbare Belastung %d %% × %s + %d %% × %s + %d %% × %s = %s; abziehbar %s",
		proz[0], euro(teil1), proz[1], euro(teil2), proz[2], euro(teil3), euro(zumutbar), euro(abz))
	si := schwaechste(rw.si("agb.stufe1_bis"), rw.si("agb.stufenweise"))
	const fA = "Anlage Außergewöhnliche Belastungen"
	if kr > 0 {
		r.zeile(Ergebniszeile{Formular: fA, Zeile: "23-25", Kennziffer: "302", Bezeichnung: "Krankheitskosten", BetragCent: kr, Herleitung: her, Rechtsgrundlage: "§ 33 EStG", Sicherheit: si})
	}
	if so > 0 {
		r.zeile(Ergebniszeile{Formular: fA, Zeile: "36-38", Kennziffer: "312", Bezeichnung: "Sonstige außergewöhnliche Belastungen", BetragCent: so, Herleitung: her, Rechtsgrundlage: "§ 33 EStG", Sicherheit: schwaechste(si, StarkEvident)})
		r.hinweis("pruefen", "Sonstige außergewöhnliche Belastungen", "Ob die Aufwendungen zwangsläufig im Sinne des § 33 EStG sind, prüft das Programm nicht.")
	}
	r.hinweis("pruefen", "Erstattungen", "Krankheitskosten nur abzüglich Erstattungen (Krankenkasse, Versicherung) erfassen.")
	r.kz("agb_aufwendungen", summe)
	r.kz("zumutbare_belastung", zumutbar)
	r.kz("agb_abziehbar", abz)
	return abz
}

// kinder führt die Günstigerprüfung Kindergeld/Kinderfreibeträge durch.
// Liefert das zvE, die Summe aller Kinderfreibeträge (für den Soli) und die
// Hinzurechnung des Kindergeldanspruchs.
func (r *rechnung) kinder(zvEOhne int64) (zvE, kfbAlle, hinzu int64) {
	rw, f := r.rw, r.f
	zvE = zvEOhne
	jahresKFB := 2 * (rw.cent("kind.freibetrag_saechlich") + rw.cent("kind.freibetrag_bea"))
	kgMonat := rw.cent("kind.kindergeld_monat")
	var kgSumme int64
	for _, k := range f.kinder {
		monate := r.kindMonate(k)
		if monate == 0 {
			continue
		}
		kfb := anteil(jahresKFB, monate, 12)
		kg := monate * kgMonat
		kgSumme += kg
		kfbAlle += kfb
		ohne := rw.SplittingEuro(zvEEuro(zvE)) * 100
		mit := rw.SplittingEuro(zvEEuro(zvE-kfb)) * 100
		entlastung := ohne - mit
		fb := entlastung > kg
		her := fmt.Sprintf("Freibeträge %d/12 × %s = %s; Steuerentlastung %s gegenüber Kindergeldanspruch %d × %s = %s → %s",
			monate, euro(jahresKFB), euro(kfb), euro(entlastung), monate, euro(kgMonat), euro(kg),
			map[bool]string{true: "Freibeträge günstiger (Kindergeld wird hinzugerechnet)", false: "Kindergeld günstiger"}[fb])
		if fb {
			zvE -= kfb
			hinzu += kg
		}
		r.zeile(Ergebniszeile{Formular: "Anlage Kind", Zeile: "5-9", Bezeichnung: fmt.Sprintf("Kind %d: Geburtsdatum und Kindergeldanspruch", k.nr), Wert: k.geburt.Format("02.01.2006") + ", " + itoa(monate) + " Monate", BetragCent: kg, Herleitung: her, Rechtsgrundlage: "§ 31, § 32 Abs. 6, § 66 EStG", Sicherheit: rw.si("kind.freibetrag_saechlich")})
		r.zeile(Ergebniszeile{Formular: "Anlage Kind", Zeile: "10", Kennziffer: "02 / 03", Bezeichnung: fmt.Sprintf("Kind %d: Kindschaftsverhältnis zu Person A und B", k.nr), Wert: "1 = leibliches Kind/Adoptivkind (bitte bestätigen)", Herleitung: "Annahme: gemeinsames Kind beider Ehegatten", Rechtsgrundlage: "§ 32 Abs. 1 EStG", Sicherheit: StarkEvident})
	}
	if len(f.kinder) > 1 {
		r.hinweis("pruefen", "Günstigerprüfung mehrere Kinder", "Die Günstigerprüfung ist je Kind nacheinander gerechnet (Reihenfolge der Eingabe). Bei mehreren Kindern kann die Reihenfolge das Ergebnis geringfügig beeinflussen.")
	}
	r.kz("kinderfreibetraege_abgezogen", zvEOhne-zvE)
	r.kz("kinderfreibetraege_alle", kfbAlle)
	r.kz("kindergeld_anspruch", kgSumme)
	r.kz("kindergeld_hinzurechnung", hinzu)
	return zvE, kfbAlle, hinzu
}

// monateAusGeburt leitet die Anspruchsmonate aus dem Geburtsdatum ab
// (nur für Kinder, die das ganze Jahr unter 18 sind). −1 = nicht ableitbar.
func (r *rechnung) monateAusGeburt(k kindDaten) int64 {
	j := r.f.jahr
	if k.geburt.Year() == j {
		return int64(13 - int(k.geburt.Month())) // Monatsprinzip: Geburtsmonat zählt
	}
	if k.geburt.AddDate(18, 0, 0).Year() > j {
		return 12
	}
	r.hinweis("pruefen", fmt.Sprintf("Kind %d: volljährig", k.nr), "Das Kind ist 2026 volljährig oder wird es. Bitte die Monate mit Kindergeldanspruch angeben (Ausbildung usw.); ohne Angabe wird das Kind nicht berücksichtigt.")
	return -1
}

// ermaessigungen berechnet § 34g und § 35a, begrenzt auf die tarifliche ESt.
func (r *rechnung) ermaessigungen(tarif int64) (e34g, e35a int64) {
	e34g, e35a, zeilen := r.ermaessigungenRoh(tarif)
	for _, z := range zeilen {
		r.zeile(z)
	}
	r.kz("ermaessigung_34g", e34g)
	r.kz("ermaessigung_35a", e35a)
	if e35a > 0 {
		r.hinweis("pruefen", "Haushaltsnahe Aufwendungen", "Nur Arbeitskosten (ohne Material), Rechnung und unbare Zahlung. Kosten, die als Werbungskosten/Betriebsausgaben (z.B. beruflicher Umzug, Arbeitszimmer) oder Kinderbetreuung angesetzt sind, dürfen hier nicht nochmals erfasst werden. Höchstbeträge gelten im Umzugsjahr nur einmal.")
	}
	return e34g, e35a
}

func (r *rechnung) ermaessigungenOhneHinweis(tarif int64) (int64, int64) {
	a, b, _ := r.ermaessigungenRoh(tarif)
	return a, b
}

func (r *rechnung) ermaessigungenRoh(tarif int64) (e34g, e35a int64, zeilen []Ergebniszeile) {
	rw := r.rw
	rest := tarif
	if ps := r.pos.summe("sa.parteispenden", "*"); ps > 0 {
		e34g = min(prozentAb(ps, rw.rat("sa.parteispenden_ermaessigung_prozent")), rw.cent("sa.parteispenden_ermaessigung_hoechst_zusammen"), rest)
		rest -= e34g
	}
	const fH = "Anlage Haushaltsnahe Aufwendungen"
	for _, t := range []struct {
		kat, p, h, z, kz, bez string
	}{
		{"h.minijob", "35a.minijob_prozent", "35a.minijob_hoechst", "4", "202", "Geringfügige Beschäftigung im Privathaushalt"},
		{"h.dienstleistung", "35a.dienstleistung_prozent", "35a.dienstleistung_hoechst", "5", "212", "Haushaltsnahe Dienstleistungen"},
		{"h.handwerker", "35a.handwerker_prozent", "35a.handwerker_hoechst", "6-9", "214", "Handwerkerleistungen (Lohn-, Maschinen-, Fahrtkosten)"},
	} {
		b := r.pos.summe(t.kat, "*")
		if b == 0 {
			continue
		}
		erm := min(prozentAb(b, rw.rat(t.p)), rw.cent(t.h))
		wirk := min(erm, rest)
		rest -= wirk
		e35a += wirk
		zeilen = append(zeilen, Ergebniszeile{Formular: fH, Zeile: t.z, Kennziffer: t.kz, Bezeichnung: t.bez, BetragCent: b, Herleitung: fmt.Sprintf("20 %% von %s = %s, höchstens %s; wirksam %s", euro(b), euro(prozentAb(b, rw.rat(t.p))), euro(rw.cent(t.h)), euro(wirk)), Rechtsgrundlage: rw.rg(t.p), Sicherheit: rw.si(t.p)})
	}
	return e34g, e35a, zeilen
}

// kindMonate liefert die Monate mit Anspruch auf Kindergeld (0 = Kind wird
// nicht berücksichtigt).
func (r *rechnung) kindMonate(k kindDaten) int64 {
	if k.monate >= 0 {
		return k.monate
	}
	return max(r.monateAusGeburt(k), 0)
}

var kindMarke = regexp.MustCompile(`(?i)\bkind\s*(\d)\b`)

// jeKind berechnet einen Sonderausgabenabzug mit Höchstbetrag je Kind.
// Positionen werden über „Kind N“ in Bezeichnung oder Notiz einem Kind
// zugeordnet; bei genau einem berechtigten Kind ist die Zuordnung eindeutig.
// Nicht zuordenbare Beträge bei mehreren Kindern werden so angesetzt, wie es
// bei jeder denkbaren Verteilung mindestens zulässig ist.
func (r *rechnung) jeKind(kat string, kinder []kindDaten, prozent *big.Rat, hoechst int64, thema string) (abz int64, her string) {
	berechtigt := map[int]bool{}
	for _, k := range kinder {
		berechtigt[k.nr] = true
	}
	proKind := map[int]int64{}
	var offen int64
	for _, p := range r.pos.liste(kat, "*") {
		nr := 0
		if m := kindMarke.FindStringSubmatch(p.Bezeichnung + " " + p.Notiz); m != nil {
			nr = int(m[1][0] - '0')
		}
		switch {
		case nr == 0 && len(kinder) == 1:
			proKind[kinder[0].nr] += p.BetragCent
		case nr == 0:
			offen += p.BetragCent
		case berechtigt[nr]:
			proKind[nr] += p.BetragCent
		default:
			r.hinweis("warnung", thema+" nicht angesetzt", fmt.Sprintf("%q (%s) ist Kind %d zugeordnet, für das die Voraussetzungen nicht angegeben sind.", p.Bezeichnung, euro(p.BetragCent), nr))
		}
	}
	f := func(x int64) int64 { return min(prozentAb(x, prozent), hoechst) }
	var teile []string
	for _, k := range kinder {
		abz += f(proKind[k.nr])
		teile = append(teile, fmt.Sprintf("Kind %d: %s %% von %s, höchstens %s = %s", k.nr, prozent.FloatString(0), euro(proKind[k.nr]), euro(hoechst), euro(f(proKind[k.nr]))))
	}
	if offen > 0 {
		mindest := int64(-1)
		for _, k := range kinder {
			v := abz - f(proKind[k.nr]) + f(proKind[k.nr]+offen)
			if mindest < 0 || v < mindest {
				mindest = v
			}
		}
		abz = mindest
		teile = append(teile, fmt.Sprintf("keinem Kind zugeordnet %s: mit der ungünstigsten Verteilung gerechnet", euro(offen)))
		r.hinweis("pruefen", thema+" je Kind", fmt.Sprintf("%s über %s ist keinem Kind zugeordnet. Der Höchstbetrag gilt je Kind; bitte „Kind 1“, „Kind 2“ … in Bezeichnung oder Notiz der Position angeben. Bis dahin ist der Mindestabzug angesetzt.", thema, euro(offen)))
	}
	return abz, strings.Join(teile, "; ") + "; abziehbar " + euro(abz)
}
