// Steuer ist ein lokales Hilfsprogramm für die Einkommensteuererklärung.
// Es läuft vollständig auf dem eigenen Rechner; die Oberfläche öffnet sich
// im Browser unter einer nur lokal erreichbaren Adresse.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"steuer/internal/erfassung"
	"steuer/internal/store"
	"steuer/internal/web"
)

const standardJahr = 2026

func main() {
	heim, _ := os.UserHomeDir()
	ordner := flag.String("ordner", filepath.Join(heim, "Documents", "Steuer"), "Datenordner mit Eingang, Belegen und Datenbank")
	port := flag.Int("port", 8741, "bevorzugter Port auf 127.0.0.1 (ist er belegt, wird ein anderer gewählt; 0 = beliebiger freier Port)")
	keinBrowser := flag.Bool("kein-browser", false, "den Browser nicht automatisch öffnen")
	flag.Parse()

	for _, u := range []string{"Eingang", "Belege", "daten"} {
		if err := os.MkdirAll(filepath.Join(*ordner, u), 0o755); err != nil {
			log.Fatalf("Ordner %s kann nicht angelegt werden: %v", u, err)
		}
	}
	s, err := store.Open(filepath.Join(*ordner, "daten", "steuer.db"))
	if err != nil {
		log.Fatalf("Datenbank kann nicht geöffnet werden: %v", err)
	}
	defer s.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dienst := erfassung.Neu(*ordner, s, standardJahr)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dienst.Starte(ctx, 5*time.Second)
	}()

	l, err := web.Lausche(*port)
	if err != nil {
		log.Fatalf("Kein Port verfügbar: %v", err)
	}
	srv := &http.Server{
		Handler:           (&web.Server{Store: s, Dienst: dienst, Datenordner: *ordner, StandardJahr: standardJahr}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	adresse := "http://" + l.Addr().String() + "/"
	fmt.Println("Datenordner:", *ordner)
	fmt.Println("Eingang:    ", filepath.Join(*ordner, "Eingang"))
	fmt.Println("Oberfläche: ", adresse)
	fmt.Println("Beenden mit Strg+C.")
	if !*keinBrowser {
		if err := browserOeffnen(adresse); err != nil {
			fmt.Println("Der Browser konnte nicht geöffnet werden. Öffne die Adresse bitte selbst.")
		}
	}

	fehler := make(chan error, 1)
	go func() { fehler <- srv.Serve(l) }()
	select {
	case <-ctx.Done():
		fmt.Println("Wird beendet ...")
	case err := <-fehler:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Server: %v", err)
		}
	}
	stop()
	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
	wg.Wait()
}

func browserOeffnen(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
