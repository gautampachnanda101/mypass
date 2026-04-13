package importexport

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Export writes records to w in the given format.
// Supported export formats: FormatVaultx, FormatCSVGeneric, FormatBitwarden.
func Export(w io.Writer, records []Record, format Format) error {
	switch format {
	case FormatVaultx:
		return exportVaultxJSON(w, records)
	case FormatCSVGeneric:
		return exportGenericCSV(w, records)
	case FormatBitwarden:
		return exportBitwardenJSON(w, records)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// --- vaultx native JSON (lossless round-trip) ---

func exportVaultxJSON(w io.Writer, records []Record) error {
	b := vaultxBackup{Version: 1, Records: records}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(b)
}

// --- generic CSV ---

func exportGenericCSV(w io.Writer, records []Record) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"name", "username", "password", "url", "notes", "totp_seed", "updated_at",
	}); err != nil {
		return err
	}
	for _, r := range records {
		var ts string
		if !r.UpdatedAt.IsZero() {
			ts = r.UpdatedAt.UTC().Format(time.RFC3339)
		}
		if err := cw.Write([]string{
			r.Name, r.Username, r.Password, r.URL, r.Notes, r.TOTPSeed, ts,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// --- Bitwarden JSON (importable by Bitwarden / Vaultwarden) ---

func exportBitwardenJSON(w io.Writer, records []Record) error {
	type bwURI struct {
		Match interface{} `json:"match"`
		URI   string      `json:"uri"`
	}
	type bwLogin struct {
		Username string  `json:"username"`
		Password string  `json:"password"`
		Totp     string  `json:"totp,omitempty"`
		URIs     []bwURI `json:"uris"`
	}
	type bwItem struct {
		Type             int     `json:"type"` // 1 = login
		Name             string  `json:"name"`
		Notes            string  `json:"notes,omitempty"`
		FavoriteItem     bool    `json:"favorite"`
		Login            bwLogin `json:"login"`
		CollectionIDs    []interface{} `json:"collectionIds"`
	}
	type bwExport struct {
		Encrypted bool     `json:"encrypted"`
		Items     []bwItem `json:"items"`
	}

	items := make([]bwItem, len(records))
	for i, r := range records {
		uris := []bwURI{}
		if r.URL != "" {
			uris = append(uris, bwURI{URI: r.URL})
		}
		items[i] = bwItem{
			Type:  1,
			Name:  r.Name,
			Notes: r.Notes,
			Login: bwLogin{
				Username: r.Username,
				Password: r.Password,
				Totp:     r.TOTPSeed,
				URIs:     uris,
			},
			CollectionIDs: []interface{}{},
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(bwExport{Encrypted: false, Items: items})
}
