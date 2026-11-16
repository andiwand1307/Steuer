package erfassung

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// Hinweistexte für fehlende Texterkennung.
const (
	hinweisKeinOCR      = "Texterkennung nicht verfügbar - bitte manuell erfassen"
	hinweisKeinPDFKonv  = "Gescanntes PDF: Texterkennung nicht verfügbar (Konverter pdftoppm fehlt) - bitte manuell erfassen"
	hinweisOCRGenutzt   = "Text per Texterkennung gewonnen - Werte bitte besonders sorgfältig prüfen"
	hinweisUnbekannt    = "Dateityp nicht lesbar - bitte manuell erfassen"
	maxTextZeichen      = 300000
	maxOCRSeiten        = 10
	ocrZeitlimitProSeit = 2 * time.Minute
)

// Pfade der externen Werkzeuge; in Tests ersetzbar.
var (
	tesseractSuchen = func() string {
		if p, err := exec.LookPath("tesseract"); err == nil {
			return p
		}
		const win = `C:\Program Files\Tesseract-OCR\tesseract.exe`
		if st, err := os.Stat(win); err == nil && !st.IsDir() {
			return win
		}
		return ""
	}
	pdftoppmSuchen = func() string {
		if p, err := exec.LookPath("pdftoppm"); err == nil {
			return p
		}
		return ""
	}
)

// TexterkennungVerfuegbar meldet, ob ein Tesseract für Bilder gefunden wurde.
func TexterkennungVerfuegbar() bool { return tesseractSuchen() != "" }

// textErgebnis ist der gewonnene Text samt Hinweis.
type textErgebnis struct {
	Text    string
	Hinweis string
}

// textGewinnen liest den Text einer Datei (PDF, Bild, txt/csv).
func textGewinnen(pfad string) textErgebnis {
	switch strings.ToLower(filepath.Ext(pfad)) {
	case ".txt", ".csv", ".tsv", ".text":
		b, err := os.ReadFile(pfad)
		if err != nil {
			return textErgebnis{Hinweis: "Datei nicht lesbar: " + err.Error()}
		}
		return textErgebnis{Text: dekodiereText(b)}
	case ".pdf":
		return pdfText(pfad)
	case ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".bmp", ".webp":
		te := tesseractSuchen()
		if te == "" {
			return textErgebnis{Hinweis: hinweisKeinOCR}
		}
		t, err := ocrBild(te, pfad)
		if err != nil {
			return textErgebnis{Hinweis: "Texterkennung fehlgeschlagen (" + err.Error() + ") - bitte manuell erfassen"}
		}
		return ocrErgebnis(t)
	}
	return textErgebnis{Hinweis: hinweisUnbekannt}
}

func ocrErgebnis(t string) textErgebnis {
	if len(strings.Fields(t)) < 3 {
		return textErgebnis{Text: t, Hinweis: "Texterkennung fand kaum Text - bitte manuell erfassen"}
	}
	return textErgebnis{Text: t, Hinweis: hinweisOCRGenutzt}
}

// dekodiereText wandelt UTF-8 oder (Rückfall) Windows-1252 in UTF-8.
func dekodiereText(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if len(b) > 5<<20 {
		b = b[:5<<20]
	}
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for _, c := range b {
		if c == 0x80 {
			sb.WriteRune('€')
		} else {
			sb.WriteRune(rune(c)) // Latin-1/CP1252 stimmen im Bereich A0-FF überein
		}
	}
	return sb.String()
}

