package cmd

import (
	"sort"
	"strings"
)

func buildLookupAliases(coin string) []string {
	trimmed := strings.TrimSpace(coin)
	if trimmed == "" {
		return []string{}
	}

	aliases := make([]string, 0, 8)
	candidate := trimmed
	if idx := strings.Index(candidate, ":"); idx > 0 && idx+1 < len(candidate) {
		suffix := strings.TrimSpace(candidate[idx+1:])
		if suffix != "" {
			aliases = append(aliases, suffix)
			candidate = suffix
		}
	}

	base, quote, ok := splitMarketSymbol(candidate)
	if ok {
		aliases = append(aliases,
			base,
			base+"/"+quote,
			base+"-"+quote,
			base+quote,
		)
		if strings.HasSuffix(strings.ToUpper(quote), "USD") {
			aliases = append(aliases, base+"USD")
		}
	}

	return dedupeLookupAliases(aliases, coin)
}

func splitMarketSymbol(s string) (base string, quote string, ok bool) {
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		base = strings.TrimSpace(parts[0])
		quote = strings.TrimSpace(parts[1])
		if base != "" && quote != "" {
			return base, quote, true
		}
	}

	if parts := strings.SplitN(s, "-", 2); len(parts) == 2 {
		base = strings.TrimSpace(parts[0])
		quote = strings.TrimSpace(parts[1])
		if base != "" && quote != "" {
			return base, quote, true
		}
	}

	return "", "", false
}

func unitTokenAliases(token lookupSpotToken) []string {
	fullName := strings.TrimSpace(token.FullName)
	if !strings.HasPrefix(strings.ToUpper(fullName), "UNIT ") {
		return nil
	}

	name := strings.ToUpper(token.Name)
	if len(name) < 2 || !strings.HasPrefix(name, "U") {
		return nil
	}

	aliases := make([]string, 0, len(name)-1)
	seen := make(map[string]struct{}, len(name)-1)
	for stripped := name; len(stripped) > 1 && strings.HasPrefix(stripped, "U"); {
		stripped = stripped[1:]
		if _, ok := seen[stripped]; ok {
			continue
		}
		seen[stripped] = struct{}{}
		aliases = append(aliases, stripped)
	}

	return aliases
}

func dedupeLookupAliases(aliases []string, coin string) []string {
	seen := make(map[string]struct{}, len(aliases))
	out := make([]string, 0, len(aliases))
	coinLower := strings.ToLower(coin)
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if key == coinLower {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
