package client

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/timbrinded/hlgo/pkg/output"
)

func validateExchangeResponse(raw json.RawMessage) error {
	var envelope struct {
		Status   string          `json:"status"`
		Response json.RawMessage `json:"response"`
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to decode exchange response envelope").
			WithDetails("path", "/exchange").
			WithDetails("cause", err.Error())
	}

	if !strings.EqualFold(envelope.Status, "err") {
		return validateExchangeStatuses(envelope.Status, envelope.Response)
	}

	cliErr := output.NewCLIError(output.ErrAPI, "exchange returned error status").
		WithDetails("path", "/exchange").
		WithDetails("exchange_status", envelope.Status)

	if len(envelope.Response) > 0 {
		var responseMessage string
		if err := json.Unmarshal(envelope.Response, &responseMessage); err == nil {
			cliErr.Message = "exchange error: " + responseMessage
			cliErr = cliErr.WithDetails("exchange_response", responseMessage)
		} else {
			cliErr = cliErr.WithDetails("exchange_response", string(envelope.Response))
		}
	}

	return cliErr
}

func isBenignExchangeStatus(status string) bool {
	switch normalized := strings.ToLower(strings.Trim(strings.TrimSpace(status), `"`)); normalized {
	case "", "success", "waitingforfill":
		return true
	default:
		return false
	}
}

func validateExchangeStatuses(status string, response json.RawMessage) error {
	var payload struct {
		Data struct {
			Statuses []json.RawMessage `json:"statuses"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &payload); err != nil {
		return nil
	}
	if len(payload.Data.Statuses) == 0 {
		return nil
	}

	var errs []string
	for _, entryRaw := range payload.Data.Statuses {
		var asString string
		if err := json.Unmarshal(entryRaw, &asString); err == nil {
			asString = strings.TrimSpace(asString)
			if isBenignExchangeStatus(asString) {
				continue
			}
			errs = append(errs, asString)
			continue
		}

		var asObject map[string]json.RawMessage
		if err := json.Unmarshal(entryRaw, &asObject); err != nil {
			continue
		}

		rawErr, ok := asObject["error"]
		if !ok {
			continue
		}

		var msg string
		if err := json.Unmarshal(rawErr, &msg); err == nil && strings.TrimSpace(msg) != "" {
			if isBenignExchangeStatus(msg) {
				continue
			}
			errs = append(errs, msg)
			continue
		}

		rawErrText := strings.TrimSpace(string(rawErr))
		if isBenignExchangeStatus(rawErrText) {
			continue
		}
		errs = append(errs, rawErrText)
	}

	if len(errs) == 0 {
		return nil
	}

	return output.NewCLIError(output.ErrAPI, "exchange action returned error statuses").
		WithDetails("path", "/exchange").
		WithDetails("exchange_status", status).
		WithDetails("exchange_errors", errs)
}
