# Steuer – Hilfe für die Einkommensteuererklärung

Ein kleines Programm, das nur auf Deinem Rechner läuft. Es sammelt Deine Belege, rechnet die Einkommensteuer nach und zeigt Dir, **was Du wo in Mein ELSTER einträgst**. Nichts wird ins Internet geschickt.

## Starten

1. Doppelklicke `steuer.exe`. Es öffnet sich ein Konsolenfenster (nicht schließen) und Dein Browser mit der Oberfläche.
2. Beenden: Strg+C im Konsolenfenster oder das Fenster schließen.

Die Oberfläche ist nur auf diesem Rechner erreichbar (Adresse `http://127.0.0.1:8741/`). Optionen für die Eingabeaufforderung:

- `-ordner <Pfad>` anderer Datenordner
- `-port <Zahl>` anderer Port (ist er belegt, wird automatisch ein freier gewählt)
- `-kein-browser` Browser nicht automatisch öffnen

## Datenordner und Eingang

Standard: `Dokumente\Steuer`. Darin liegen:

- `Eingang` – hier legst Du Rechnungen, Lohnsteuerbescheinigungen, Bescheide usw. ab (PDF, Fotos, Textdateien). Das Programm liest sie alle paar Sekunden ein, schlägt Belegart, Datum, Betrag und Positionen vor und legt die Datei in `Belege` ab.
- `Belege` – die abgelegten Originale, nach Steuerjahr sortiert.
- `daten` – die Datenbank (`steuer.db`).

Vorschläge gehen erst in die Berechnung ein, wenn Du sie unter „Belege“ geprüft und **bestätigt** hast.

## Scans und Fotos (optional)

Für Fotos und eingescannte PDFs braucht das Programm die kostenlose Texterkennung **Tesseract** (inklusive deutscher Sprachdaten). Installiere sie und starte das Programm neu; auf der Übersicht steht dann „verfügbar“. PDFs mit echtem Text und Textdateien funktionieren auch ohne.

## Wichtig

- Die Formularzeilen und Kennziffern beruhen auf den **Vordrucken 2025**. Prüfe sie vor der Abgabe gegen die aktuellen Formulare in Mein ELSTER.
- Das Programm ersetzt **keine Steuerberatung**. Achte auf die Hinweise und die Sicherheitsstufen (sicher / stark evident / unsicher) in der Erklärung.

## Datensicherung

Kopiere den gesamten Datenordner (z. B. auf einen USB-Stick). Zum Wiederherstellen kopierst Du ihn zurück, solange das Programm nicht läuft.
