package amp

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
)

var internalRPCURL = "https://ampcode.com/api/internal"

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcResponse struct {
	Result *balanceResult `json:"result"`
	Error  *rpcError      `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type balanceResult struct {
	DisplayText string `json:"displayText"`
}

func fetchBalance(ctx context.Context, token string, source string, httpTimeout float64) (fetch.FetchResult, error) {
	client := httpclient.NewFromConfig(httpTimeout)
	body := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      "vibeusage",
		Method:  "userDisplayBalanceInfo",
		Params:  map[string]any{},
	}

	var rpcResp rpcResponse
	resp, err := client.PostJSONCtx(ctx, internalRPCURL, body, &rpcResp,
		httpclient.WithBearer(token),
		httpclient.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return fetch.ResultFail("Request failed: " + err.Error()), nil
	}

	if r := provider.CheckResponse(resp, "amp", "Amp"); r != nil {
		return *r, nil
	}
	if rpcResp.Error != nil {
		msg := strings.TrimSpace(rpcResp.Error.Message)
		if msg == "" {
			msg = fmt.Sprintf("rpc error code %d", rpcResp.Error.Code)
		}
		if strings.Contains(strings.ToLower(msg), "unauthor") {
			return fetch.ResultFatal("Token invalid or expired. Run `vibeusage auth amp` to re-authenticate."), nil
		}
		return fetch.ResultFail(msg), nil
	}
	if rpcResp.Result == nil {
		return fetch.ResultFail("Amp response missing result payload"), nil
	}

	snapshot, err := parseDisplayBalance(*rpcResp.Result, source)
	if err != nil {
		return fetch.ResultFail("Failed to parse Amp balance text: " + err.Error()), nil
	}
	return fetch.ResultOK(*snapshot), nil
}

var (
	quotaPattern   = regexp.MustCompile(`(?i)\$([0-9]+(?:\.[0-9]+)?)\s*/\s*\$([0-9]+(?:\.[0-9]+)?)`)
	hourlyPattern  = regexp.MustCompile(`(?i)\$([0-9]+(?:\.[0-9]+)?)\s*/\s*hour`)
	creditsPattern = regexp.MustCompile(`(?i)(?:bonus\s+)?credits?:\s*\$([0-9]+(?:\.[0-9]+)?)`)
)

func parseDisplayBalance(result balanceResult, source string) (*models.UsageSnapshot, error) {
	text := strings.TrimSpace(result.DisplayText)
	if text == "" {
		return nil, fmt.Errorf("empty displayText")
	}

	periods := make([]models.UsagePeriod, 0, 1)

	quotaMatch := quotaPattern.FindStringSubmatch(text)
	if len(quotaMatch) == 3 {
		remaining, err := strconv.ParseFloat(quotaMatch[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse remaining quota: %w", err)
		}
		total, err := strconv.ParseFloat(quotaMatch[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse total quota: %w", err)
		}

		utilization := 0
		if total > 0 {
			used := total - remaining
			if used < 0 {
				used = 0
			}
			utilization = int((used / total) * 100)
			if utilization < 0 {
				utilization = 0
			}
			if utilization > 100 {
				utilization = 100
			}
		}

		period := models.UsagePeriod{
			Name:        "Daily Free Quota",
			Utilization: utilization,
			PeriodType:  models.PeriodDaily,
		}

		rateMatch := hourlyPattern.FindStringSubmatch(text)
		if len(rateMatch) == 2 {
			rate, err := strconv.ParseFloat(rateMatch[1], 64)
			if err == nil && rate > 0 && total > remaining {
				hoursUntilFull := (total - remaining) / rate
				if hoursUntilFull > 0 {
					reset := time.Now().UTC().Add(time.Duration(hoursUntilFull * float64(time.Hour)))
					period.ResetsAt = &reset
				}
			}
		}

		periods = append(periods, period)
	}

	var billing *models.BillingDetail
	creditMatch := creditsPattern.FindStringSubmatch(text)
	if len(creditMatch) == 2 {
		credits, err := strconv.ParseFloat(creditMatch[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse credits amount: %w", err)
		}
		billing = &models.BillingDetail{Balance: &credits}
	}

	if len(periods) == 0 && billing == nil {
		return nil, fmt.Errorf("unrecognized displayText format: %q", text)
	}

	snapshot := &models.UsageSnapshot{
		Provider:  "amp",
		FetchedAt: time.Now().UTC(),
		Periods:   periods,
		Billing:   billing,
		Source:    source,
	}
	return snapshot, nil
}
