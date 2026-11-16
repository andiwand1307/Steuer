// Package web stellt die lokale Bedienoberfläche bereit: eine JSON-API unter
// /api/ und die eingebetteten statischen Dateien der Oberfläche.
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"

	"steuer/internal/store"
)

//go:embed static
var staticDateien embed.FS

// Eingangsdienst ist der Teil des Erfassungsdienstes, den die Oberfläche braucht.
type Eingangsdienst interface {
	Durchlauf() (neu int, fehler []error)
}

// Server hält alles, was die Handler brauchen.
type Server struct {
	Store        *store.Store
	Dienst       Eingangsdienst
	Datenordner  string
	StandardJahr int
}

// Handler liefert den vollständigen, abgesicherten HTTP-Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routen(mux)
	statisch, err := fs.Sub(staticDateien, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServerFS(statisch))
	return schutz(mux)
}

// hostErlaubt akzeptiert nur Loopback-Namen (Schutz gegen DNS-Rebinding).
func hostErlaubt(hostport string) bool {
	h := hostport
	if hh, _, err := net.SplitHostPort(hostport); err == nil {
		h = hh
	}
	h = strings.Trim(strings.ToLower(h), "[]")
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func schutz(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostErlaubt(r.Host) {
			http.Error(w, "Zugriff nur über 127.0.0.1 oder localhost", http.StatusForbidden)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !jsonInhalt(r.Header.Get("Content-Type")) {
				fehler(w, http.StatusUnsupportedMediaType, "Nur Content-Type application/json ist erlaubt.")
				return
			}
			if r.Header.Get("Origin") != "http://"+r.Host {
				fehler(w, http.StatusForbidden, "Anfrage von fremdem Ursprung abgelehnt.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func jsonInhalt(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct == "application/json"
}

// ---- Hilfsfunktionen ----

func antwort(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fehler(w http.ResponseWriter, status int, text string) {
	antwort(w, status, map[string]string{"fehler": text})
}

func serverfehler(w http.ResponseWriter, err error) {
	fehler(w, http.StatusInternalServerError, "Interner Fehler: "+err.Error())
}

func lies(w http.ResponseWriter, r *http.Request, ziel any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(ziel); err != nil {
		fehler(w, http.StatusBadRequest, "Ungültige Eingabe: "+err.Error())
		return false
	}
	return true
}

func (s *Server) jahr(r *http.Request) (int, error) {
	return jahrParam(r, "jahr", s.StandardJahr)
}

func jahrParam(r *http.Request, name string, standard int) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return standard, nil
	}
	j, err := strconv.Atoi(v)
	if err != nil || j < 2000 || j > 2100 {
		return 0, fmt.Errorf("ungültiges Steuerjahr %q", v)
	}
	return j, nil
}

func idParam(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("ungültige Nummer")
	}
	return id, nil
}

// Lausche öffnet einen Listener auf 127.0.0.1. Ist port > 0, werden dieser und
// die folgenden 19 Ports versucht, danach ein beliebiger freier Port.
func Lausche(port int) (net.Listener, error) {
	if port > 0 {
		for p := port; p < port+20 && p < 65536; p++ {
			if l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p))); err == nil {
				return l, nil
			}
		}
	}
	return net.Listen("tcp", "127.0.0.1:0")
}
