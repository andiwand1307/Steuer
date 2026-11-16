package erfassung

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"steuer/internal/store"
)

// mindestAlter: Dateien, die jünger sind, werden noch geschrieben und
// übersprungen.
const mindestAlter = 2 * time.Second

// Dienst verarbeitet den Eingangsordner.
type Dienst struct {
	datenordner string
	store       *store.Store
	standard    int
	mu          sync.Mutex
	jetzt       func() time.Time
}

// Neu erzeugt den Dienst für <datenordner>/Eingang.
func Neu(datenordner string, s *store.Store, standardJahr int) *Dienst {
	return &Dienst{datenordner: datenordner, store: s, standard: standardJahr, jetzt: time.Now}
}

func (d *Dienst) eingang() string { return filepath.Join(d.datenordner, "Eingang") }

// ignoriert: temporäre und Systemdateien.
func ignoriert(name string) bool {
	l := strings.ToLower(name)
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~$") || strings.HasPrefix(name, "~") {
		return true
	}
	switch l {
	case "desktop.ini", "thumbs.db":
		return true
	}
	switch filepath.Ext(l) {
	case ".tmp", ".part", ".crdownload", ".download", ".partial", ".lnk":
		return true
	}
	return false
}

// Durchlauf verarbeitet alle fertigen Dateien im Eingang. neu zählt neu
// angelegte Belege; Doppelte zählen nicht.
func (d *Dienst) Durchlauf() (neu int, fehler []error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	eintraege, err := os.ReadDir(d.eingang())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, []error{err}
	}
	for _, e := range eintraege {
		if e.IsDir() || ignoriert(e.Name()) {
			continue
		}
		ok, err := d.verarbeite(filepath.Join(d.eingang(), e.Name()))
		if err != nil {
			fehler = append(fehler, fmt.Errorf("%s: %w", e.Name(), err))
		}
		if ok {
			neu++
		}
	}
	return neu, fehler
}

// verarbeite behandelt eine Datei; true = neuer Beleg angelegt.
func (d *Dienst) verarbeite(pfad string) (bool, error) {
	st, err := os.Stat(pfad)
	if err != nil {
		return false, err
	}
	if !st.Mode().IsRegular() || d.jetzt().Sub(st.ModTime()) < mindestAlter {
		return false, nil // wird noch geschrieben
	}
	hash, err := sha256Datei(pfad)
	if err != nil {
		return false, err // z.B. noch gesperrt; nächster Durchlauf versucht es erneut
	}
	if st2, err := os.Stat(pfad); err != nil || st2.Size() != st.Size() || !st2.ModTime().Equal(st.ModTime()) {
		return false, nil // hat sich während des Lesens geändert
	}
	te := textGewinnen(pfad)
	erg := auswerten(filepath.Base(pfad), hash, te, d.standard)
	b := erg.Beleg
	ext := strings.ToLower(filepath.Ext(pfad))

	// Ablageziel bestimmen.
	datum := b.Belegdatum
	if datum == "" {
		datum = b.ErfasstAm.Format("2006-01-02")
	}
	zielDir := filepath.Join(d.datenordner, "Belege", fmt.Sprint(b.Steuerjahr))
	if err := os.MkdirAll(zielDir, 0o755); err != nil {
		return false, err
	}
	ziel, err := freierName(zielDir, ablageName(datum, b.Art, b.Aussteller), ext)
	if err != nil {
		return false, err
	}
	if err := os.Rename(pfad, ziel); err != nil {
		return false, fmt.Errorf("verschieben: %w", err)
	}
	rel, err := filepath.Rel(d.datenordner, ziel)
	if err != nil {
		rel = ziel
	}
	b.Datei = filepath.ToSlash(rel)

	id, err := d.store.BelegAnlegen(b)
	if errors.Is(err, store.ErrDoppelt) {
		dopp := filepath.Join(d.eingang(), "Doppelt")
		if mkErr := os.MkdirAll(dopp, 0o755); mkErr != nil {
			_ = os.Rename(ziel, pfad)
			return false, mkErr
		}
		dz, nErr := freierName(dopp, strings.TrimSuffix(filepath.Base(pfad), filepath.Ext(pfad)), filepath.Ext(pfad))
		if nErr == nil {
			nErr = os.Rename(ziel, dz)
		}
		if nErr != nil {
			_ = os.Rename(ziel, pfad)
			return false, nErr
		}
		return false, nil
	}
	if err != nil {
		_ = os.Rename(ziel, pfad) // zurück in den Eingang, nächster Durchlauf versucht es erneut
		return false, err
	}
	var perr []error
	for _, p := range erg.Positionen {
		p.BelegID = &id
		if _, err := d.store.PositionAnlegen(p); err != nil {
			perr = append(perr, err)
		}
	}
	if len(perr) > 0 {
		return true, fmt.Errorf("positionen nicht vollständig gespeichert: %w", errors.Join(perr...))
	}
	return true, nil
}

// Starte führt sofort und dann alle intervall einen Durchlauf aus, bis ctx
// endet. Der Aufruf blockiert; Fehler werden protokolliert.
func (d *Dienst) Starte(ctx context.Context, intervall time.Duration) {
	if intervall <= 0 {
		intervall = 5 * time.Second
	}
	lauf := func() {
		if n, fehler := d.Durchlauf(); n > 0 || len(fehler) > 0 {
			log.Printf("Erfassung: %d neue Belege, %d Fehler", n, len(fehler))
			for _, e := range fehler {
				log.Printf("Erfassung: %v", e)
			}
		}
	}
	lauf()
	t := time.NewTicker(intervall)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			lauf()
		}
	}
}

// ---- Dateinamen ----

// bereinige ersetzt für Windows unzulässige Zeichen und kürzt.
func bereinige(s string, max int) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r < 32 || strings.ContainsRune(`<>:"/\|?*`, r):
			sb.WriteRune('_')
		default:
			sb.WriteRune(r)
		}
	}
	out := strings.Join(strings.Fields(sb.String()), " ")
	if rs := []rune(out); len(rs) > max {
		out = string(rs[:max])
	}
	return strings.TrimSpace(out)
}

// ablageName bildet "JJJJMMTT_<Art> (<Aussteller>)" aus einem Datum JJJJ-MM-TT.
func ablageName(datum, art, aussteller string) string {
	n := strings.ReplaceAll(datum, "-", "") + "_" + bereinige(art, 40)
	if a := bereinige(aussteller, 40); a != "" {
		n += " (" + a + ")"
	}
	return n
}

// freierName liefert einen noch nicht vorhandenen Pfad, ggf. mit _2, _3 ...
func freierName(dir, basis, ext string) (string, error) {
	for i := 1; i < 10000; i++ {
		name := basis + ext
		if i > 1 {
			name = fmt.Sprintf("%s_%d%s", basis, i, ext)
		}
		p := filepath.Join(dir, name)
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			return p, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("kein freier Dateiname")
}
