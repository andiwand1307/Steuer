package rechner

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"steuer/regelwerk"
)

// Sicherheitsstufen wie in der Recherche.
const (
	Sicher       = "sicher"
	StarkEvident = "stark evident"
	Unsicher     = "unsicher"
)

// Regelwert ist ein einzelner Steuerwert mit Herkunft.
type Regelwert struct {
	Wert            string `json:"wert"`
	Einheit         string `json:"einheit"`
	Bezeichnung     string `json:"bezeichnung"`
	Rechtsgrundlage string `json:"rechtsgrundlage"`
	Quelle          string `json:"quelle"`
	Sicherheit      string `json:"sicherheit"`
}

// Tarifzone: ESt = (a·t + b)·t + c mit t = (zvE − Basis) / Teiler.
type Tarifzone struct {
	AbEuro    int64  `json:"ab_euro"`
	BasisEuro int64  `json:"basis_euro"`
	Teiler    int64  `json:"teiler"`
	A         string `json:"a"`
	B         string `json:"b"`
	C         string `json:"c"`
}

// Regelwerk sind alle Steuerwerte eines Jahres.
type Regelwerk struct {
	Jahr         int    `json:"jahr"`
	Stand        string `json:"stand"`
	Herkunft     string `json:"herkunft"`
	Formularjahr int    `json:"formularjahr"`
	Tarif        struct {
		Bezeichnung     string      `json:"bezeichnung"`
		Rechtsgrundlage string      `json:"rechtsgrundlage"`
		Quelle          string      `json:"quelle"`
		Sicherheit      string      `json:"sicherheit"`
		Zonen           []Tarifzone `json:"zonen"`
	} `json:"tarif"`
	Werte map[string]Regelwert `json:"werte"`

	zonen []tarifzoneRat
}

type tarifzoneRat struct {
	ab, basis, teiler int64
	a, b, c           *big.Rat
}

// LadeRegelwerk liest die eingebetteten Steuerwerte eines Jahres und prüft sie.
func LadeRegelwerk(jahr int) (*Regelwerk, error) {
	daten, err := regelwerk.Jahr(jahr)
	if err != nil {
		return nil, err
	}
	var rw Regelwerk
	if err := json.Unmarshal(daten, &rw); err != nil {
		return nil, fmt.Errorf("regelwerk %d: %w", jahr, err)
	}
	if rw.Jahr != jahr {
		return nil, fmt.Errorf("regelwerk %d: Datei enthält Jahr %d", jahr, rw.Jahr)
	}
	for i, z := range rw.Tarif.Zonen {
		zr := tarifzoneRat{ab: z.AbEuro, basis: z.BasisEuro, teiler: z.Teiler}
		if z.Teiler <= 0 {
			return nil, fmt.Errorf("regelwerk %d: Tarifzone %d ohne Teiler", jahr, i+1)
		}
		for _, p := range []struct {
			s    string
			ziel **big.Rat
		}{{z.A, &zr.a}, {z.B, &zr.b}, {z.C, &zr.c}} {
			r, ok := new(big.Rat).SetString(p.s)
			if !ok {
				return nil, fmt.Errorf("regelwerk %d: Tarifzone %d Koeffizient %q ungültig", jahr, i+1, p.s)
			}
			*p.ziel = r
		}
		if i > 0 && z.AbEuro <= rw.Tarif.Zonen[i-1].AbEuro {
			return nil, fmt.Errorf("regelwerk %d: Tarifzonen nicht aufsteigend", jahr)
		}
		rw.zonen = append(rw.zonen, zr)
	}
	if len(rw.zonen) == 0 || rw.zonen[0].ab != 0 {
		return nil, fmt.Errorf("regelwerk %d: Tarifzonen müssen bei 0 beginnen", jahr)
	}
	for k, w := range rw.Werte {
		switch w.Sicherheit {
		case Sicher, StarkEvident, Unsicher:
		default:
			return nil, fmt.Errorf("regelwerk %d: %s hat unbekannte Sicherheit %q", jahr, k, w.Sicherheit)
		}
		if w.Rechtsgrundlage == "" || w.Quelle == "" {
			return nil, fmt.Errorf("regelwerk %d: %s ohne Rechtsgrundlage oder Quelle", jahr, k)
		}
	}
	return &rw, nil
}

