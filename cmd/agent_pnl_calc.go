package cmd

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

func positionPnl(position info.Position, mids info.MidsResult, fundingByCoin map[string]decimal.Decimal) (*agentPositionPnl, *agentStepError) {
	size, err := decimal.NewFromString(position.Szi)
	if err != nil {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "invalid position size for " + position.Coin,
		}
		return nil, &stepErr
	}

	entryPx, err := decimal.NewFromString(position.EntryPx)
	if err != nil {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "invalid entry price for " + position.Coin,
		}
		return nil, &stepErr
	}

	midStr, ok := mids[position.Coin]
	if !ok {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "missing mid price for " + position.Coin,
		}
		return nil, &stepErr
	}

	midPx, err := decimal.NewFromString(midStr)
	if err != nil {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "invalid mid price for " + position.Coin,
		}
		return nil, &stepErr
	}

	unrealized := midPx.Sub(entryPx).Mul(size)
	funding := fundingByCoin[position.Coin]

	return &agentPositionPnl{Coin: position.Coin, Size: position.Szi, EntryPrice: position.EntryPx, MidPrice: midStr, UnrealizedPnl: unrealized.String(), FundingPnl: funding.String()}, nil
}

func addClosedPnl(fills info.FillsResult, errs []agentStepError, dex string) (decimal.Decimal, []agentStepError) {
	total := decimal.Zero
	for _, fill := range fills {
		if !coinInDexScope(fill.Coin, dex) {
			continue
		}

		closedPnl := fill.ClosedPnl
		if closedPnl == "" {
			continue
		}

		value, err := decimal.NewFromString(closedPnl)
		if err != nil {
			errs = append(errs, agentStepError{
				Step:  "fills",
				Code:  output.ErrAPI,
				Error: "invalid closedPnl for oid " + strconv.FormatInt(fill.Oid, 10),
			})
			continue
		}
		total = total.Add(value)
	}
	return total, errs
}

func aggregateFundingByCoin(funding info.UserFundingResult, errs []agentStepError, dex string) (map[string]decimal.Decimal, decimal.Decimal, []agentStepError) {
	byCoin := make(map[string]decimal.Decimal)
	total := decimal.Zero

	for _, entry := range funding {
		coin := entry.Delta.Coin
		if !coinInDexScope(coin, dex) {
			continue
		}
		if coin == "" {
			coin = "UNKNOWN"
		}
		if entry.Delta.USDC == "" {
			continue
		}

		value, err := decimal.NewFromString(entry.Delta.USDC)
		if err != nil {
			errs = append(errs, agentStepError{
				Step:  "user-funding",
				Code:  output.ErrAPI,
				Error: "invalid funding usdc for coin " + coin,
			})
			continue
		}

		byCoin[coin] = byCoin[coin].Add(value)
		total = total.Add(value)
	}

	return byCoin, total, errs
}

func coinInDexScope(coin, dex string) bool {
	scope := strings.TrimSpace(strings.ToLower(dex))
	if scope == "" {
		return true
	}

	trimmedCoin := strings.TrimSpace(coin)
	idx := strings.Index(trimmedCoin, ":")
	if idx <= 0 {
		return false
	}
	coinDex := strings.ToLower(strings.TrimSpace(trimmedCoin[:idx]))
	return coinDex == scope
}
