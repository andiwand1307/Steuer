package modell

// Kategorie ordnet eine Position steuerlich ein. Die Zuordnung zu
// Formularzeilen liegt im Paket rechner.
type Kategorie struct {
	Schluessel string `json:"schluessel"`
	Titel      string `json:"titel"`
	Gruppe     string `json:"gruppe"`
}

// Kategorien ist die verbindliche Liste aller Positionsarten.
var Kategorien = []Kategorie{
	// Lohnsteuerbescheinigung (Nummern wie auf der Bescheinigung)
	{"lstb.3", "Bruttoarbeitslohn (Nr. 3)", "Lohnsteuerbescheinigung"},
	{"lstb.4", "Einbehaltene Lohnsteuer (Nr. 4)", "Lohnsteuerbescheinigung"},
	{"lstb.5", "Einbehaltener Solidaritätszuschlag (Nr. 5)", "Lohnsteuerbescheinigung"},
	{"lstb.17", "Steuerfreie Arbeitgeberleistungen für Fahrten (Nr. 17)", "Lohnsteuerbescheinigung"},
	{"lstb.22a", "Arbeitgeberanteil Rentenversicherung (Nr. 22a)", "Lohnsteuerbescheinigung"},
	{"lstb.23a", "Arbeitnehmeranteil Rentenversicherung (Nr. 23a)", "Lohnsteuerbescheinigung"},
	{"lstb.25", "Arbeitnehmerbeitrag Krankenversicherung (Nr. 25)", "Lohnsteuerbescheinigung"},
	{"lstb.26", "Arbeitnehmerbeitrag Pflegeversicherung (Nr. 26)", "Lohnsteuerbescheinigung"},
	{"lstb.27", "Arbeitnehmerbeitrag Arbeitslosenversicherung (Nr. 27)", "Lohnsteuerbescheinigung"},
	// Werbungskosten Arbeitnehmer (Anlage N)
	{"n.arbeitsmittel", "Arbeitsmittel", "Werbungskosten Anstellung"},
	{"n.fortbildung", "Fortbildung", "Werbungskosten Anstellung"},
	{"n.reisekosten", "Reisekosten und Dienstreisen", "Werbungskosten Anstellung"},
	{"n.telefon_internet", "Telefon und Internet", "Werbungskosten Anstellung"},
	{"n.gewerkschaft", "Gewerkschaft und Berufsverbände", "Werbungskosten Anstellung"},
	{"n.umzug", "Umzugskosten", "Werbungskosten Anstellung"},
	{"n.sonstige", "Sonstige Werbungskosten", "Werbungskosten Anstellung"},
	// Selbständige Nebentätigkeit (Anlage S / EÜR)
	{"s.einnahmen", "Betriebseinnahmen (Honorare)", "Selbständige Tätigkeit"},
	{"s.fahrtkosten", "Fahrt- und Reisekosten", "Selbständige Tätigkeit"},
	{"s.arbeitsmittel", "Arbeitsmittel und Ausstattung", "Selbständige Tätigkeit"},
	{"s.fortbildung", "Fortbildung und Fachliteratur", "Selbständige Tätigkeit"},
	{"s.telefon_internet", "Telefon und Internet", "Selbständige Tätigkeit"},
	{"s.bewirtung", "Bewirtung", "Selbständige Tätigkeit"},
	{"s.sonstige", "Sonstige Betriebsausgaben", "Selbständige Tätigkeit"},
	// Wohnungskosten als Grundlage für das Arbeitszimmer
	{"wohnung.miete", "Miete (kalt)", "Wohnung und Arbeitszimmer"},
	{"wohnung.nebenkosten", "Nebenkosten und Heizung", "Wohnung und Arbeitszimmer"},
	{"wohnung.strom", "Strom", "Wohnung und Arbeitszimmer"},
	{"wohnung.versicherung", "Hausratversicherung", "Wohnung und Arbeitszimmer"},
	{"wohnung.zimmer_direkt", "Kosten nur für das Arbeitszimmer (z.B. Renovierung)", "Wohnung und Arbeitszimmer"},
	// Vorsorge und Sonderausgaben
	{"vor.kranken_pflege", "Kranken- und Pflegeversicherung (Basis)", "Vorsorge"},
	{"vor.altersvorsorge", "Altersvorsorge (Rürup, Versorgungswerk)", "Vorsorge"},
	{"vor.sonstige", "Sonstige Versicherungen (Haftpflicht, Unfall, Berufsunfähigkeit)", "Vorsorge"},
	{"sa.spenden", "Spenden und Mitgliedsbeiträge", "Sonderausgaben"},
	{"sa.parteispenden", "Parteispenden", "Sonderausgaben"},
	{"kind.betreuung", "Kinderbetreuungskosten", "Kind"},
	{"kind.schulgeld", "Schulgeld", "Kind"},
	// Außergewöhnliche Belastungen
	{"agb.krankheit", "Krankheitskosten", "Außergewöhnliche Belastungen"},
	{"agb.sonstige", "Sonstige außergewöhnliche Belastungen", "Außergewöhnliche Belastungen"},
	// Haushaltsnahe Aufwendungen (§ 35a)
	{"h.minijob", "Haushaltsnahe Minijobs", "Haushaltsnahe Aufwendungen"},
	{"h.dienstleistung", "Haushaltsnahe Dienstleistungen", "Haushaltsnahe Aufwendungen"},
	{"h.handwerker", "Handwerkerleistungen (nur Arbeitskosten)", "Haushaltsnahe Aufwendungen"},
}

// KategorieBekannt meldet, ob ein Schlüssel in der Liste steht.
func KategorieBekannt(schluessel string) bool {
	for _, k := range Kategorien {
		if k.Schluessel == schluessel {
			return true
		}
	}
	return false
}
