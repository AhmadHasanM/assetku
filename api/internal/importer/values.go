package importer

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"
)

var categoryAliases = map[string][]string{
	"it_equipment":      {"peralatan it", "it", "it equipment", "komputer", "laptop", "computer"},
	"vehicle":           {"kendaraan", "mobil", "motor", "vehicle"},
	"building_facility": {"fasilitas gedung", "gedung", "bangunan", "building facility", "building"},
	"furniture":         {"furnitur", "furniture", "mebel", "perabot"},
	"electronics":       {"elektronik", "electronics", "electronic"},
	"other":             {"lainnya", "lain-lain", "other"},
}

var statusAliases = map[string][]string{
	"available":   {"available", "tersedia", "ready to deploy", "ready", "siap"},
	"in_use":      {"in use", "digunakan", "dipakai", "terpakai", "deployed"},
	"maintenance": {"maintenance", "perbaikan", "servis", "service", "pending"},
	"damaged":     {"damaged", "rusak", "broken", "un-deployable", "undeployable"},
	"lost":        {"lost", "hilang", "stolen", "dicuri"},
	"archived":    {"archived", "diarsipkan", "arsip"},
}

func invert(m map[string][]string) map[string]string {
	out := make(map[string]string)
	for code, aliases := range m {
		out[NormalizeHeader(code)] = code
		for _, a := range aliases {
			out[NormalizeHeader(a)] = code
		}
	}
	return out
}

var (
	categoryLookup = invert(categoryAliases)
	statusLookup   = invert(statusAliases)
)

// ParseCategory maps a category label to its enum code. Unknown input yields
// ("other", false); empty input yields ("other", true), as there is nothing to
// report.
func ParseCategory(s string) (code string, recognized bool) {
	n := NormalizeHeader(s)
	if n == "" {
		return "other", true
	}
	if c, ok := categoryLookup[n]; ok {
		return c, true
	}
	return "other", false
}

// ParseStatus maps a status label to its enum code. Empty is ("available",
// true); unknown is ("available", false).
func ParseStatus(s string) (code string, recognized bool) {
	n := NormalizeHeader(s)
	if n == "" {
		return "available", true
	}
	if c, ok := statusLookup[n]; ok {
		return c, true
	}
	return "available", false
}

var (
	serialRe  = regexp.MustCompile(`^\d+(\.\d+)?$`)
	dmyRe     = regexp.MustCompile(`^(\d{1,2})[/-](\d{1,2})[/-](\d{4})(?:[ T].*)?$`)
	ymdRe     = regexp.MustCompile(`^(\d{4})[/-](\d{1,2})[/-](\d{1,2})(?:[ T].*)?$`)
	textDayRe = regexp.MustCompile(`^(\d{1,2})[\s\-/.]+([\p{L}.]+)[\s\-/.,]+(\d{4})(?:[ T].*)?$`)
)

var months = map[string]time.Month{
	"januari": 1, "january": 1, "jan": 1,
	"februari": 2, "february": 2, "feb": 2, "pebruari": 2,
	"maret": 3, "march": 3, "mar": 3,
	"april": 4, "apr": 4,
	"mei": 5, "may": 5,
	"juni": 6, "june": 6, "jun": 6,
	"juli": 7, "july": 7, "jul": 7,
	"agustus": 8, "august": 8, "agu": 8, "ags": 8, "aug": 8,
	"september": 9, "sep": 9, "sept": 9,
	"oktober": 10, "october": 10, "okt": 10, "oct": 10,
	"november": 11, "nov": 11, "nopember": 11,
	"desember": 12, "december": 12, "des": 12, "dec": 12,
}

var excelEpoch = civil.Date{Year: 1899, Month: time.December, Day: 30}

// ParseDate parses an Excel serial (1-80000, fraction ignored), dd/mm/yyyy,
// dd-mm-yyyy, yyyy-mm-dd, or a date with an Indonesian/English month name.
// Empty input yields nil.
func ParseDate(s string) (*civil.Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if serialRe.MatchString(s) {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f < 1 || f >= 80001 {
			return nil, fmt.Errorf("invalid date %q", s)
		}
		d := excelEpoch.AddDays(int(math.Floor(f)))
		return &d, nil
	}
	var y, m, d int
	var month time.Month
	switch {
	case dmyRe.MatchString(s):
		g := dmyRe.FindStringSubmatch(s)
		d, _ = strconv.Atoi(g[1])
		m, _ = strconv.Atoi(g[2])
		y, _ = strconv.Atoi(g[3])
		month = time.Month(m)
	case ymdRe.MatchString(s):
		g := ymdRe.FindStringSubmatch(s)
		y, _ = strconv.Atoi(g[1])
		m, _ = strconv.Atoi(g[2])
		d, _ = strconv.Atoi(g[3])
		month = time.Month(m)
	case textDayRe.MatchString(s):
		g := textDayRe.FindStringSubmatch(s)
		d, _ = strconv.Atoi(g[1])
		name := strings.ToLower(strings.Trim(g[2], "."))
		mm, ok := months[name]
		if !ok {
			return nil, fmt.Errorf("invalid date %q", s)
		}
		month = mm
		y, _ = strconv.Atoi(g[3])
	default:
		return nil, fmt.Errorf("invalid date %q", s)
	}
	date := civil.Date{Year: y, Month: month, Day: d}
	if !date.IsValid() || y < 1900 {
		return nil, fmt.Errorf("invalid date %q", s)
	}
	return &date, nil
}

