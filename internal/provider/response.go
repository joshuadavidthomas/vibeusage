package provider

import (
	"fmt"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
)

// CheckResponse validates an HTTP response from a provider API. It returns a
// non-nil FetchResult for error conditions (auth failures, unexpected status
// codes, JSON parse errors) and nil when the response is OK for further
// processing.
//
// The providerID is used to construct the `vibeusage auth <provider>` hint in
// auth failure messages. The providerName is used in human-readable error
// messages.
func CheckResponse(resp *httpclient.Response, providerID, providerName string) *fetch.FetchResult {
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		r := fetch.ResultFatal(fmt.Sprintf(
			"Token invalid or expired. Run `vibeusage auth %s` to re-authenticate.", providerID,
		))
		return &r
	}
	if resp.StatusCode != 200 {
		r := fetch.ResultFail(fmt.Sprintf(
			"%s request failed: HTTP %d (%s)", providerName, resp.StatusCode, httpclient.SummarizeBody(resp.Body),
		))
		return &r
	}
	if resp.JSONErr != nil {
		r := fetch.ResultFail(fmt.Sprintf(
			"Invalid response from %s API: %v", providerName, resp.JSONErr,
		))
		return &r
	}
	return nil
}
