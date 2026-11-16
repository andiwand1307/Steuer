// Steuer ist ein lokales Hilfsprogramm für die Einkommensteuererklärung.
// Es läuft vollständig auf dem eigenen Rechner; die Oberfläche öffnet sich
// im Browser unter einer nur lokal erreichbaren Adresse.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"steuer/internal/store"
)

func main() {
	heim, _ := os.UserHomeDir()
	ordner := flag.String("ordner", filepath.Join(heim, "Documents", "Steuer"), "Datenordner mit Eingang, Belegen und Datenbank")
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
	fmt.Println("Datenordner:", *ordner)
	// Start von Erfassung und Oberfläche folgt in Phase 2.
}
