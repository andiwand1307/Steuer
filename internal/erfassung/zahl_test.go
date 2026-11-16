package erfassung

import "testing"

func TestParseBetrag(t *testing.T) {
	tests := []struct {
		in     string
		cent   int64
		sicher bool
		ok     bool
	}{
		{"1.234,56", 123456, true, true},
		{"1.234,56 €", 123456, true, true},
		{"€ 1.234,56", 123456, true, true},
		{"EUR 12,5", 1250, true, true},
		{"12,50 EUR", 1250, true, true},
		{"1234,56", 123456, true, true},
		{"0,99", 99, true, true},
		{"1.234.567,89", 123456789, true, true},
		{"1,234.56", 123456, true, true},
		{"1234.56", 123456, true, true},
		{"12.5", 1250, true, true},
		{"-12,00", -1200, true, true},
		{"12,00-", -1200, true, true},
		{"−45,10", -4510, true, true},
		{"+7,00", 700, true, true},
		{"1 234,56", 123456, true, true},
		{"1 234,56", 123456, true, true},
		{"45.000", 4500000, false, true}, // mehrdeutig
		{"1,234", 123400, false, true},   // mehrdeutig
		{"1.234.567", 123456700, true, true},
		{"500", 50000, true, true},
		{"abc", 0, false, false},
		{"", 0, false, false},
		{"01.01.2024", 0, false, false},
		{"12,345,6", 0, false, false},
		{"1.23.456,00", 0, false, false},
		{"12,3456", 0, false, false},
	}
	for _, tc := range tests {
		c, s, ok := ParseBetrag(tc.in)
		if ok != tc.ok || (ok && (c != tc.cent || s != tc.sicher)) {
			t.Errorf("ParseBetrag(%q) = %d,%v,%v; erwartet %d,%v,%v", tc.in, c, s, ok, tc.cent, tc.sicher, tc.ok)
		}
	}
}

func TestZahlenInZeile(t *testing.T) {
	z := zahlenInZeile("4. Einbehaltene Lohnsteuer von 3. 7.123,00", false)
	if len(z) != 1 || z[0].Cent != 712300 {
		t.Errorf("Lohnsteuerzeile: %+v", z)
	}
	if z := zahlenInZeile("Rechnung vom 12.03.2024 über 119,00 €", false); len(z) != 1 || z[0].Cent != 11900 {
		t.Errorf("Datum nicht ausgeschlossen: %+v", z)
	}
	if z := zahlenInZeile("19% MwSt 19,00 EUR", false); len(z) != 1 || z[0].Cent != 1900 {
		t.Errorf("Prozent: %+v", z)
	}
	if z := zahlenInZeile("Einkommensteuer 2023: 7.512", true); len(z) != 1 || z[0].Cent != 751200 {
		t.Errorf("Jahreszahl: %+v", z)
	}
	if z := zahlenInZeile("Zu versteuern nach § 18 EStG 4 EUR", true); len(z) != 1 || z[0].Cent != 400 {
		t.Errorf("Paragraf: %+v", z)
	}
	if z := zahlenInZeile("Steuerklasse 3", false); len(z) != 0 {
		t.Errorf("Ganzzahl ohne Währung: %+v", z)
	}
	if z := zahlenInZeile("Position 22a", true); len(z) != 0 {
		t.Errorf("22a: %+v", z)
	}
}

func TestParseDatum(t *testing.T) {
	tests := map[string]string{
		"12.03.2024":        "2024-03-12",
		"1.3.2024":          "2024-03-01",
		"01.03.24":          "2024-03-01",
		"31.12.99":          "1999-12-31",
		"2024-03-12":        "2024-03-12",
		"12/03/2024":        "2024-03-12",
		"12. März 2024":     "2024-03-12",
		"3 Mai 2023":        "2023-05-03",
		"5. Dez. 2022":      "2022-12-05",
		"15. Januar 2025":   "2025-01-15",
		"Datum: 07.08.2021": "2021-08-07",
		"10 Okt 2020":       "2020-10-10",
	}
	for in, want := range tests {
		got, ok := ParseDatum(in)
		if !ok || got != want {
			t.Errorf("ParseDatum(%q) = %q,%v; erwartet %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"31.02.2024", "32.01.2024", "13.13.2024", "keine Angabe", "12.03.", "2024-13-01", "01.01.1850"} {
		if got, ok := ParseDatum(in); ok {
			t.Errorf("ParseDatum(%q) = %q; erwartet ungültig", in, got)
		}
	}
	if d := findeDaten("01.01. - 31.12.2024"); len(d) != 1 || d[0].ISO != "2024-12-31" {
		t.Errorf("Zeitraum: %+v", d)
	}
}
