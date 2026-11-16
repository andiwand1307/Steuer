package rechner

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"steuer/internal/modell"
)

// personDaten sind die Angaben je Ehegatte.
type personDaten struct {
	buerotageVor, buerotageNach  int64
	kmVor, kmNach                int64
	kmVorRoh, kmNachRoh          *big.Rat
	pkw                          bool
	oepnvKosten                  int64
	hoVor, hoUmzugsmonat, hoNach int64
	andererArbeitsplatz          bool
	hoMitFahrt                   int64
	vpf8, vpfAnAb, vpf24         int64
	vpfKuerzung, vpfErsatz       int64
	kvZuschuss, krankengeld      bool
}

type kindDaten struct {
	nr       int
	geburt   time.Time
	monate   int64
	haushalt bool
}

// fall bündelt alle gelesenen Angaben.
type fall struct {
	jahr   int
	person map[string]*personDaten

	hatUmzug       bool
	umzug          time.Time
	umzugMonat     int
	umzugBeruflich bool

	azVorhanden, azDurchgang, azAusschliesslich bool
	azWohnflaeche, azFlaeche                    *big.Rat
	azPerson                                    string
	mpAnstellung, mpNeben                       string
	wirkungskreis, nachweise, alteWohnung       bool
	anteilNeben                                 *big.Rat
	azAnsatz, zuordnung                         string

	sPerson, sArt    string
	kleinunternehmer bool

	vzESt, vzSoli int64
	kinder        []kindDaten
}

func leseFall(a *angaben, jahr int) *fall {
	f := &fall{jahr: jahr, person: map[string]*personDaten{}}
	for _, p := range []string{personA, personB} {
		x := praefix(p)
		d := &personDaten{
			buerotageVor:        a.ganzzahl(x + "buerotage_vor"),
			buerotageNach:       a.ganzzahl(x + "buerotage_nach"),
			kmVorRoh:            a.zahl(x + "km_vor"),
			kmNachRoh:           a.zahl(x + "km_nach"),
			kmVor:               a.volleKm(x + "km_vor"),
			kmNach:              a.volleKm(x + "km_nach"),
			pkw:                 a.auswahl(x+"verkehrsmittel", optPkw) == optPkw,
			oepnvKosten:         a.betrag(x + "oepnv_kosten"),
			hoVor:               a.ganzzahl(x + "hotage_vor"),
			hoUmzugsmonat:       a.ganzzahl(x + "hotage_umzugsmonat"),
			hoNach:              a.ganzzahl(x + "hotage_nach"),
			andererArbeitsplatz: a.jaNein(x+"anderer_arbeitsplatz", true),
			hoMitFahrt:          a.ganzzahl(x + "hotage_mit_fahrt"),
			vpf8:                a.ganzzahl(x + "verpflegung_8h"),
			vpfAnAb:             a.ganzzahl(x + "verpflegung_anabreise"),
			vpf24:               a.ganzzahl(x + "verpflegung_24h"),
			vpfKuerzung:         a.betrag(x + "verpflegung_kuerzung"),
			vpfErsatz:           a.betrag(x + "verpflegung_ersatz"),
			kvZuschuss:          a.jaNein(x+"kv_zuschuss", true),
			krankengeld:         a.jaNein(x+"krankengeld", true),
		}
		if d.hoUmzugsmonat > d.hoVor {
			a.fehl(x+"hotage_umzugsmonat", "darf nicht größer sein als die Homeoffice-Tage vor dem Umzug")
		}
		if d.buerotageVor+d.buerotageNach+d.hoVor+d.hoNach > 366 {
			a.fehl(x+"buerotage_vor", "Büro- und Homeoffice-Tage zusammen über 366")
		}
		f.person[p] = d
	}

	if t, ok := a.datum("umzug.datum"); ok {
		if t.Year() != jahr {
			a.fehl("umzug.datum", fmt.Sprintf("muss in %d liegen (sonst leer lassen)", jahr))
		} else {
			f.hatUmzug, f.umzug, f.umzugMonat = true, t, int(t.Month())
		}
	}
	f.umzugBeruflich = a.jaNein("umzug.beruflich", false)

	f.azVorhanden = a.jaNein("az.vorhanden", false)
	f.azPerson = a.auswahl("az.person", personA)
	f.azDurchgang = a.jaNein("az.durchgangszimmer", false)
	f.azAusschliesslich = a.jaNein("az.ausschliesslich_beruflich", false)
	f.azWohnflaeche = a.zahl("az.wohnflaeche")
	f.azFlaeche = a.zahl("az.flaeche")
	if f.azFlaeche.Cmp(f.azWohnflaeche) > 0 {
		a.fehl("az.flaeche", "größer als die Wohnfläche")
	}
	f.mpAnstellung = a.auswahl("az.mittelpunkt_anstellung", optUnklar)
	f.mpNeben = a.auswahl("az.mittelpunkt_nebentaetigkeit", optUnklar)
	f.wirkungskreis = a.jaNein("az.einheitlicher_wirkungskreis", false)
	f.nachweise = a.jaNein("az.nachweise", false)
	f.anteilNeben = a.zahl("az.anteil_nebentaetigkeit")
	if f.anteilNeben.Cmp(big.NewRat(100, 1)) > 0 {
		a.fehl("az.anteil_nebentaetigkeit", "höchstens 100 %")
	}
	f.alteWohnung = a.jaNein("az.alte_wohnung", false)
	f.azAnsatz = a.auswahl("az.ansatz", optAutomatisch)
	f.zuordnung = a.auswahl("ho.zuordnung", optOptimiert)

	f.sPerson = a.auswahl("s.person", personA)
	f.sArt = a.auswahl("s.einkunftsart", optUnklar)
	f.kleinunternehmer = a.jaNein("s.kleinunternehmer", true)

	f.vzESt = a.betrag("allg.vorauszahlung_est")
	f.vzSoli = a.betrag("allg.vorauszahlung_soli")

	for i := 1; i <= maxKinder; i++ {
		x := fmt.Sprintf("kind%d.", i)
		g, ok := a.datum(x + "geburtsdatum")
		if !ok {
			if a.gesetzt(x+"monate") || a.gesetzt(x+"haushalt") {
				a.fehl(x+"geburtsdatum", "fehlt, obwohl weitere Angaben zum Kind gemacht sind")
			}
			continue
		}
		if g.Year() > jahr {
			a.fehl(x+"geburtsdatum", "liegt nach dem Veranlagungszeitraum")
			continue
		}
		k := kindDaten{nr: i, geburt: g, haushalt: a.jaNein(x+"haushalt", true), monate: -1}
		if a.gesetzt(x + "monate") {
			k.monate = a.ganzzahl(x + "monate")
			if k.monate > 12 {
				a.fehl(x+"monate", "höchstens 12")
			}
		}
		f.kinder = append(f.kinder, k)
	}
	return f
}

