package erfassung

// Erfundene Beispieltexte (keine echten Personendaten).

var textLohnsteuer = []string{
	"Ausdruck der elektronischen Lohnsteuerbescheinigung für 2024",
	"Bescheinigungszeitraum 01.01. - 31.12.2024",
	"Arbeitgeber: Muster Maschinenbau GmbH, Beispielweg 1, 12345 Beispielstadt",
	"Steuerklasse 1  Kinderfreibetrag 0,0  eTIN MM12345678901",
	"1. Dauer des Dienstverhältnisses 01.01. - 31.12.",
	"3. Bruttoarbeitslohn einschl. sonstiger Bezüge ohne die",
	"nachfolgend in den Nummern 8, 9 und 10 gesondert ausgewiesenen Bezüge 48.250,00",
	"4. Einbehaltene Lohnsteuer von 3. 7.123,00",
	"5. Einbehaltener Solidaritätszuschlag von 3. 0,00",
	"6. Einbehaltene Kirchensteuer des Arbeitnehmers von 3. 570,00",
	"17. Steuerfreie Arbeitgeberleistungen für Fahrten zwischen Wohnung und Tätigkeitsstätte 312,50",
	"22. a) Arbeitgeberanteil zur gesetzlichen Rentenversicherung 4.487,25",
	"22. b) Arbeitgeberanteil zur Knappschaft 0,00",
	"23. a) Arbeitnehmeranteil zur gesetzlichen Rentenversicherung 4.487,25",
	"25. Arbeitnehmerbeiträge zur gesetzlichen Krankenversicherung 3.940,13",
	"26. Arbeitnehmerbeiträge zur sozialen Pflegeversicherung 820,50",
	"27. Arbeitnehmerbeiträge zur Arbeitslosenversicherung 627,25",
}

var textBescheid = []string{
	"Finanzamt Musterstadt",
	"Datum: 15.03.2025",
	"Einkommensteuerbescheid 2023",
	"Veranlagungszeitraum 2023",
	"Einkünfte aus nichtselbständiger Arbeit 46.800 EUR",
	"Einkünfte aus selbständiger Arbeit nach § 18 EStG",
	"3.250 EUR",
	"Werbungskosten 1.230 EUR",
	"Gesamtbetrag der Einkünfte 50.050 EUR",
	"Zu versteuerndes Einkommen 41.980 EUR",
	"Festgesetzte Einkommensteuer 7.512 EUR",
	"Solidaritätszuschlag 0 EUR",
	"Bereits entrichtete Vorauszahlungen 7.900,00 EUR",
	"Erstattung 388,00 EUR",
	"Rechtsbehelfsbelehrung: Gegen diesen Bescheid ist der Einspruch zulässig.",
}

var textHandwerker = []string{
	"Malerbetrieb Beispiel GmbH",
	"Musterstr. 5, 12345 Beispielstadt",
	"Rechnung Nr. 2024-0815",
	"Rechnungsdatum: 12. März 2024",
	"Malerarbeiten im Wohnzimmer (Handwerkerleistung)",
	"Material netto 210,00 EUR",
	"Arbeitskosten brutto 595,00 EUR",
	"Summe netto 676,47 EUR",
	"19% MwSt 128,53 EUR",
	"Gesamtbetrag 805,00 EUR",
}

var textSpende = []string{
	"Tierschutzverein Beispielstadt e.V.",
	"Bestätigung über Zuwendungen im Sinne des § 10b EStG",
	"Zuwendungsbestätigung",
	"Datum der Ausstellung: 20.01.2025",
	"Betrag der Zuwendung: 150,00 EUR",
	"Der Verein ist wegen Förderung des Tierschutzes gemeinnützig.",
}

var textRechnung = []string{
	"Elektro Fischer GmbH",
	"Rechnung",
	"Rechnungsnummer: R-4711",
	"Rechnungsdatum: 03.05.2024",
	"1 x Laptop Notebook 14 Zoll 1.190,00 EUR",
	"Zwischensumme netto 1.000,00 EUR",
	"zzgl. 19% MwSt 190,00 EUR",
	"Gesamtbetrag: 1.190,00 EUR",
}
