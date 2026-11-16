package web

import (
	"regexp"
	"strings"
	"time"
)

var reZahl = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

// normZahl wandelt deutsche oder englische Schreibweise in "1234.56".
func normZahl(s string) (string, bool) {
	s = strings.NewReplacer(" ", "", " ", "", "€", "", "EUR", "").Replace(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	if strings.Contains(s, ",") {
		if strings.Count(s, ",") > 1 {
			return "", false
		}
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	} else if strings.Count(s, ".") > 1 {
		s = strings.ReplaceAll(s, ".", "")
	} else if i := strings.LastIndex(s, "."); i >= 0 && len(s)-i-1 == 3 {
		s = s[:i] + s[i+1:]
	}
	if !reZahl.MatchString(s) {
		return "", false
	}
	return s, true
}

// normDatum akzeptiert leer, JJJJ-MM-TT oder TT.MM.JJJJ und liefert JJJJ-MM-TT.
func normDatum(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	for _, f := range []string{"2006-01-02", "02.01.2006", "2.1.2006"} {
		if t, err := time.Parse(f, s); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}
