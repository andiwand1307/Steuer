package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"steuer/internal/modell"
	"steuer/internal/store"
)

type umgebung struct {
	t      *testing.T
	ordner string
	s      *store.Store
	h      http.Handler
}

type falscherDienst struct{ n int }

func (f *falscherDienst) Durchlauf() (int, []error) { f.n++; return 2, nil }

func neu(t *testing.T) *umgebung {
	t.Helper()
	ordner := t.TempDir()
	for _, u := range []string{"Eingang", "Belege/2026", "daten"} {
		if err := os.MkdirAll(filepath.Join(ordner, filepath.FromSlash(u)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	srv := &Server{Store: s, Dienst: &falscherDienst{}, Datenordner: ordner, StandardJahr: 2026}
	return &umgebung{t, ordner, s, srv.Handler()}
}

// anfrage sendet eine Anfrage wie der Browser (richtiger Host und Origin).
func (u *umgebung) anfrage(methode, pfad string, body any) *httptest.ResponseRecorder {
	u.t.Helper()
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(methode, pfad, bytes.NewReader(b))
	} else {
		r = httptest.NewRequest(methode, pfad, nil)
	}
	r.Host = "127.0.0.1:8741"
	if methode != "GET" {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://127.0.0.1:8741")
	}
	w := httptest.NewRecorder()
	u.h.ServeHTTP(w, r)
	return w
}

func dekodiere(t *testing.T, w *httptest.ResponseRecorder, ziel any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), ziel); err != nil {
		t.Fatalf("kein JSON: %v\n%s", err, w.Body.String())
	}
}

func TestHostPruefung(t *testing.T) {
	u := neu(t)
	for host, erwartet := range map[string]int{
		"127.0.0.1:8741": 200, "localhost:8741": 200, "LOCALHOST": 200, "[::1]:8741": 200,
		"evil.example.com": 403, "evil.example.com:8741": 403, "127.0.0.1.evil.com": 403, "192.168.1.5:8741": 403, "": 403,
	} {
		r := httptest.NewRequest("GET", "/api/meta", nil)
		r.Host = host
		w := httptest.NewRecorder()
		u.h.ServeHTTP(w, r)
		if w.Code != erwartet {
			t.Errorf("Host %q: %d, erwartet %d", host, w.Code, erwartet)
		}
	}
	// auch die statischen Dateien sind geschützt
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "evil.example.com"
	w := httptest.NewRecorder()
	u.h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("Startseite mit fremdem Host: %d", w.Code)
	}
}

func TestSchreibschutz(t *testing.T) {
	u := neu(t)
	body := `{"jahr":2026,"schluessel":"az.flaeche","wert":"12"}`
	cases := []struct {
		name, ct, origin string
		erwartet         int
	}{
		{"ohne Content-Type", "", "http://127.0.0.1:8741", 415},
		{"Formular-Typ", "application/x-www-form-urlencoded", "http://127.0.0.1:8741", 415},
		{"text/plain", "text/plain", "http://127.0.0.1:8741", 415},
		{"fremder Origin", "application/json", "http://evil.example.com", 403},
		{"fehlender Origin", "application/json", "", 403},
		{"anderer Port", "application/json", "http://127.0.0.1:9999", 403},
		{"richtig", "application/json; charset=utf-8", "http://127.0.0.1:8741", 200},
	}
	for _, c := range cases {
		r := httptest.NewRequest("PUT", "/api/angaben", strings.NewReader(body))
		r.Host = "127.0.0.1:8741"
		if c.ct != "" {
			r.Header.Set("Content-Type", c.ct)
		}
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		u.h.ServeHTTP(w, r)
		if w.Code != c.erwartet {
			t.Errorf("%s: %d, erwartet %d (%s)", c.name, w.Code, c.erwartet, w.Body.String())
		}
	}
}

func (u *umgebung) beleg(datei string) int64 {
	u.t.Helper()
	id, err := u.s.BelegAnlegen(modell.Beleg{Steuerjahr: 2026, Datei: datei, SHA256: datei, Art: "rechnung"})
	if err != nil {
		u.t.Fatal(err)
	}
	return id
}

