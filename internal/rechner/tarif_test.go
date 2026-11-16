package rechner

import "testing"

func regelwerk2026(t *testing.T) *Regelwerk {
	t.Helper()
	rw, err := LadeRegelwerk(2026)
	if err != nil {
		t.Fatal(err)
	}
	return rw
}

// Stützstellen von Hand aus § 32a Abs. 1 EStG (Fassung ab VZ 2026)
// nachgerechnet; Zwischenergebnisse im Kommentar.
func TestTarifStuetzstellen(t *testing.T) {
	rw := regelwerk2026(t)
	faelle := []struct {
		zvE, est int64
		rechnung string
	}{
		{0, 0, "Zone 1"},
		{12348, 0, "Zone 1, Grundfreibetrag"},
		{12349, 0, "Zone 2: y=0,0001; (914,51·y+1400)·y = 0,14 → 0"},
		{12350, 0, "Zone 2: y=0,0002 → 0,28 → 0"},
		{15000, 435, "Zone 2: y=0,2652; 914,51·y=242,528; +1400=1642,528; ·y=435,598 → 435"},
		{17799, 1034, "Zone 2 Ende: y=0,5451; 914,51·y=498,499; +1400=1898,499; ·y=1034,872 → 1034"},
		{17800, 1035, "Zone 3 Anfang: z=0,0001; (173,10·z+2397)·z=0,2397; +1034,87=1035,11 → 1035"},
		{25000, 2850, "Zone 3: z=0,7201; 173,10·z=124,649; +2397=2521,649; ·z=1815,84; +1034,87=2850,71 → 2850"},
		{30000, 4217, "Zone 3: z=1,2201; 173,10·z=211,199; +2397=2608,199; ·z=3182,26; +1034,87=4217,13 → 4217"},
		{50000, 10548, "Zone 3: z=3,2201; 173,10·z=557,399; +2397=2954,399; ·z=9513,46; +1034,87=10548,33 → 10548"},
		{69878, 18213, "Zone 3 Ende: z=5,2079; 173,10·z=901,487; +2397=3298,487; ·z=17178,18; +1034,87=18213,05 → 18213"},
		{69879, 18213, "Zone 4 Anfang: 0,42·69879=29349,18; −11135,63=18213,55 → 18213"},
		{100000, 30864, "Zone 4: 42000 − 11135,63 = 30864,37 → 30864"},
		{277825, 105550, "Zone 4 Ende: 116686,50 − 11135,63 = 105550,87 → 105550"},
		{277826, 105551, "Zone 5 Anfang: 0,45·277826=125021,70; −19470,38=105551,32 → 105551"},
		{300000, 115529, "Zone 5: 135000 − 19470,38 = 115529,62 → 115529"},
	}
	for _, f := range faelle {
		if got := rw.TarifEuro(f.zvE); got != f.est {
			t.Errorf("Tarif(%d) = %d, erwartet %d (%s)", f.zvE, got, f.est, f.rechnung)
		}
	}
}

func TestTarifStetigAnZonengrenzen(t *testing.T) {
	rw := regelwerk2026(t)
	for _, g := range []int64{12348, 17799, 69878, 277825} {
		a, b := rw.TarifEuro(g), rw.TarifEuro(g+1)
		if b < a || b-a > 1 {
			t.Errorf("Sprung an Zonengrenze %d: %d → %d", g, a, b)
		}
	}
}

func TestSplitting(t *testing.T) {
	rw := regelwerk2026(t)
	faelle := []struct{ zvE, est int64 }{
		{24696, 0},       // 2 × Tarif(12348)
		{24698, 0},       // 2 × Tarif(12349) = 2 × 0
		{60000, 8434},    // 2 × 4217
		{60001, 8434},    // Hälfte 30000,5 → 30000
		{80000, 14418},   // 2 × Tarif(40000)
		{150000, 40728},  // 2 × Tarif(75000) = 2 × (31500 − 11135,63 → 20364)
		{600000, 231058}, // 2 × Tarif(300000) = 2 × 115529
	}
	for _, f := range faelle {
		if got := rw.SplittingEuro(f.zvE); got != f.est {
			t.Errorf("Splitting(%d) = %d, erwartet %d", f.zvE, got, f.est)
		}
	}
}

func TestSoliMilderungszone(t *testing.T) {
	rw := regelwerk2026(t)
	faelle := []struct {
		bmgEuro, soliCent int64
		rechnung          string
	}{
		{40700, 0, "Freigrenze nicht überschritten"},
		{40701, 11, "min(5,5 % = 2238,55; 11,9 % × 1 = 0,119) → 0,11"},
		{45000, 51170, "min(2475,00; 11,9 % × 4300 = 511,70)"},
		{60000, 229670, "min(3300,00; 11,9 % × 19300 = 2296,70)"},
		{80000, 440000, "min(4400,00; 11,9 % × 39300 = 4676,70) → volle 5,5 %"},
	}
	for _, f := range faelle {
		if got := rw.SoliCent(f.bmgEuro * 100); got != f.soliCent {
			t.Errorf("Soli(%d) = %d Cent, erwartet %d (%s)", f.bmgEuro, got, f.soliCent, f.rechnung)
		}
	}
}

func TestRegelwerkVollstaendig(t *testing.T) {
	rw := regelwerk2026(t)
	for k, w := range rw.Werte {
		if w.Wert == "" || w.Bezeichnung == "" {
			t.Errorf("%s unvollständig", k)
		}
	}
	if _, err := LadeRegelwerk(2025); err == nil {
		t.Error("2025 darf nicht geladen werden (keine Werte hinterlegt)")
	}
}

func TestFeldkatalog(t *testing.T) {
	f := Feldkatalog()
	if len(f) < 100 {
		t.Fatalf("Feldkatalog zu klein: %d", len(f))
	}
	for _, z := range Zuordnungen() {
		if z.Formular == "" {
			t.Errorf("Kategorie %s ohne Formular", z.Kategorie)
		}
	}
}
