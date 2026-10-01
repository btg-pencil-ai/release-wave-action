package usecases

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"release-candidate/internal/configs"
	"release-candidate/internal/utils"
)

// releaseWaveStatusPayload mirrors hydra-controller's ProductionReleaseWavePayload
// (POST /production/release-wave) — keep the JSON tags in sync across the two repos.
type releaseWaveStatusPayload struct {
	RcVersion    string   `json:"rc_version"`
	Environment  string   `json:"environment,omitempty"`
	Repositories []string `json:"repositories,omitempty"`
	RunID        int64    `json:"run_id,omitempty"`
	RunAttempt   int      `json:"run_attempt,omitempty"`
	GithubRepo   string   `json:"github_repo,omitempty"`
	Event        string   `json:"event"`
	Conclusion   string   `json:"conclusion,omitempty"`
}

// PostReleaseWaveStatus reports this release wave's status to the Hydra console. Best-effort: it logs
// and returns on any error and never blocks the release. event is workflow_start|workflow_completed|workflow_failed.
func PostReleaseWaveStatus(l utils.LogInterface, cfg *configs.Config, event, conclusion string, repos []string) {
	if cfg.HydraWebhookURL == "" {
		l.Info("PostReleaseWaveStatus: hydra webhook URL not set, skipping (event=%s)", event)
		return
	}
	runID, _ := strconv.ParseInt(os.Getenv("GITHUB_RUN_ID"), 10, 64)
	runAttempt, _ := strconv.Atoi(os.Getenv("GITHUB_RUN_ATTEMPT"))

	payload := releaseWaveStatusPayload{
		RcVersion:    cfg.RCVersion,
		Environment:  cfg.Environment,
		Repositories: repos,
		RunID:        runID,
		RunAttempt:   runAttempt,
		GithubRepo:   os.Getenv("GITHUB_REPOSITORY"),
		Event:        event,
		Conclusion:   conclusion,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		l.Error("PostReleaseWaveStatus: marshal failed: %v", err)
		return
	}

	endpoint := strings.TrimRight(cfg.HydraWebhookURL, "/") + "/production/release-wave"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		l.Error("PostReleaseWaveStatus: build request failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.HydraWebhookSecret != "" {
		sig := computeHMACSHA256(body, cfg.HydraWebhookSecret)
		req.Header.Set("X-Hub-Signature-256", fmt.Sprintf("sha256=%s", sig))
	}

	l.Info("PostReleaseWaveStatus: POST %s body=%s", endpoint, string(body))

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		l.Error("PostReleaseWaveStatus: request failed (event=%s): %v", event, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		l.Error("PostReleaseWaveStatus: webhook returned status %d (event=%s)", resp.StatusCode, event)
		return
	}
	l.Info("PostReleaseWaveStatus: reported event=%s conclusion=%s version=%s repos=%d", event, conclusion, cfg.RCVersion, len(repos))
}