// pdfText liest die Textebene; fehlt sie, wird (falls möglich) per OCR gelesen.
func pdfText(pfad string) textErgebnis {
	t, err := pdfTextebene(pfad)
	if len(strings.Fields(t)) >= 3 {
		return textErgebnis{Text: t}
	}
	te, conv := tesseractSuchen(), pdftoppmSuchen()
	switch {
	case te == "":
		return textErgebnis{Text: t, Hinweis: hinweisKeinOCR}
	case conv == "":
		return textErgebnis{Text: t, Hinweis: hinweisKeinPDFKonv}
	}
	o, oerr := ocrPDF(conv, te, pfad)
	if oerr != nil {
		h := "Texterkennung fehlgeschlagen (" + oerr.Error() + ") - bitte manuell erfassen"
		if err != nil {
			h += "; PDF: " + err.Error()
		}
		return textErgebnis{Text: t, Hinweis: h}
	}
	return ocrErgebnis(o)
}

// pdfTextebene liest den Text zeilenweise mit der Go-Bibliothek; Panics der
// Bibliothek bei defekten PDFs werden abgefangen.
func pdfTextebene(pfad string) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PDF nicht lesbar: %v", r)
		}
	}()
	f, r, err := pdf.Open(pfad)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		sb.WriteString(seitenText(p.Content().Text))
		sb.WriteString("\n")
		if sb.Len() > maxTextZeichen {
			break
		}
	}
	return sb.String(), err
}

func ocrBild(tesseract, bild string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ocrZeitlimitProSeit)
	defer cancel()
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, tesseract, bild, "stdout", "-l", "deu")
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if len(msg) > 120 {
			msg = msg[:120]
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return out.String(), nil
}

func ocrPDF(pdftoppm, tesseract, pdfPfad string) (string, error) {
	tmp, err := os.MkdirTemp("", "steuer-ocr-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	ctx, cancel := context.WithTimeout(context.Background(), ocrZeitlimitProSeit)
	defer cancel()
	cmd := exec.CommandContext(ctx, pdftoppm, "-r", "300", "-l", fmt.Sprint(maxOCRSeiten), "-png", pdfPfad, filepath.Join(tmp, "s"))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftoppm: %v", err)
	}
	bilder, _ := filepath.Glob(filepath.Join(tmp, "s*.png"))
	sort.Strings(bilder)
	var sb strings.Builder
	for _, b := range bilder {
		t, err := ocrBild(tesseract, b)
		if err != nil {
			return "", err
		}
		sb.WriteString(t)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}

// seitenText setzt die Zeichen einer Seite zu Zeilen zusammen: gleiche Y-Lage
// = eine Zeile (von oben nach unten), innerhalb der Zeile von links nach
// rechts; größere Lücken werden zu Leerzeichen.
func seitenText(zeichen []pdf.Text) string {
	type zeile struct {
		y  float64
		it []pdf.Text
	}
	var zeilen []*zeile
	for _, c := range zeichen {
		tol := c.FontSize * 0.4
		if tol < 1 {
			tol = 2
		}
		var z *zeile
		for _, k := range zeilen {
			if d := k.y - c.Y; d < tol && d > -tol {
				z = k
				break
			}
		}
		if z == nil {
			z = &zeile{y: c.Y}
			zeilen = append(zeilen, z)
		}
		z.it = append(z.it, c)
	}
	sort.SliceStable(zeilen, func(a, b int) bool { return zeilen[a].y > zeilen[b].y })
	var sb strings.Builder
	for _, z := range zeilen {
		sort.SliceStable(z.it, func(a, b int) bool { return z.it[a].X < z.it[b].X })
		var line strings.Builder
		for k, c := range z.it {
			if k > 0 {
				p := z.it[k-1]
				luecke := c.X - (p.X + p.W)
				schwelle := c.FontSize * 0.2
				if schwelle < 1 {
					schwelle = 1
				}
				if luecke > schwelle*4 {
					line.WriteString("  ") // Spaltenabstand
				} else if luecke > schwelle && !strings.HasSuffix(line.String(), " ") && !strings.HasPrefix(c.S, " ") {
					line.WriteByte(' ')
				}
			}
			line.WriteString(c.S)
		}
		if t := strings.TrimSpace(line.String()); t != "" {
			sb.WriteString(t)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
