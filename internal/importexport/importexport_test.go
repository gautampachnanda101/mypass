package importexport

import (
	"bytes"
	"strings"
	"testing"
)

// --- detection ---

func TestDetectGoogle(t *testing.T) {
	csv := "name,url,username,password\nMyBank,https://bank.com,alice,s3cr3t\n"
	if got := Detect([]byte(csv)); got != FormatGoogle {
		t.Fatalf("got %s want %s", got, FormatGoogle)
	}
}

func TestDetectLastPass(t *testing.T) {
	csv := "url,username,password,totp,extra,name,grouping,fav\n"
	if got := Detect([]byte(csv)); got != FormatLastPass {
		t.Fatalf("got %s want %s", got, FormatLastPass)
	}
}

func TestDetectBitwardenJSON(t *testing.T) {
	j := `{"encrypted":false,"items":[]}`
	if got := Detect([]byte(j)); got != FormatBitwarden {
		t.Fatalf("got %s want %s", got, FormatBitwarden)
	}
}

func TestDetectVaultxJSON(t *testing.T) {
	j := `{"vaultx":true,"records":[]}`
	if got := Detect([]byte(j)); got != FormatVaultx {
		t.Fatalf("got %s want %s", got, FormatVaultx)
	}
}

func TestDetectGenericCSV(t *testing.T) {
	csv := "site,login,pass\nexample.com,bob,pw\n"
	if got := Detect([]byte(csv)); got != FormatCSVGeneric {
		t.Fatalf("got %s want %s", got, FormatCSVGeneric)
	}
}

// --- import ---

func TestImportGoogle(t *testing.T) {
	csv := "name,url,username,password\nMyBank,https://bank.com,alice,hunter2\n"
	recs, err := Import(strings.NewReader(csv), FormatGoogle)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	r := recs[0]
	if r.Name != "MyBank" || r.Username != "alice" || r.Password != "hunter2" || r.URL != "https://bank.com" {
		t.Fatalf("unexpected record: %+v", r)
	}
}

func TestImportLastPass(t *testing.T) {
	csv := "url,username,password,totp,extra,name,grouping,fav\nhttps://github.com,bob,passw0rd,JBSWY3DPEHPK3PXP,,GitHub,,0\n"
	recs, err := Import(strings.NewReader(csv), FormatLastPass)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].TOTPSeed != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("unexpected record: %+v", recs)
	}
}

func TestImportBitwardenJSON(t *testing.T) {
	j := `{
		"encrypted": false,
		"items": [{
			"name": "GitHub",
			"notes": "work account",
			"login": {
				"username": "dev",
				"password": "gh-token",
				"totp": "",
				"uris": [{"uri": "https://github.com"}]
			}
		}]
	}`
	recs, err := Import(strings.NewReader(j), FormatBitwarden)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].URL != "https://github.com" {
		t.Fatalf("unexpected record: %+v", recs)
	}
}

func TestImportBitwardenEncryptedReturnsError(t *testing.T) {
	j := `{"encrypted": true, "items": []}`
	_, err := Import(strings.NewReader(j), FormatBitwarden)
	if err == nil {
		t.Fatal("expected error for encrypted bitwarden export")
	}
}

func TestImportGenericCSV(t *testing.T) {
	csv := "title,email,pass,website\nAWS,admin@example.com,tok3n,https://aws.amazon.com\n"
	recs, err := Import(strings.NewReader(csv), FormatCSVGeneric)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Username != "admin@example.com" || recs[0].Password != "tok3n" {
		t.Fatalf("unexpected record: %+v", recs)
	}
}

func TestImportAutoDetect(t *testing.T) {
	csv := "name,url,username,password\nSite,https://x.com,u,p\n"
	recs, err := Import(strings.NewReader(csv), FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record via auto-detect, got %d", len(recs))
	}
}

// --- export ---

func TestExportVaultxRoundTrip(t *testing.T) {
	in := []Record{
		{Name: "GitHub", Username: "alice", Password: "s3cr3t", URL: "https://github.com"},
		{Name: "AWS", Username: "bob", Password: "tok3n", Notes: "prod account"},
	}

	var buf bytes.Buffer
	if err := Export(&buf, in, FormatVaultx); err != nil {
		t.Fatalf("export: %v", err)
	}

	out, err := Import(&buf, FormatVaultx)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("round-trip length mismatch: got %d want %d", len(out), len(in))
	}
	for i, r := range out {
		if r.Name != in[i].Name || r.Password != in[i].Password {
			t.Fatalf("row %d mismatch: got %+v want %+v", i, r, in[i])
		}
	}
}

func TestExportCSV(t *testing.T) {
	recs := []Record{{Name: "Test", Username: "u", Password: "p", URL: "https://t.com"}}
	var buf bytes.Buffer
	if err := Export(&buf, recs, FormatCSVGeneric); err != nil {
		t.Fatalf("export CSV: %v", err)
	}
	if !strings.Contains(buf.String(), "https://t.com") {
		t.Fatal("exported CSV missing URL")
	}
}

func TestExportBitwardenImportable(t *testing.T) {
	recs := []Record{{Name: "Site", Username: "u", Password: "pw", URL: "https://site.com"}}
	var buf bytes.Buffer
	if err := Export(&buf, recs, FormatBitwarden); err != nil {
		t.Fatalf("export bitwarden: %v", err)
	}
	// Should be importable back via bitwarden parser.
	out, err := Import(&buf, FormatBitwarden)
	if err != nil {
		t.Fatalf("re-import bitwarden: %v", err)
	}
	if len(out) != 1 || out[0].URL != "https://site.com" {
		t.Fatalf("unexpected re-imported record: %+v", out)
	}
}
