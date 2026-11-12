package usecases

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"release-candidate/internal/configs"
	"release-candidate/internal/utils"
)

// syncStatusPayload mirrors hydra-controller's ProductionSyncPRsPayload (POST /production/sync-prs).
// Keep the JSON tags in sync across the two repos.
type syncStatusPayload struct {
	Event       string             `json:"event"` // workflow_start | workflow_completed | workflow_failed
	RcVersion   string             `json:"rc_version"`
	SyncTrigger string             `json:"sync_trigger,omitempty"`
	Epics       []string           `json:"epics,omitempty"` // the epics input as given; sent on every call
	RunID       int64              `json:"run_id,omitempty"`
	RunAttempt  int                `json:"run_attempt,omitempty"`
	GithubRepo  string             `json:"github_repo,omitempty"`
	TriggeredBy string             `json:"triggered_by,omitempty"`
	Conclusion  string             `json:"conclusion,omitempty"`
	Error       string             `json:"error,omitempty"`
	Results     []syncStatusResult `json:"results,omitempty"`
}

// syncStatusResult is one sync PR that was opened.
type syncStatusResult struct {
	Epic     string `json:"epic"`
	Repo     string `json:"repo"`
	PRURL    string `json:"pr_url"`
	Conflict bool   `json:"conflict,omitempty"`
}

// openedSyncPRs keeps the PRs that were actually opened (they have a link). Results with no PR,
// such as "No commits between" or a failed create, are left out.
func openedSyncPRs(prResults []SyncToEpicPRResult) []syncStatusResult {
	var opened []syncStatusResult
	for _, r := range prResults {
		if r.PRURL == "" {
			continue
		}
		opened = append(opened, syncStatusResult{Epic: r.Epic, Repo: r.Repo, PRURL: r.PRURL, Conflict: r.HasConflicts})
	}
	return opened
}

// PostSyncStatus reports this sync run's status to Hydra. Best-effort: it logs and returns on any
// error and never changes the run. event is workflow_start | workflow_completed | workflow_failed;
// errMsg is why the run failed; prResults are the PR results so far (only opened PRs are sent).
func PostSyncStatus(l utils.LogInterface, cfg *configs.Config, event, conclusion, errMsg string, prResults []SyncToEpicPRResult) {
	if cfg.HydraWebhookURL == "" {
		l.Info("PostSyncStatus: hydra webhook URL not set, skipping (event=%s)", event)
		return
	}
	runID, _ := strconv.ParseInt(os.Getenv("GITHUB_RUN_ID"), 10, 64)
	runAttempt, _ := strconv.Atoi(os.Getenv("GITHUB_RUN_ATTEMPT"))

	payload := syncStatusPayload{
		Event:       event,
		RcVersion:   cfg.RCVersion,
		SyncTrigger: cfg.SyncTrigger,
		Epics:       cfg.Epics,
		RunID:       runID,
		RunAttempt:  runAttempt,
		GithubRepo:  os.Getenv("GITHUB_REPOSITORY"),
		TriggeredBy: cfg.TriggeredBy,
		Conclusion:  conclusion,
		Error:       errMsg,
		Results:     openedSyncPRs(prResults),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		l.Error("PostSyncStatus: marshal failed: %v", err)
		return
	}

	endpoint := hydraEndpoint(cfg.HydraWebhookURL, "/production/sync-prs")
	l.Info("PostSyncStatus: POST %s event=%s prs=%d", endpoint, event, len(payload.Results))

	// Signed only when a secret is configured
	status, respBody, err := postToHydra(endpoint, cfg.HydraWebhookSecret, cfg.HydraWebhookSecret != "", body)
	var buildErr errHydraRequest
	switch {
	case errors.As(err, &buildErr):
		l.Error("PostSyncStatus: build request failed: %v", buildErr.err)
		return
	case err != nil && status == 0:
		l.Error("PostSyncStatus: request failed (event=%s): %v", event, err)
		return
	case status != http.StatusOK:
		l.Error("PostSyncStatus: webhook returned status %d (event=%s): %s", status, event, string(respBody))
		return
	}
	l.Info("PostSyncStatus: reported event=%s conclusion=%s version=%s prs=%d", event, conclusion, cfg.RCVersion, len(payload.Results))
}

// failSync reports the failure, with the PRs opened so far, to Hydra, then exits.
func failSync(l utils.LogInterface, cfg *configs.Config, prResults []SyncToEpicPRResult, format string, args ...interface{}) {
	PostSyncStatus(l, cfg, "workflow_failed", "failure", fmt.Sprintf(format, args...), prResults)
	l.Fatal(format, args...)
}
