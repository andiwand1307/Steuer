package erfassung

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// cp1252 wandelt Text in WinAnsi-Bytes für ein PDF.
func cp1252(s string) []byte {
	var out []byte
	for _, r := range s {
		switch {
		case r == '€':
			out = append(out, 0x80)
		case r < 256:
			out = append(out, byte(r))
		default:
			out = append(out, '?')
		}
	}
	return out
}

// erzeugePDF baut ein minimales einseitiges PDF mit Helvetica-Textzeilen.
// Ohne Zeilen entsteht eine Seite ohne Textebene (nur ein Rechteck).
func erzeugePDF(zeilen []string) []byte {
	var inhalt bytes.Buffer
	if len(zeilen) == 0 {
		inhalt.WriteString("0.8 g 100 100 300 200 re f\n")
	} else {
		inhalt.WriteString("BT /F1 10 Tf 40 800 Td\n")
		for _, z := range zeilen {
			z = strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(z)
			inhalt.WriteString("(")
			inhalt.Write(cp1252(z))
			inhalt.WriteString(") Tj 0 -12 Td\n")
		}
		inhalt.WriteString("ET\n")
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", inhalt.Len(), inhalt.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n", i+1)
		if i == 3 { // Inhaltsstrom enthält Latin-1-Bytes: roh schreiben
			buf.WriteString(fmt.Sprintf("<< /Length %d >>\nstream\n", inhalt.Len()))
			buf.Write(inhalt.Bytes())
			buf.WriteString("endstream")
		} else {
			buf.WriteString(o)
		}
		buf.WriteString("\nendobj\n")
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
}

func schreibe(t *testing.T, pfad string, inhalt []byte) {
	t.Helper()
	if err := os.WriteFile(pfad, inhalt, 0o644); err != nil {
		t.Fatal(err)
	}
}

func alt(t *testing.T, pfad string) {
	t.Helper()
	vor := time.Now().Add(-time.Minute)
	if err := os.Chtimes(pfad, vor, vor); err != nil {
		t.Fatal(err)
	}
}
