package importexport

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Import reads all records from r in the given format.
// Use FormatAuto to detect the format from the file content.
func Import(r io.Reader, format Format) ([]Record, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read import data: %w", err)
	}

	if format == FormatAuto {
		format = Detect(data)
	}

	switch format {
	case FormatVaultx:
		return importVaultxJSON(data)
	case FormatBitwarden:
		return importBitwardenJSON(data)
	case FormatGoogle:
		return importCSV(data, mapGoogle)
	case FormatOnePassword:
		return importCSV(data, mapOnePassword)
	case FormatLastPass:
		return importCSV(data, mapLastPass)
	case FormatSamsung:
		return importCSV(data, mapSamsung)
	case FormatMcAfee:
		return importCSV(data, mapMcAfee)
	case FormatDashlane:
		return importCSV(data, mapDashlane)
	case FormatKeeper:
		return importCSV(data, mapKeeper)
	case FormatCSVGeneric:
		return importCSV(data, mapGenericCSV)
	default:
		return nil, fmt.Errorf("unsupported import format: %s", format)
	}
}

// --- vaultx native JSON ---

type vaultxBackup struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

func importVaultxJSON(data []byte) ([]Record, error) {
	var b vaultxBackup
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parse vaultx backup: %w", err)
	}
	return b.Records, nil
}

// --- Bitwarden JSON ---

type bitwardenExport struct {
	Encrypted bool `json:"encrypted"`
	Items     []struct {
		Name  string `json:"name"`
		Notes string `json:"notes"`
		Login *struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Totp     string `json:"totp"`
			URIs     []struct {
				URI string `json:"uri"`
			} `json:"uris"`
		} `json:"login"`
	} `json:"items"`
}

func importBitwardenJSON(data []byte) ([]Record, error) {
	var e bitwardenExport
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse bitwarden export: %w", err)
	}
	if e.Encrypted {
		return nil, fmt.Errorf("bitwarden export is encrypted — export as unencrypted JSON first")
	}

	var out []Record
	for _, item := range e.Items {
		r := Record{Name: item.Name, Notes: item.Notes}
		if item.Login != nil {
			r.Username = item.Login.Username
			r.Password = item.Login.Password
			r.TOTPSeed = item.Login.Totp
			if len(item.Login.URIs) > 0 {
				r.URL = item.Login.URIs[0].URI
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// --- CSV helpers ---

// rowMapper maps a header→value map to a Record. Returns nil to skip the row.
type rowMapper func(row map[string]string) *Record

func importCSV(data []byte, mapper rowMapper) ([]Record, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	// Normalise header names.
	for i, h := range headers {
		headers[i] = strings.ToLower(strings.TrimSpace(h))
	}

	var out []Record
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row: %w", err)
		}

		m := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(row) {
				m[h] = strings.TrimSpace(row[i])
			}
		}

		rec := mapper(m)
		if rec != nil {
			out = append(out, *rec)
		}
	}
	return out, nil
}

func col(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}

// --- per-format mappers ---

func mapGoogle(m map[string]string) *Record {
	return &Record{
		Name:     col(m, "name"),
		URL:      col(m, "url"),
		Username: col(m, "username"),
		Password: col(m, "password"),
		Notes:    col(m, "note", "notes"),
	}
}

func mapOnePassword(m map[string]string) *Record {
	return &Record{
		Name:     col(m, "title"),
		Username: col(m, "username"),
		Password: col(m, "password"),
		URL:      col(m, "url"),
		Notes:    col(m, "notes", "note"),
		TOTPSeed: col(m, "otp"),
	}
}

func mapLastPass(m map[string]string) *Record {
	name := col(m, "name")
	if name == "" {
		name = col(m, "grouping")
	}
	return &Record{
		Name:     name,
		URL:      col(m, "url"),
		Username: col(m, "username"),
		Password: col(m, "password"),
		Notes:    col(m, "extra"),
		TOTPSeed: col(m, "totp"),
	}
}

func mapSamsung(m map[string]string) *Record {
	return &Record{
		Name:     col(m, "name"),
		URL:      col(m, "url"),
		Username: col(m, "login"),
		Password: col(m, "password"),
	}
}

func mapMcAfee(m map[string]string) *Record {
	return &Record{
		Name:     col(m, "name"),
		URL:      col(m, "url"),
		Username: col(m, "username"),
		Password: col(m, "password"),
		Notes:    col(m, "note"),
	}
}

func mapDashlane(m map[string]string) *Record {
	return &Record{
		Name:     col(m, "title"),
		URL:      col(m, "url"),
		Username: col(m, "username"),
		Password: col(m, "password"),
		Notes:    col(m, "note"),
	}
}

func mapKeeper(m map[string]string) *Record {
	return &Record{
		Name:     col(m, "name"),
		URL:      col(m, "login_uri"),
		Username: col(m, "login_username"),
		Password: col(m, "login_password"),
		Notes:    col(m, "notes"),
	}
}

func mapGenericCSV(m map[string]string) *Record {
	name := col(m, "name", "title", "site", "service", "account")
	if name == "" {
		name = col(m, "url") // fall back to URL as name
	}
	r := &Record{
		Name:     name,
		Username: col(m, "username", "email", "login", "user"),
		Password: col(m, "password", "pass"),
		URL:      col(m, "url", "website", "uri"),
		Notes:    col(m, "notes", "note", "comment", "extra"),
		TOTPSeed: col(m, "totp", "otp", "2fa"),
	}
	if ts := col(m, "updated_at", "modified", "last_modified"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			r.UpdatedAt = t
		}
	}
	return r
}