// wert liefert einen Regelwert; fehlende Schlüssel sind Programmfehler.
func (rw *Regelwerk) wert(schluessel string) Regelwert {
	w, ok := rw.Werte[schluessel]
	if !ok {
		panic("rechner: Regelwert fehlt: " + schluessel)
	}
	return w
}

// cent liefert einen Eurowert des Regelwerks in Cent (exakt).
func (rw *Regelwerk) cent(schluessel string) int64 {
	r := rw.rat(schluessel)
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() {
		panic("rechner: Regelwert ist kein ganzer Centbetrag: " + schluessel)
	}
	return r.Num().Int64()
}

// rat liefert einen Zahlenwert exakt als Bruch.
func (rw *Regelwerk) rat(schluessel string) *big.Rat {
	w := rw.wert(schluessel)
	r, ok := new(big.Rat).SetString(w.Wert)
	if !ok {
		panic("rechner: Regelwert ist keine Zahl: " + schluessel)
	}
	return r
}

// ganz liefert eine ganze Zahl (z.B. Prozentsatz, Tage).
func (rw *Regelwerk) ganz(schluessel string) int64 {
	r := rw.rat(schluessel)
	if !r.IsInt() {
		panic("rechner: Regelwert ist keine ganze Zahl: " + schluessel)
	}
	return r.Num().Int64()
}

// stufen liefert Prozentsätze der Form "2/3/4".
func (rw *Regelwerk) stufen(schluessel string) [3]int64 {
	teile := strings.Split(rw.wert(schluessel).Wert, "/")
	var out [3]int64
	if len(teile) != 3 {
		panic("rechner: Regelwert ist keine Stufenangabe: " + schluessel)
	}
	for i, t := range teile {
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			panic("rechner: Regelwert ist keine Stufenangabe: " + schluessel)
		}
		out[i] = n
	}
	return out
}

func (rw *Regelwerk) rg(schluessel string) string { return rw.wert(schluessel).Rechtsgrundlage }
func (rw *Regelwerk) si(schluessel string) string { return rw.wert(schluessel).Sicherheit }

// anteil berechnet betrag · zaehler / nenner, abgerundet (für positive Werte).
func anteil(betrag, zaehler, nenner int64) int64 {
	if nenner == 0 {
		return 0
	}
	r := new(big.Int).Mul(big.NewInt(betrag), big.NewInt(zaehler))
	q := new(big.Int)
	m := new(big.Int)
	q.DivMod(r, big.NewInt(nenner), m) // euklidisch: rundet Richtung −∞ bei positivem Nenner
	return q.Int64()
}

// anteilAuf berechnet betrag · zaehler / nenner, aufgerundet.
func anteilAuf(betrag, zaehler, nenner int64) int64 {
	return -anteil(-betrag, zaehler, nenner)
}

// prozentAb liefert p % von betrag, abgerundet auf Cent.
func prozentAb(betrag int64, p *big.Rat) int64 {
	r := new(big.Rat).Mul(new(big.Rat).SetInt64(betrag), p)
	r.Quo(r, big.NewRat(100, 1))
	return floorRat(r)
}

func floorRat(r *big.Rat) int64 {
	q := new(big.Int)
	m := new(big.Int)
	q.DivMod(r.Num(), r.Denom(), m)
	return q.Int64()
}

// schwaechste liefert die niedrigste Sicherheitsstufe.
func schwaechste(stufen ...string) string {
	rang := func(s string) int {
		switch s {
		case Sicher:
			return 2
		case StarkEvident:
			return 1
		}
		return 0
	}
	ergebnis := Sicher
	for _, s := range stufen {
		if s == "" {
			continue
		}
		if rang(s) < rang(ergebnis) {
			ergebnis = s
		}
	}
	return ergebnis
}
