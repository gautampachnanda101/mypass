package importexport

import (
	"bytes"
	"strings"
)

// Detect sniffs the first few hundred bytes of a file and returns the most
// likely Format. Returns FormatCSVGeneric if nothing more specific matches.
func Detect(data []byte) Format {
	head := strings.ToLower(string(bytes.TrimSpace(data[:min(len(data), 512)])))

	// JSON exports
	if strings.HasPrefix(head, "{") || strings.HasPrefix(head, "[") {
		if strings.Contains(head, `"encrypted"`) || strings.Contains(head, `"items"`) {
			return FormatBitwarden
		}
		if strings.Contains(head, `"vaultx"`) || strings.Contains(head, `"records"`) {
			return FormatVaultx
		}
		return FormatBitwarden // fall through to bitwarden parser for unknown JSON
	}

	// CSV header sniffing
	firstLine := head
	if nl := strings.IndexByte(head, '\n'); nl >= 0 {
		firstLine = head[:nl]
	}
	firstLine = strings.TrimSpace(firstLine)

	switch {
	case strings.HasPrefix(firstLine, "name,url,username,password"):
		return FormatGoogle
	case strings.HasPrefix(firstLine, "title,username,password,otp,url,notes"):
		return FormatOnePassword
	case strings.HasPrefix(firstLine, "url,username,password,totp,extra,name,grouping,fav"):
		return FormatLastPass
	case strings.HasPrefix(firstLine, "name,url,login,password"):
		return FormatSamsung
	case strings.HasPrefix(firstLine, "kind,name,url,username,password,note"):
		return FormatMcAfee
	case strings.HasPrefix(firstLine, "title,note,type,url,username,password"):
		return FormatDashlane
	case strings.Contains(firstLine, "folder,favorite,type,name,notes,fields,reprompt,login_uri,login_username,login_password"):
		return FormatKeeper
	default:
		return FormatCSVGeneric
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
