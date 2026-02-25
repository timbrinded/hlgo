package config

// RedactKey redacts a private key for display, showing only the first 4 and
// last 4 meaningful characters. For 0x-prefixed keys, the prefix is preserved
// and 4 hex characters are shown on each side. Keys with fewer than 9 body
// characters show only a prefix to avoid revealing most of the key through
// overlapping head/tail windows.
func RedactKey(key string) string {
	if key == "" {
		return ""
	}

	prefix := ""
	body := key
	if len(key) >= 2 && key[:2] == "0x" {
		prefix = "0x"
		body = key[2:]
	}

	// Need at least 9 body chars for head/tail to not overlap (4 + "..." + 4
	// would reveal 8 of 8 chars otherwise).
	if len(body) < 9 {
		if len(body) >= 4 {
			return prefix + body[:4] + "..."
		}
		return prefix + "****"
	}
	return prefix + body[:4] + "..." + body[len(body)-4:]
}
