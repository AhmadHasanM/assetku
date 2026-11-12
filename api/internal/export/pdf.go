package export

import (
	"io"
	"strings"

	"assetku/api/internal/asset"
	"assetku/api/internal/pdf"
)

// WritePDF renders rows with pdf.AssetList.
func WritePDF(w io.Writer, tenantName, printedAt, filterSummary, locale string, rows []asset.ExportRow) error {
	pr := make([]pdf.AssetListRow, len(rows))
	for i, r := range rows {
		pr[i] = pdf.AssetListRow{AssetTag: r.AssetTag, Name: r.Name, Category: r.Category, Model: r.Model,
			SerialNumber: r.SerialNumber, Status: r.Status, LocationName: r.LocationName, AssignedTo: r.AssignedTo}
	}
	return pdf.AssetList(w, pdf.AssetListData{TenantName: tenantName, PrintedAt: printedAt,
		FilterSummary: filterSummary, Rows: pr, Locale: locale})
}

var monthsID = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
var monthsEN = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// filterSummary describes the active filter in the user's language.
func filterSummary(f asset.ListFilter, locationName, orgName, locale string) string {
	en := locale == "en"
	tr := func(id, e string) string {
		if en {
			return e
		}
		return id
	}
	var parts []string
	if f.Q != "" {
		parts = append(parts, tr("Cari", "Search")+": \""+f.Q+"\"")
	}
	if len(f.Categories) > 0 {
		var l []string
		for _, c := range f.Categories {
			l = append(l, pdf.LocalizedCategory(locale, c))
		}
		parts = append(parts, tr("Kategori", "Category")+": "+strings.Join(l, ", "))
	}
	if len(f.Statuses) > 0 {
		var l []string
		for _, s := range f.Statuses {
			l = append(l, pdf.LocalizedStatus(locale, s))
		}
		parts = append(parts, "Status: "+strings.Join(l, ", "))
	}
	if f.LocationID != nil {
		parts = append(parts, tr("Lokasi", "Location")+": "+locationName)
	}
	if f.OrganizationID != nil {
		parts = append(parts, tr("Organisasi", "Organization")+": "+orgName)
	}
	if len(parts) == 0 {
		return tr("Filter: semua aset", "Filter: all assets")
	}
	return tr("Filter: ", "Filter: ") + strings.Join(parts, "; ")
}
