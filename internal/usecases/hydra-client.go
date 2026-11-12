package usecases

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// hydraSignatureHeader carries the HMAC-SHA256 of the request body, in the same form as GitHub's
// webhook signature ("sha256=<hex>"), which Hydra verifies with its webhook secret.
const hydraSignatureHeader = "X-Hub-Signature-256"

// errHydraRequest marks a failure to build the request (as opposed to sending it).
type errHydraRequest struct{ err error }

func (e errHydraRequest) Error() string { return e.err.Error() }
func (e errHydraRequest) Unwrap() error { return e.err }

// hydraEndpoint joins the configured Hydra base URL and an endpoint path.
func hydraEndpoint(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

// postToHydra sends a JSON body to a Hydra endpoint: it sets the content type and, when sign is
// true, the HMAC signature of the body. It returns the response status and body. A failure to
// build the request comes back as errHydraRequest; a failure to send it as the HTTP client's error.
// The caller decides what a non-200 status means and what to log. No timeout and no retries
func postToHydra(endpoint, secret string, sign bool, body []byte) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, errHydraRequest{err}
	}
	req.Header.Set("Content-Type", "application/json")
	if sign {
		req.Header.Set(hydraSignatureHeader, fmt.Sprintf("sha256=%s", computeHMACSHA256(body, secret)))
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}

// computeHMACSHA256 computes the HMAC-SHA256 signature for the given data
func computeHMACSHA256(data []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
