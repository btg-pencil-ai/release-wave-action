package usecases

import (
	"encoding/json"
	"errors"
	"fmt"
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

	// Signed even with an empty secret
	status, respBody, err := postToHydra(hydraEndpoint(hydraWebhookURL, "/epics/hydra-active"), hydraWebhookSecret, true, body)
	var buildErr errHydraRequest
	switch {
	case errors.As(err, &buildErr):
		return nil, fmt.Errorf("failed to create request: %w", buildErr.err)
	case err != nil && status == 0:
		return nil, fmt.Errorf("failed to call webhook endpoint: %w", err)
	case status != http.StatusOK:
		return nil, fmt.Errorf("webhook returned status %d: %s", status, string(respBody))
	case err != nil:
		return nil, err
	}

	var result ActiveEpicsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	l.Info("Fetched active epics: %v", result.EpicNames)
	return result.EpicNames, nil
}
