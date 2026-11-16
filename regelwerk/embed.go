// Package regelwerk enthält die Steuerwerte je Veranlagungszeitraum und den
// Feldkatalog der Formulare als eingebettete JSON-Dateien. Die Auswertung
// liegt im Paket rechner.
package regelwerk

import (
	"embed"
	"fmt"
)

//go:embed 2026.json felder_2025.json
var dateien embed.FS

// Jahr liefert die Steuerwerte eines Veranlagungszeitraums als JSON.
func Jahr(jahr int) ([]byte, error) {
	b, err := dateien.ReadFile(fmt.Sprintf("%d.json", jahr))
	if err != nil {
		return nil, fmt.Errorf("keine Steuerwerte für %d hinterlegt", jahr)
	}
	return b, nil
}

// Felder liefert den Feldkatalog der Formulare (Stand Vordrucke 2025) als JSON.
func Felder() []byte {
	b, err := dateien.ReadFile("felder_2025.json")
	if err != nil {
		panic("regelwerk: felder_2025.json fehlt: " + err.Error())
	}
	return b
}
