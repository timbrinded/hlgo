package cmd

import (
	"sort"
	"strconv"
	"strings"
)

type lookupScoredMatch struct {
	match infoLookupMatch
	rank  int
}

func scoreLookupMatches(records []lookupAssetRecord, query, mode string, limit int) []infoLookupMatch {
	if len(records) == 0 {
		return []infoLookupMatch{}
	}

	var scored []lookupScoredMatch
	if mode == "id" {
		id, err := strconv.Atoi(query)
		if err != nil {
			return []infoLookupMatch{}
		}
		scored = scoreIDMatches(records, id)
	} else {
		scored = scoreNameMatches(records, query)
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].rank != scored[j].rank {
			return scored[i].rank < scored[j].rank
		}
		leftType := marketTypeSortRank(scored[i].match.MarketType)
		rightType := marketTypeSortRank(scored[j].match.MarketType)
		if leftType != rightType {
			return leftType < rightType
		}
		if scored[i].match.Dex != scored[j].match.Dex {
			return scored[i].match.Dex < scored[j].match.Dex
		}
		if scored[i].match.Coin != scored[j].match.Coin {
			return scored[i].match.Coin < scored[j].match.Coin
		}
		return scored[i].match.AssetID < scored[j].match.AssetID
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	out := make([]infoLookupMatch, 0, len(scored))
	for _, scoredMatch := range scored {
		out = append(out, scoredMatch.match)
	}
	return out
}

func marketTypeSortRank(marketType string) int {
	switch marketType {
	case "perp":
		return 0
	case "spot":
		return 1
	default:
		return 2
	}
}

func scoreIDMatches(records []lookupAssetRecord, id int) []lookupScoredMatch {
	out := make([]lookupScoredMatch, 0, 2)
	for _, record := range records {
		if record.AssetID != id {
			continue
		}
		out = append(out, lookupScoredMatch{
			match: infoLookupMatch{
				Coin:       record.Coin,
				AssetID:    record.AssetID,
				MarketType: record.MarketType,
				Dex:        record.Dex,
				SzDecimals: record.SzDecimals,
				Aliases:    record.Aliases,
				MatchType:  "id",
			},
			rank: 0,
		})
	}
	return out
}

func scoreNameMatches(records []lookupAssetRecord, query string) []lookupScoredMatch {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	queryNormalized := normalizeLookupToken(queryLower)
	out := make([]lookupScoredMatch, 0, 16)

	for _, record := range records {
		rank := -1
		matchType := ""
		for _, token := range append([]string{record.Coin}, record.Aliases...) {
			candidate := strings.ToLower(strings.TrimSpace(token))
			if candidate == "" {
				continue
			}
			candidateNormalized := normalizeLookupToken(candidate)

			score, label := scoreLookupToken(candidate, candidateNormalized, queryLower, queryNormalized)
			if score == -1 {
				continue
			}
			if rank == -1 || score < rank {
				rank = score
				matchType = label
			}
			if rank == 0 {
				break
			}
		}

		if rank == -1 {
			continue
		}

		out = append(out, lookupScoredMatch{
			match: infoLookupMatch{
				Coin:       record.Coin,
				AssetID:    record.AssetID,
				MarketType: record.MarketType,
				Dex:        record.Dex,
				SzDecimals: record.SzDecimals,
				Aliases:    record.Aliases,
				MatchType:  matchType,
			},
			rank: rank,
		})
	}

	return out
}

func scoreLookupToken(candidate, candidateNormalized, query, queryNormalized string) (int, string) {
	switch {
	case candidate == query:
		return 0, "exact"
	case queryNormalized != "" && candidateNormalized == queryNormalized:
		return 1, "normalized_exact"
	case strings.HasPrefix(candidate, query):
		return 2, "prefix"
	case queryNormalized != "" && strings.HasPrefix(candidateNormalized, queryNormalized):
		return 3, "normalized_prefix"
	case strings.Contains(candidate, query):
		return 4, "contains"
	case queryNormalized != "" && strings.Contains(candidateNormalized, queryNormalized):
		return 5, "normalized_contains"
	case len(queryNormalized) >= 3 && isSubsequence(queryNormalized, candidateNormalized):
		return 6, "subsequence"
	default:
		return -1, ""
	}
}

func normalizeLookupToken(s string) string {
	if s == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return strings.ToLower(builder.String())
}

func isSubsequence(needle, haystack string) bool {
	if needle == "" {
		return true
	}
	needleIndex := 0
	for i := 0; i < len(haystack) && needleIndex < len(needle); i++ {
		if haystack[i] == needle[needleIndex] {
			needleIndex++
		}
	}
	return needleIndex == len(needle)
}
