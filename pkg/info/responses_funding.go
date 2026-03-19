package info

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

// annualHours is the number of hours per year, used for APR calculation from hourly rates.
var annualHours = decimal.NewFromInt(8760)

// UserFundingDelta holds the nested delta payload for user funding entries.
type UserFundingDelta struct {
	Type        string `json:"type"`
	Coin        string `json:"coin"`
	USDC        string `json:"usdc"`
	Szi         string `json:"szi,omitempty"`
	FundingRate string `json:"fundingRate,omitempty"`
}

// UserFundingEntry represents a single user funding event.
type UserFundingEntry struct {
	Time  int64            `json:"time"`
	Hash  string           `json:"hash"`
	Delta UserFundingDelta `json:"delta"`
}

// UserFundingResult is a list of user funding events.
type UserFundingResult []UserFundingEntry

// ParseUserFundingResult unmarshals raw JSON into a UserFundingResult.
func ParseUserFundingResult(raw json.RawMessage) (UserFundingResult, error) {
	var result UserFundingResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing user funding: %w", err)
	}
	return result, nil
}

// FundingEntry represents a single funding rate entry.
type FundingEntry struct {
	Coin        string `json:"coin"`
	FundingRate string `json:"fundingRate"`
	Premium     string `json:"premium"`
	Time        int64  `json:"time"`
}

// FundingResult is a list of funding entries.
type FundingResult []FundingEntry

// ParseFundingResult unmarshals raw JSON into a FundingResult.
func ParseFundingResult(raw json.RawMessage) (FundingResult, error) {
	var funding FundingResult
	if err := json.Unmarshal(raw, &funding); err != nil {
		return nil, fmt.Errorf("parsing funding: %w", err)
	}
	return funding, nil
}

func (FundingResult) Headers() []string { return []string{"COIN", "TIME", "RATE", "APR"} }

func (f FundingResult) Rows() [][]string {
	rows := make([][]string, 0, len(f))
	for _, entry := range f {
		rows = append(rows, []string{
			entry.Coin,
			formatTimestamp(entry.Time),
			entry.FundingRate,
			computeAPR(entry.FundingRate),
		})
	}
	return rows
}

// PredictedFundingVenueDetails holds venue funding details.
type PredictedFundingVenueDetails struct {
	FundingRate     string `json:"fundingRate"`
	NextFundingTime int64  `json:"nextFundingTime"`
}

// PredictedFundingVenue represents one venue entry for a coin.
type PredictedFundingVenue struct {
	Venue   string
	Details PredictedFundingVenueDetails
}

// PredictedFundingCoin represents one coin entry with all venue predictions.
type PredictedFundingCoin struct {
	Coin   string
	Venues []PredictedFundingVenue
}

// PredictedFundingsResult wraps the nested predicted funding response.
// API shape:
// [
//
//	["AVAX", [["BinPerp", {...}], ["HlPerp", {...}]]],
//	...
//
// ]
type PredictedFundingsResult []PredictedFundingCoin

// ParsePredictedFundingsResult unmarshals raw JSON into a PredictedFundingsResult.
func ParsePredictedFundingsResult(raw json.RawMessage) (PredictedFundingsResult, error) {
	var outer []json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parsing predicted fundings: %w", err)
	}

	result := make(PredictedFundingsResult, 0, len(outer))
	for i, coinEntryRaw := range outer {
		var coinEntry []json.RawMessage
		if err := json.Unmarshal(coinEntryRaw, &coinEntry); err != nil {
			return nil, fmt.Errorf("parsing predicted fundings coin entry %d: %w", i, err)
		}
		if len(coinEntry) != 2 {
			return nil, fmt.Errorf("parsing predicted fundings coin entry %d: expected length 2, got %d", i, len(coinEntry))
		}

		var coin string
		if err := json.Unmarshal(coinEntry[0], &coin); err != nil {
			return nil, fmt.Errorf("parsing predicted fundings coin name %d: %w", i, err)
		}

		var venuesRaw []json.RawMessage
		if err := json.Unmarshal(coinEntry[1], &venuesRaw); err != nil {
			return nil, fmt.Errorf("parsing predicted fundings venues for %s: %w", coin, err)
		}

		venues := make([]PredictedFundingVenue, 0, len(venuesRaw))
		for j, venueEntryRaw := range venuesRaw {
			var venueEntry []json.RawMessage
			if err := json.Unmarshal(venueEntryRaw, &venueEntry); err != nil {
				return nil, fmt.Errorf("parsing predicted fundings venue entry %s[%d]: %w", coin, j, err)
			}
			if len(venueEntry) != 2 {
				return nil, fmt.Errorf("parsing predicted fundings venue entry %s[%d]: expected length 2, got %d", coin, j, len(venueEntry))
			}

			var venue string
			if err := json.Unmarshal(venueEntry[0], &venue); err != nil {
				return nil, fmt.Errorf("parsing predicted fundings venue name %s[%d]: %w", coin, j, err)
			}

			var details PredictedFundingVenueDetails
			if err := json.Unmarshal(venueEntry[1], &details); err != nil {
				return nil, fmt.Errorf("parsing predicted fundings venue details %s[%d]: %w", coin, j, err)
			}

			venues = append(venues, PredictedFundingVenue{Venue: venue, Details: details})
		}

		result = append(result, PredictedFundingCoin{Coin: coin, Venues: venues})
	}

	return result, nil
}

func (PredictedFundingsResult) Headers() []string {
	return []string{"COIN", "VENUE", "PREDICTED_RATE", "APR"}
}

func (p PredictedFundingsResult) Rows() [][]string {
	rows := make([][]string, 0, len(p))
	for _, coinEntry := range p {
		for _, venueEntry := range coinEntry.Venues {
			rate := venueEntry.Details.FundingRate
			rows = append(rows, []string{coinEntry.Coin, venueEntry.Venue, rate, computeAPR(rate)})
		}
	}
	return rows
}

// PerpDex represents a HIP-3 perp dex.
type PerpDex struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	NumMarkets int    `json:"numMarkets"`
}

// PerpDexsResult is a list of perp dexes.
type PerpDexsResult []PerpDex

// ParsePerpDexsResult unmarshals raw JSON into a PerpDexsResult.
func ParsePerpDexsResult(raw json.RawMessage) (PerpDexsResult, error) {
	var dexs PerpDexsResult
	if err := json.Unmarshal(raw, &dexs); err != nil {
		return nil, fmt.Errorf("parsing perp dexs: %w", err)
	}
	return dexs, nil
}

func (PerpDexsResult) Headers() []string { return []string{"NAME", "INDEX", "NUM_MARKETS"} }

func (p PerpDexsResult) Rows() [][]string {
	rows := make([][]string, 0, len(p))
	for _, dex := range p {
		rows = append(rows, []string{dex.Name, fmt.Sprintf("%d", dex.Index), fmt.Sprintf("%d", dex.NumMarkets)})
	}
	return rows
}

// computeAPR converts an hourly funding rate string to an annualized percentage.
func computeAPR(rateStr string) string {
	rate, err := decimal.NewFromString(rateStr)
	if err != nil {
		return "N/A"
	}
	return rate.Mul(annualHours).String()
}
