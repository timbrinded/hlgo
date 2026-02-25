package config

// RedactKey redacts a private key for display, showing only the first 4 and
// last 4 meaningful characters. For 0x-prefixed keys, the prefix is preserved
// and 4 hex characters are shown on each side. Keys shorter than 10 characters
// are fully redacted.
func RedactKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) < 10 {
		if len(key) >= 2 && key[:2] == "0x" {
			return "0x****"
		}
		return "****"
	}

	prefix := ""
	body := key
	if len(key) >= 2 && key[:2] == "0x" {
		prefix = "0x"
		body = key[2:]
	}
	return prefix + body[:4] + "..." + body[len(body)-4:]
}