func TestDateiAuslieferung(t *testing.T) {
	u := neu(t)
	os.WriteFile(filepath.Join(u.ordner, "Belege", "2026", "rechnung.pdf"), []byte("%PDF-1.4 test"), 0o644)
	os.WriteFile(filepath.Join(u.ordner, "Belege", "2026", "seite.html"), []byte("<script>alert(1)</script>"), 0o644)
	os.WriteFile(filepath.Join(u.ordner, "daten", "steuer.db"), []byte("geheim"), 0o644)
	// Datei außerhalb des Datenordners
	aussen := filepath.Join(filepath.Dir(u.ordner), "aussen-"+filepath.Base(u.ordner)+".txt")
	os.WriteFile(aussen, []byte("außen"), 0o644)
	t.Cleanup(func() { os.Remove(aussen) })

	ok := u.beleg("Belege/2026/rechnung.pdf")
	w := u.anfrage("GET", "/api/belege/"+itoa(int(ok))+"/datei", nil)
	if w.Code != 200 || w.Body.String() != "%PDF-1.4 test" || w.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("PDF: %d %q %q", w.Code, w.Body.String(), w.Header().Get("Content-Type"))
	}
	if !strings.HasPrefix(w.Header().Get("Content-Disposition"), "inline") {
		t.Errorf("PDF sollte inline sein: %q", w.Header().Get("Content-Disposition"))
	}

	html := u.beleg("Belege/2026/seite.html")
	w = u.anfrage("GET", "/api/belege/"+itoa(int(html))+"/datei", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("HTML darf nie inline ausgeliefert werden: %q %q", w.Header().Get("Content-Type"), w.Header().Get("Content-Disposition"))
	}

	verboten := []string{
		"../" + filepath.Base(aussen),
		"Belege/../../" + filepath.Base(aussen),
		"Belege/2026/../../../" + filepath.Base(aussen),
		"/etc/passwd",
		aussen,
		`..\` + filepath.Base(aussen),
		"daten/steuer.db",
		"Daten/steuer.db",
		"Belege/../daten/steuer.db",
		"Belege",
		"",
	}
	for _, p := range verboten {
		id := u.beleg(p)
		w := u.anfrage("GET", "/api/belege/"+itoa(int(id))+"/datei", nil)
		if w.Code != 403 && w.Code != 404 {
			t.Errorf("Pfad %q wurde nicht abgelehnt: %d", p, w.Code)
		}
		if strings.Contains(w.Body.String(), "außen") || strings.Contains(w.Body.String(), "geheim") || strings.Contains(w.Body.String(), "root:") {
			t.Errorf("Pfad %q lieferte Inhalt aus", p)
		}
	}

	// Symlink aus dem Datenordner hinaus
	if err := os.Symlink(aussen, filepath.Join(u.ordner, "Belege", "2026", "link.txt")); err == nil {
		id := u.beleg("Belege/2026/link.txt")
		w := u.anfrage("GET", "/api/belege/"+itoa(int(id))+"/datei", nil)
		if w.Code != 403 {
			t.Errorf("Symlink nach außen: %d", w.Code)
		}
	}
	// fehlende Datei
	id := u.beleg("Belege/2026/gibtsnicht.pdf")
	if w := u.anfrage("GET", "/api/belege/"+itoa(int(id))+"/datei", nil); w.Code != 404 {
		t.Errorf("fehlende Datei: %d", w.Code)
	}
	// unbekannter Beleg
	if w := u.anfrage("GET", "/api/belege/9999/datei", nil); w.Code != 404 {
		t.Errorf("unbekannter Beleg: %d", w.Code)
	}
}

func TestAngabenUndErklaerung(t *testing.T) {
	u := neu(t)
	w := u.anfrage("GET", "/api/angaben?jahr=2026", nil)
	var a struct {
		Defs  []angabeDefDTO    `json:"defs"`
		Werte map[string]string `json:"werte"`
	}
	dekodiere(t, w, &a)
	if len(a.Defs) < 20 {
		t.Fatalf("zu wenige Angaben: %d", len(a.Defs))
	}
	// Validierung
	for _, c := range []struct{ k, v string }{{"az.flaeche", "abc"}, {"az.vorhanden", "vielleicht"}, {"az.person", "C"}, {"umzug.datum", "31.02.2026"}, {"gibt.es.nicht", "1"}} {
		if w := u.anfrage("PUT", "/api/angaben", angabeEingabe{2026, c.k, c.v}); w.Code != 400 {
			t.Errorf("%s=%q: %d", c.k, c.v, w.Code)
		}
	}
	// Normalisierung
	w = u.anfrage("PUT", "/api/angaben", angabeEingabe{2026, "allg.vorauszahlung_est", "1.234,50 €"})
	var r struct{ Wert string }
	dekodiere(t, w, &r)
	if w.Code != 200 || r.Wert != "1234.50" {
		t.Fatalf("Betrag: %d %q", w.Code, r.Wert)
	}
	u.anfrage("PUT", "/api/angaben", angabeEingabe{2026, "umzug.datum", "15.03.2026"})
	if m, _ := u.s.Angaben(2026); m["umzug.datum"] != "2026-03-15" {
		t.Errorf("Datum: %v", m)
	}

	// Position anlegen: Lohn Person A
	for _, p := range []positionEingabe{
		{Steuerjahr: 2026, Person: "A", Kategorie: "lstb.3", BetragCent: 6000000, Bestaetigt: true},
		{Steuerjahr: 2026, Person: "A", Kategorie: "lstb.4", BetragCent: 900000, Bestaetigt: true},
	} {
		if w := u.anfrage("POST", "/api/positionen", p); w.Code != 201 {
			t.Fatalf("Position: %d %s", w.Code, w.Body.String())
		}
	}
	w = u.anfrage("GET", "/api/erklaerung?jahr=2026", nil)
	var e struct {
		Fehler    string
		Formulare []formularDTO `json:"formulare"`
		Hinweise  []hinweisDTO  `json:"hinweise"`
		ZvE       int64         `json:"zve_cent"`
	}
	dekodiere(t, w, &e)
	if e.Fehler != "" || len(e.Formulare) == 0 || e.ZvE <= 0 {
		t.Fatalf("Erklärung: %+v", e)
	}
	for i := 1; i < len(e.Hinweise); i++ {
		if stufeRang(e.Hinweise[i-1].Stufe) > stufeRang(e.Hinweise[i].Stufe) {
			t.Errorf("Hinweise nicht nach Stufe sortiert")
		}
	}
	// CSV
	w = u.anfrage("GET", "/api/erklaerung.csv?jahr=2026", nil)
	if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "\xef\xbb\xbfFormular;Zeile;") || !strings.Contains(w.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("CSV: %d %.80q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "60000,00") {
		t.Errorf("CSV ohne deutschen Betrag 60000,00")
	}

	// ungültige Eingabe wird als Text gemeldet, nicht als Serverfehler
	u.s.AngabeSetzen(modell.Angabe{Steuerjahr: 2026, Schluessel: "az.flaeche", Wert: "xyz"})
	w = u.anfrage("GET", "/api/erklaerung?jahr=2026", nil)
	dekodiere(t, w, &e)
	if w.Code != 200 || !strings.Contains(e.Fehler, "Eingaben fehlerhaft") {
		t.Errorf("Fehler nicht gemeldet: %d %q", w.Code, e.Fehler)
	}
	// nicht unterstütztes Jahr
	w = u.anfrage("GET", "/api/erklaerung?jahr=2019", nil)
	dekodiere(t, w, &e)
	if w.Code != 200 || e.Fehler == "" {
		t.Errorf("Jahr 2019: %d %q", w.Code, e.Fehler)
	}
	if w := u.anfrage("GET", "/api/erklaerung?jahr=abc", nil); w.Code != 400 {
		t.Errorf("jahr=abc: %d", w.Code)
	}
}

func TestPositionenUndBelege(t *testing.T) {
	u := neu(t)
	id := u.beleg("Belege/2026/x.pdf")
	pid, _ := u.s.PositionAnlegen(modell.Position{Steuerjahr: 2026, BelegID: &id, Kategorie: "n.arbeitsmittel", BetragCent: 5000, Vorschlag: true})

	// ungültige Positionen
	for _, p := range []positionEingabe{
		{Steuerjahr: 2026, Kategorie: "gibt.es.nicht", BetragCent: 1},
		{Steuerjahr: 2026, Kategorie: "n.arbeitsmittel", Person: "C"},
		{Steuerjahr: 2026, Kategorie: "n.arbeitsmittel", Datum: "morgen"},
		{Steuerjahr: 1, Kategorie: "n.arbeitsmittel"},
	} {
		if w := u.anfrage("POST", "/api/positionen", p); w.Code != 400 {
			t.Errorf("%+v: %d", p, w.Code)
		}
	}
	// Bestätigen und Ändern behält die Belegverknüpfung
	w := u.anfrage("PUT", "/api/positionen/"+itoa(int(pid)), positionEingabe{Steuerjahr: 2026, Person: "B", Kategorie: "n.fortbildung", Bezeichnung: "Kurs", BetragCent: 7500, Datum: "01.02.2026", Bestaetigt: true})
	if w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body.String())
	}
	p, _ := u.s.Position(pid)
	if p.BelegID == nil || *p.BelegID != id || !p.Bestaetigt || p.Vorschlag || p.Kategorie != "n.fortbildung" || p.Datum != "2026-02-01" || p.Person != "B" {
		t.Fatalf("Position: %+v", p)
	}
	if w := u.anfrage("PUT", "/api/positionen/4711", positionEingabe{Steuerjahr: 2026, Kategorie: "n.fortbildung"}); w.Code != 404 {
		t.Errorf("unbekannte Position: %d", w.Code)
	}

	// Beleg ändern
	w = u.anfrage("PUT", "/api/belege/"+itoa(int(id)), belegEingabe{Steuerjahr: 2026, Belegdatum: "05.03.2026", Aussteller: "Muster GmbH", Art: "rechnung", BetragCent: 7500, Status: "bestaetigt"})
	if w.Code != 200 {
		t.Fatalf("Beleg PUT: %d %s", w.Code, w.Body.String())
	}
	b, _ := u.s.Beleg(id)
	if b.Status != modell.StatusBestaetigt || b.Belegdatum != "2026-03-05" || b.Datei != "Belege/2026/x.pdf" {
		t.Fatalf("Beleg: %+v", b)
	}
	if w := u.anfrage("PUT", "/api/belege/"+itoa(int(id)), belegEingabe{Steuerjahr: 2026, Status: "kaputt"}); w.Code != 400 {
		t.Errorf("Status kaputt: %d", w.Code)
	}
	// Liste ohne Volltext
	u.s.BelegAendern(b)
	w = u.anfrage("GET", "/api/belege?jahr=alle&status=bestaetigt", nil)
	var l struct{ Belege []belegDTO }
	dekodiere(t, w, &l)
	if len(l.Belege) != 1 || l.Belege[0].Text != "" {
		t.Errorf("Liste: %+v", l)
	}
	// Löschen
	if w := u.anfrage("DELETE", "/api/belege/"+itoa(int(id)), nil); w.Code != 200 {
		t.Errorf("Löschen: %d", w.Code)
	}
	if w := u.anfrage("DELETE", "/api/belege/"+itoa(int(id)), nil); w.Code != 404 {
		t.Errorf("zweites Löschen: %d", w.Code)
	}
	if w := u.anfrage("DELETE", "/api/positionen/"+itoa(int(pid)), nil); w.Code != 200 {
		t.Errorf("Position löschen: %d", w.Code)
	}
}

func TestVorjahreUndUebersicht(t *testing.T) {
	u := neu(t)
	d := &falscherDienst{}
	srv := &Server{Store: u.s, Dienst: d, Datenordner: u.ordner, StandardJahr: 2026}
	u.h = srv.Handler()

	if w := u.anfrage("POST", "/api/vorjahre", vorjahrEingabe{Werte: []modell.Vorjahreswert{{Steuerjahr: 2025, Schluessel: "unsinn", BetragCent: 1}}}); w.Code != 400 {
		t.Errorf("unbekannter Schlüssel: %d", w.Code)
	}
	u.anfrage("POST", "/api/vorjahre", vorjahrEingabe{Werte: []modell.Vorjahreswert{
		{Steuerjahr: 2025, Schluessel: "bescheid.zve", BetragCent: 1},
		{Steuerjahr: 2025, Schluessel: "lstb.3", BetragCent: 100000},
	}})
	u.anfrage("POST", "/api/positionen", positionEingabe{Steuerjahr: 2026, Person: "A", Kategorie: "lstb.3", BetragCent: 200000, Bestaetigt: true})
	w := u.anfrage("GET", "/api/vorjahre?jahr=2026", nil)
	var v struct {
		Vorjahr int            `json:"vorjahr"`
		Werte   []vorjahrZeile `json:"werte"`
	}
	dekodiere(t, w, &v)
	if w.Code != 200 || v.Vorjahr != 2025 || len(v.Werte) != 2 {
		t.Fatalf("Vorjahre: %d %+v", w.Code, v)
	}
	for _, z := range v.Werte {
		if z.Schluessel == "lstb.3" && (!z.Auffaellig || z.AbweichungPro == nil || *z.AbweichungPro != 100) {
			t.Errorf("lstb.3 Abweichung: %+v", z)
		}
	}
	if p, auf, _ := abweichung(1000, 1200); auf || *p != 20 {
		t.Errorf("20 %% sind nicht auffällig")
	}
	if _, auf, _ := abweichung(1000, 1260); !auf {
		t.Errorf("26 %% sind auffällig")
	}
	if _, auf, _ := abweichung(-1000, -1300); !auf {
		t.Errorf("-30 %% sind auffällig")
	}
	// Erstattung ↔ Nachzahlung oder Vorjahr 0: keine Prozentangabe, nur Markierung.
	if p, auf, m := abweichung(50000, -20000); p != nil || !auf || m != "Vorzeichenwechsel" {
		t.Errorf("Vorzeichenwechsel: %v %v %q", p, auf, m)
	}
	if p, auf, m := abweichung(0, 5000); p != nil || !auf || m == "" {
		t.Errorf("Vorjahr 0: %v %v %q", p, auf, m)
	}
	if p, auf, _ := abweichung(0, 0); p != nil || auf {
		t.Errorf("beide 0: %v %v", p, auf)
	}
	if w := u.anfrage("DELETE", "/api/vorjahre?jahr=2025&schluessel=lstb.3", nil); w.Code != 200 {
		t.Errorf("Löschen: %d", w.Code)
	}

	// Übersicht und Eingang
	u.beleg("Belege/2026/a.pdf")
	w = u.anfrage("GET", "/api/uebersicht", nil)
	var ue map[string]any
	dekodiere(t, w, &ue)
	if ue["belege_neu"].(float64) != 1 || ue["jahr"].(float64) != 2026 || ue["eingang"] == "" {
		t.Errorf("Übersicht: %v", ue)
	}
	w = u.anfrage("POST", "/api/eingang/pruefen", map[string]any{})
	var ein struct{ Neu int }
	dekodiere(t, w, &ein)
	if w.Code != 200 || ein.Neu != 2 || d.n != 1 {
		t.Errorf("Eingang: %d %+v %d", w.Code, ein, d.n)
	}
	// Checkliste
	w = u.anfrage("GET", "/api/checkliste", nil)
	var c struct{ Punkte []checkpunkt }
	dekodiere(t, w, &c)
	if w.Code != 200 || len(c.Punkte) == 0 {
		t.Errorf("Checkliste: %d %s", w.Code, w.Body.String())
	}
	var gefunden bool
	for _, p := range c.Punkte {
		if strings.Contains(p.Titel, "Person B") {
			gefunden = true
		}
	}
	if !gefunden {
		t.Errorf("Lohnsteuerbescheinigung Person B fehlt nicht in der Checkliste")
	}
}

func TestStatischeDateien(t *testing.T) {
	u := neu(t)
	for _, p := range []string{"/", "/app.js", "/style.css"} {
		w := u.anfrage("GET", p, nil)
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
	w := u.anfrage("GET", "/", nil)
	body := w.Body.String()
	for _, verboten := range []string{"http://", "https://", "//cdn", "@import"} {
		if strings.Contains(body, verboten) {
			t.Errorf("index.html enthält externe Ressource: %s", verboten)
		}
	}
	css := u.anfrage("GET", "/style.css", nil).Body.String()
	if strings.Contains(css, "url(http") || strings.Contains(css, "@import") || strings.Contains(css, "@font-face") {
		t.Errorf("style.css lädt externe Ressourcen")
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Errorf("CSP fehlt")
	}
}

func TestZahlenNormalisierung(t *testing.T) {
	for in, want := range map[string]string{"1.234,56": "1234.56", "1234,56": "1234.56", "12": "12", "1.234": "1234", "1,5": "1.5", "12,5 €": "12.5", "1.234.567,89": "1234567.89", "0.5": "0.5"} {
		if got, ok := normZahl(in); !ok || got != want {
			t.Errorf("%q -> %q (%v), erwartet %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "abc", "1,2,3", "-5", "1e5"} {
		if _, ok := normZahl(in); ok {
			t.Errorf("%q sollte ungültig sein", in)
		}
	}
}

func TestLausche(t *testing.T) {
	l1, err := Lausche(18741)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	l2, err := Lausche(18741)
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	if l1.Addr().String() == l2.Addr().String() {
		t.Errorf("Ausweichlogik greift nicht")
	}
	if !strings.HasPrefix(l2.Addr().String(), "127.0.0.1:") {
		t.Errorf("nicht an Loopback gebunden: %s", l2.Addr())
	}
}
