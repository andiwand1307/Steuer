package rechner

import "math/big"

// TarifEuro berechnet die tarifliche Einkommensteuer nach § 32a Abs. 1 EStG
// (Grundtarif) für ein zu versteuerndes Einkommen in vollen Euro. Die
// Rechnung erfolgt exakt mit Brüchen; das Ergebnis wird auf volle Euro
// abgerundet (§ 32a Abs. 1 Satz 6 EStG).
func (rw *Regelwerk) TarifEuro(zvEEuro int64) int64 {
	if zvEEuro <= 0 {
		return 0
	}
	z := rw.zonen[0]
	for _, k := range rw.zonen {
		if zvEEuro >= k.ab {
			z = k
		}
	}
	t := big.NewRat(zvEEuro-z.basis, z.teiler)
	r := new(big.Rat).Mul(z.a, t)
	r.Add(r, z.b)
	r.Mul(r, t)
	r.Add(r, z.c)
	est := floorRat(r)
	if est < 0 {
		return 0
	}
	return est
}

// SplittingEuro berechnet die Einkommensteuer nach dem Splittingverfahren
// (§ 32a Abs. 5 EStG): halbes zvE auf volle Euro abgerundet, Tarif,
// verdoppelt.
func (rw *Regelwerk) SplittingEuro(zvEEuro int64) int64 {
	if zvEEuro <= 0 {
		return 0
	}
	return 2 * rw.TarifEuro(zvEEuro/2)
}

// zvEEuro rundet ein zu versteuerndes Einkommen in Cent auf volle Euro ab.
func zvEEuro(cent int64) int64 {
	if cent <= 0 {
		return 0
	}
	return cent / 100
}

// SoliCent berechnet den Solidaritätszuschlag in Cent aus der
// Bemessungsgrundlage in Cent (§§ 3, 4 SolZG, Zusammenveranlagung).
// Bruchteile eines Cents werden abgerundet.
func (rw *Regelwerk) SoliCent(bemessungCent int64) int64 {
	freigrenze := rw.cent("soli.freigrenze_zusammen")
	if bemessungCent <= freigrenze {
		return 0
	}
	voll := prozentAb(bemessungCent, rw.rat("soli.satz"))
	mild := prozentAb(bemessungCent-freigrenze, rw.rat("soli.milderung"))
	return min(voll, mild)
}