var (
	moneyExpRe   = regexp.MustCompile(`^\d+(\.\d+)?[eE][+-]?\d+$`)
	rpPrefixRe   = regexp.MustCompile(`(?i)^(rp|idr)[\s.]*`)
	moneyTrailRe = regexp.MustCompile(`[.,]-$`)
)

// ParseMoney parses amounts such as "Rp 12.500.000", "12,500,000.00",
// "12.500.000,50" and "12500000". When both separators occur the last one is
// the decimal separator; a lone separator followed by exactly three digits is
// a thousands separator. Negative or malformed input is an error; empty is nil.
func ParseMoney(s string) (*decimal.Decimal, error) {
	orig := s
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	bad := fmt.Errorf("invalid amount %q", orig)
	s = rpPrefixRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = moneyTrailRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return nil, bad
	}
	if s[0] == '-' || s[0] == '(' {
		return nil, errors.New("negative amount " + strconv.Quote(orig))
	}
	if moneyExpRe.MatchString(s) {
		d, err := decimal.NewFromString(s)
		if err != nil {
			return nil, bad
		}
		return &d, nil
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' && r != ',' {
			return nil, bad
		}
	}
	lastDot, lastComma := strings.LastIndexByte(s, '.'), strings.LastIndexByte(s, ',')
	var intPart, frac string
	switch {
	case lastDot >= 0 && lastComma >= 0:
		sepIdx, thousands := lastDot, byte(',')
		if lastComma > lastDot {
			sepIdx, thousands = lastComma, '.'
		}
		intPart, frac = s[:sepIdx], s[sepIdx+1:]
		if !validGroups(intPart, thousands) {
			return nil, bad
		}
		intPart = strings.ReplaceAll(intPart, string(thousands), "")
	case lastDot >= 0 || lastComma >= 0:
		sep := byte('.')
		idx := lastDot
		if lastComma >= 0 {
			sep, idx = ',', lastComma
		}
		multiple := strings.Count(s, string(sep)) > 1
		if multiple || len(s)-idx-1 == 3 {
			// thousands separator only
			if !validGroups(s, sep) {
				return nil, bad
			}
			intPart = strings.ReplaceAll(s, string(sep), "")
		} else {
			intPart, frac = s[:idx], s[idx+1:]
		}
	default:
		intPart = s
	}
	if intPart == "" && frac == "" {
		return nil, bad
	}
	if intPart == "" {
		intPart = "0"
	}
	if !allDigits(intPart) || (frac != "" && !allDigits(frac)) {
		return nil, bad
	}
	str := intPart
	if frac != "" {
		str += "." + frac
	}
	d, err := decimal.NewFromString(str)
	if err != nil {
		return nil, bad
	}
	return &d, nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// validGroups reports whether s is digits grouped by sep: 1-3 leading digits,
// then groups of exactly 3. A string without sep must be plain digits.
func validGroups(s string, sep byte) bool {
	parts := strings.Split(s, string(sep))
	for i, p := range parts {
		if !allDigits(p) {
			return false
		}
		if i == 0 && len(p) > 3 && len(parts) > 1 {
			return false
		}
		if i > 0 && len(p) != 3 {
			return false
		}
	}
	return true
}

var warrantyRe = regexp.MustCompile(`^(\d+)(?:\.0+)?\s*([\p{L}]*)$`)

// ParseWarrantyMonths parses "12", "12 bulan", "12 months", "1 tahun",
// "2 years" into a month count. Empty yields nil.
func ParseWarrantyMonths(s string) (*int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil, nil
	}
	g := warrantyRe.FindStringSubmatch(s)
	if g == nil {
		return nil, fmt.Errorf("invalid warranty %q", s)
	}
	n, err := strconv.Atoi(g[1])
	if err != nil || n > 100000 {
		return nil, fmt.Errorf("invalid warranty %q", s)
	}
	switch g[2] {
	case "", "bulan", "bln", "month", "months", "mo", "mos":
	case "tahun", "thn", "year", "years", "yr", "yrs":
		n *= 12
	default:
		return nil, fmt.Errorf("invalid warranty %q", s)
	}
	return &n, nil
}

// UnquoteFormula removes one leading apostrophe when the next character is
// one of = + - @. The export adds it to such cells to defuse spreadsheet
// formulas; removing it keeps export then import lossless.
func UnquoteFormula(s string) string {
	if len(s) >= 2 && s[0] == '\'' && strings.ContainsRune("=+-@", rune(s[1])) {
		return s[1:]
	}
	return s
}