// posSammlung hält die bestätigten Positionen nach Kategorie.
type posSammlung struct {
	nachKat map[string][]modell.Position
}

// summe liefert die Summe einer Kategorie; person "*" = alle Positionen.
func (s posSammlung) summe(kat, person string) int64 {
	var n int64
	for _, p := range s.nachKat[kat] {
		if person == "*" || p.Person == person {
			n += p.BetragCent
		}
	}
	return n
}

func (s posSammlung) liste(kat, person string) []modell.Position {
	var out []modell.Position
	for _, p := range s.nachKat[kat] {
		if person == "*" || p.Person == person {
			out = append(out, p)
		}
	}
	return out
}

func (s posSammlung) vorhanden(praefix, person string) bool {
	for k, l := range s.nachKat {
		if strings.HasPrefix(k, praefix) {
			for _, p := range l {
				if person == "*" || p.Person == person {
					return true
				}
			}
		}
	}
	return false
}

// sammle filtert die Positionen: nur bestätigte, bekannte, dieses Jahres.
func sammle(positionen []modell.Position, jahr int) (posSammlung, []Hinweis) {
	s := posSammlung{nachKat: map[string][]modell.Position{}}
	var hinweise []Hinweis
	var offenAnzahl int
	var offenSumme int64
	for _, p := range positionen {
		p.Person = strings.ToUpper(strings.TrimSpace(p.Person))
		switch {
		case p.Steuerjahr != 0 && p.Steuerjahr != jahr:
			hinweise = append(hinweise, Hinweis{"warnung", "Position aus anderem Jahr", fmt.Sprintf("Position %d (%s, %s) gehört zum Jahr %d und wird nicht gerechnet.", p.ID, p.Kategorie, euro(p.BetragCent), p.Steuerjahr)})
		case !p.Bestaetigt:
			offenAnzahl++
			offenSumme += p.BetragCent
		case !modell.KategorieBekannt(p.Kategorie):
			hinweise = append(hinweise, Hinweis{"warnung", "Unbekannte Kategorie", fmt.Sprintf("Position %d hat die unbekannte Kategorie %q und wird nicht gerechnet.", p.ID, p.Kategorie)})
		case p.BetragCent < 0:
			hinweise = append(hinweise, Hinweis{"warnung", "Negativer Betrag", fmt.Sprintf("Position %d (%s) hat einen negativen Betrag und wird nicht gerechnet. Erstattungen bitte mit der zugehörigen Angabe erfassen.", p.ID, p.Kategorie)})
		case (strings.HasPrefix(p.Kategorie, "lstb.") || strings.HasPrefix(p.Kategorie, "n.")) && p.Person != personA && p.Person != personB:
			hinweise = append(hinweise, Hinweis{"warnung", "Person fehlt", fmt.Sprintf("Position %d (%s, %s) ist keinem Ehegatten (A/B) zugeordnet und wird nicht gerechnet.", p.ID, p.Kategorie, euro(p.BetragCent))})
		default:
			s.nachKat[p.Kategorie] = append(s.nachKat[p.Kategorie], p)
		}
	}
	if offenAnzahl > 0 {
		hinweise = append(hinweise, Hinweis{"info", "Unbestätigte Positionen", fmt.Sprintf("%d unbestätigte Position(en) über zusammen %s sind nicht gerechnet. Bitte prüfen und bestätigen.", offenAnzahl, euro(offenSumme))})
	}
	return s, hinweise
}

// euro formatiert Cent als "1.234,56 €".
func euro(cent int64) string {
	vz := ""
	if cent < 0 {
		vz = "−"
		cent = -cent
	}
	e := itoa(cent / 100)
	var b strings.Builder
	for i, c := range e {
		if i > 0 && (len(e)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return fmt.Sprintf("%s%s,%02d €", vz, b.String(), cent%100)
}
