# Feldkatalog ESt - Hinweise (Stand 16.11.2026)

## Formularjahr
- Verwendet: **amtliche Vordrucke Einkommensteuer 2025** (Ersatz fuer 2026). Pruefung 2026: Formularserver (formulare-bfinv.de) fuehrt als neuesten Ordner "Einkommensteuer 2025"; ELSTER-Hilfe fuer 2026 ist leer. bundesfinanzministerium.de nicht erreichbar (Bot-Schutz, 503).
- Quellen: PDF-Vordrucke 2025 vom FMS (Stand 09-10/2025: ESt 1 A, Anlage N, N-Doppelte Haushaltsfuehrung, Kind, Vorsorgeaufwand, Sonderausgaben, Haushaltsnahe, Aussergewoehnliche Belastungen, AV, Sonstiges), ausgelesen mit pdftotext. Zeilennummern und Kennziffern stammen direkt aus den PDFs. Die URL (makePDF.do) liefert das PDF nur mit Browser-Sitzung (Cookie), nicht per einfachem curl.
- Anlage EUER: ELSTER-Anleitung zur Anlage EUER 2025 (https://www.elster.de/eportal/helpGlobal?themaGlobal=help_euer_ufa_77_2025). Dort nur Zeilennummern, **keine Kennziffern** (Feld = null).

## Luecken / unsichere Stellen (8 von 175 Eintraegen "unsicher")
- **Anlage G, Anlage S, AVEUER:** Das FMS stellt diese Vordrucke nicht bereit (Hinweis dort: Gewinneinkuenfte nur elektronisch). ELSTER-Hilfe hat dazu keine Zeilenanleitung. Keine Zeilen/Kennziffern; Eintraege nur als Platzhalter ohne Zeile. Gewerblich vs. freiberuflich: im EUER Z8 ("Einkunftsart") festzulegen.
- **Gewerbesteuer:** separates Formular, nicht abgelesen.
- **Lohnsteuerbescheinigung Nr. 3-7** (Brutto, LSt, SolZ, KiSt): auf den Vordrucken nicht aufgedruckt, aus Gedaechtnis. Verifiziert (auf den Vordrucken genannt) sind Nr. 8, 9, 10, 15, 17, 18, 22 a/b, 23 a/b, 24 a/b/c, 25, 26, 27, 29-32.
- EUER: keine Kennziffern; Uebertragungszeile des Gewinns in Anlage G/S nicht ermittelt (nur Z95 "Gewinn/Verlust" aus der Anleitung).
- Kinderbetreuung (Anlage Kind Z67/68): Zeilen/Kz abgelesen, Hoechstbetrag/Prozentsatz 2025 nicht gegen Gesetz geprueft. Anlage Kind Z6 (Kindergeld) hat Kz nicht eindeutig lesbar (leer gelassen).
- Anlage N Z5: zwei Spalten (Kz 110 Steuerklasse 1-5, Kz 111 Steuerklasse 6); relevant ist 110.
- Betraege/Hoechstgrenzen in "inhalt" stammen aus der ELSTER-Anleitung 2025.

## Hinweis zur Vorversion
Fruehere Zeilen aus der ELSTER-Hilfe waren bei Anlage N teils falsch zugeordnet (Reisekosten Z65-77 -> tatsaechlich Z66-68 mit Summe Z69 usw.); jetzt durch Vordruck ersetzt.

## Anfang 2027 zu aktualisieren
1. FMS-Ordner "Einkommensteuer 2026" laden (gleiches Verfahren), alle Zeilen/Kz gegen 2026 abgleichen; formularjahr und quelle anpassen.
2. EUER 2026 aus ELSTER-Hilfe (themaGlobal help_euer_ufa_77_2026) neu lesen; G/S ueber ELSTER-Formular/Software pruefen.
3. Hoechst-/Pauschbetraege 2026 pruefen (Pauschbetrag 1.230 EUR, Tagespauschale 6 EUR/210 Tage, Arbeitszimmer 1.260 EUR, Entfernungspauschale, Kinderbetreuung).
