package web

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// inlineTypen sind Dateitypen, die der Browser gefahrlos anzeigen darf.
var inlineTypen = map[string]string{
	".pdf":  "application/pdf",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".txt":  "text/plain; charset=utf-8",
}

var errPfad = errors.New("Pfad nicht erlaubt")

// sichererPfad löst einen Beleg-Pfad (Schrägstriche, relativ zum Datenordner)
// auf und stellt sicher, dass er - auch über Symlinks - im Datenordner bleibt
// und nicht in die Datenbank (Unterordner "daten") zeigt.
func (s *Server) sichererPfad(rel string) (string, error) {
	if rel == "" || strings.ContainsRune(rel, 0) {
		return "", errPfad
	}
	nativ := filepath.FromSlash(rel)
	if filepath.IsAbs(nativ) || filepath.VolumeName(nativ) != "" || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", errPfad
	}
	wurzelAbs, err := filepath.Abs(s.Datenordner)
	if err != nil {
		return "", err
	}
	wurzel, err := filepath.EvalSymlinks(wurzelAbs)
	if err != nil {
		return "", err
	}
	voll := filepath.Join(wurzel, nativ)
	if !innerhalb(wurzel, voll) {
		return "", errPfad
	}
	echt, err := filepath.EvalSymlinks(voll)
	if err != nil {
		return "", err
	}
	if !innerhalb(wurzel, echt) {
		return "", errPfad
	}
	r, _ := filepath.Rel(wurzel, echt)
	if erster := strings.Split(filepath.ToSlash(r), "/")[0]; strings.EqualFold(erster, "daten") {
		return "", errPfad
	}
	st, err := os.Stat(echt)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", errPfad
	}
	return echt, nil
}

func innerhalb(wurzel, pfad string) bool {
	r, err := filepath.Rel(wurzel, pfad)
	if err != nil || filepath.IsAbs(r) {
		return false
	}
	return r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && r != "."
}

func (s *Server) belegDatei(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		fehler(w, 400, err.Error())
		return
	}
	b, err := s.Store.Beleg(id)
	if err != nil {
		fehler(w, 404, "Beleg nicht gefunden.")
		return
	}
	pfad, err := s.sichererPfad(b.Datei)
	if err != nil {
		if errors.Is(err, errPfad) {
			fehler(w, 403, "Diese Datei darf nicht ausgeliefert werden.")
		} else {
			fehler(w, 404, "Die Datei wurde nicht gefunden (verschoben oder gelöscht?).")
		}
		return
	}
	f, err := os.Open(pfad)
	if err != nil {
		fehler(w, 404, "Die Datei wurde nicht gefunden.")
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		serverfehler(w, err)
		return
	}
	art := "attachment"
	typ, inline := inlineTypen[strings.ToLower(filepath.Ext(pfad))]
	if inline {
		art = "inline"
	} else {
		typ = "application/octet-stream"
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(art, map[string]string{"filename": filepath.Base(pfad)}))
	if typ != "application/pdf" { // Chrome blockiert PDFs mit sandbox
		w.Header().Set("Content-Security-Policy", "sandbox")
	}
	http.ServeContent(w, r, "", st.ModTime(), f)
}
