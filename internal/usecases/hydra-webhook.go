package usecases

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"release-candidate/internal/utils"
)

// ActiveEpicsResponse represents the response from the hydra-active endpoint
type ActiveEpicsResponse struct {
	EpicNames []string `json:"epic_names"`
}

// FetchHydraActiveEpics calls the Hydra webhook endpoint to get active epic names
func FetchHydraActiveEpics(l utils.LogInterface, hydraWebhookURL, hydraWebhookSecret string) ([]string, error) {
	if hydraWebhookURL == "" {
		l.Error("Hydra webhook URL not configured")
		return nil, fmt.Errorf("hydra webhook URL not configured")
	}
	body := []byte("{}")

	// Compute HMAC-SHA256 signature
	signature := computeHMACSHA256(body, hydraWebhookSecret)
	signatureHeader := fmt.Sprintf("sha256=%s", signature)

	req, err := http.NewRequest(http.MethodPost, hydraWebhookURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signatureHeader)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call webhook endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result ActiveEpicsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	l.Info("Fetched active epics: %v", result.EpicNames)
	return result.EpicNames, nil
}

// computeHMACSHA256 computes the HMAC-SHA256 signature for the given data
func computeHMACSHA256(data []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
