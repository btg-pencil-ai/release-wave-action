package usecases

import (
	"context"
	"os"
	"release-candidate/internal/configs"
	"release-candidate/internal/usecases/githubrepo"
	"release-candidate/internal/utils"

	"github.com/google/go-github/v66/github"
	"github.com/sethvargo/go-githubactions"
)

// safeSetOutput sets GitHub Actions output only if running in GitHub Actions environment
func safeSetOutput(key, value string, l utils.LogInterface) {
	if os.Getenv("GITHUB_OUTPUT") != "" || os.Getenv("GITHUB_ACTIONS") == "true" {
		githubactions.SetOutput(key, value)
	} else {
		l.Info("Not in GitHub Actions environment, skipping output set for %s", key)
	}
}

func ProductionReleaseUseCase(ctx context.Context, l utils.LogInterface, client *github.Client, cfg *configs.Config) {
	l.Info("Production-Release use case")

	githubRepo := githubrepo.NewGithubRepo(client, l) // init githubRepo struct
	repoList, err := githubRepo.ListRepositories(ctx, cfg.Owner, cfg.UseCase, cfg.IncludeRepositories, cfg.ExcludeRepositories, cfg.ExcludeProdReleaseRepositories)
	if err != nil {
		l.Fatal("Error listing repositories: %v", err)
	}
	l.Info("repoList: %v", repoList)
	var slackPayload string

	// Pre-release check removed: the Hydra platform now ensures all RC -> production
	// PRs are merged before this use case runs, so checking for open PRs here is
	// redundant. Proceed directly to dispatching the production pipeline.
	l.Info("Starting Production Pipeline Dispatch")
	slackPayload, err = ProductionWorkflowDispatch(ctx, l, githubRepo, cfg, repoList)
	if err != nil {
		l.Fatal("Error building slack payload: %v", err)
	}
	safeSetOutput("slack_payload", slackPayload, l)

	if cfg.EnableMainToEpicSync {
		MainToEpicSyncUseCase(ctx, l, githubRepo, cfg, repoList)
	} else {
		l.Info("Main to Epic Sync is disabled, skipping sync")
	}
}

func MainToEpicSyncUseCase(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, cfg *configs.Config, repoList []string) {
	l.Info("Starting Main to Epic Sync")

	if len(repoList) == 0 {
		var err error
		repoList, err = githubRepo.ListRepositories(ctx, cfg.Owner, cfg.UseCase, cfg.IncludeRepositories, cfg.ExcludeRepositories, cfg.ExcludeProdReleaseRepositories)
		if err != nil {
			l.Fatal("Error listing repositories: %v", err)
		}
	}
	l.Info("repoList: %v", repoList)
	// Fetch active epics from Hydra webhook
	activeEpics, err := FetchHydraActiveEpics(l, cfg.HydraWebhookURL, cfg.HydraWebhookSecret)
	if err != nil {
		l.Fatal("Error fetching active epics: %v", err)
	}
	l.Info("activeEpics: %v", activeEpics)

	if len(activeEpics) > 0 {
		l.Info("Active epics: %v", activeEpics)

		// Find epic branches in all repos
		epicBranchResults, err := FindEpicBranchesInRepos(ctx, l, githubRepo, cfg.Owner, repoList, activeEpics)
		if err != nil {
			l.Fatal("Error finding epic branches: %v", err)
		}

		// Log results for each epic
		for _, epic := range activeEpics {
			reposWithBranch := GetReposWithEpicBranch(epicBranchResults, epic)
			if len(reposWithBranch) > 0 {
				l.Info("Epic '%s' has branches in repos: %v", epic, reposWithBranch)
			} else {
				l.Info("Epic '%s' has no matching branches in any repo", epic)
			}
		}

		// Cleanup old sync branches and PRs
		l.Info("Cleaning up old sync branches and PRs")
		if err := CleanupOldSyncBranches(ctx, l, githubRepo, cfg.Owner, cfg.RCVersion, epicBranchResults); err != nil {
			l.Fatal("Error cleaning up old sync branches and PRs: %v", err)
		}

		// Create sync branches for each epic in repos where epic branch exists
		syncResults, err := CreateSyncBranchesForEpics(ctx, l, githubRepo, cfg.Owner, cfg.ProductionBranch, cfg.RCVersion, epicBranchResults)

		// Log sync branch creation results
		for _, result := range syncResults {
			if result.Created {
				l.Info("Sync branch '%s' created in repo '%s' for epic '%s'", result.BranchName, result.Repo, result.Epic)
			} else {
				l.Error("Failed to create sync branch '%s' in repo '%s': %s", result.BranchName, result.Repo, result.Error)
			}
		}

		if err != nil {
			l.Fatal("Error creating sync branches: %v", err)
		}

		// Create PRs from sync branches to epic branches
		prResults, err := CreatePRsFromSyncToEpic(ctx, l, githubRepo, cfg.Owner, cfg.RCVersion, syncResults)

		// Log PR creation results
		prResultsByEpic := make(map[string][]map[string]interface{})
		for _, result := range prResults {
			if result.Created {
				l.Info("PR created: %s -> %s in repo '%s': %s", result.SyncBranch, result.EpicBranch, result.Repo, result.PRURL)
			} else {
				l.Error("Failed to create PR: %s -> %s in repo '%s': %s", result.SyncBranch, result.EpicBranch, result.Repo, result.Error)
			}

			prMap := map[string]interface{}{
				"repo":         result.Repo,
				"url":          result.PRURL,
				"error":        result.Error,
				"hasConflicts": result.HasConflicts,
			}
			prResultsByEpic[result.Epic] = append(prResultsByEpic[result.Epic], prMap)
		}

		if len(prResultsByEpic) > 0 {
			slackPayload, err := utils.MainToEpicSyncSlackPayloadBuilder(cfg.RCVersion, prResultsByEpic)
			if err != nil {
				l.Error("Error building sync slack payload: %v", err)
			} else {
				l.Info("Sync PR Slack Payload:\n%s", slackPayload) //Log for manual copying
				safeSetOutput("sync_pr_slack_payload", slackPayload, l)
			}
		}

		if err != nil {
			l.Fatal("Some PRs failed to create: %v", err)
		}

	}
}
