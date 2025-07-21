package usecases

import (
	"context"
	"os"
	"release-candidate/internal/configs"
	"release-candidate/internal/usecases/githubrepo"
	"release-candidate/internal/utils"
	"strings"

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

func ReleaseCreationUseCase(ctx context.Context, l utils.LogInterface, client *github.Client, cfg *configs.Config) {
	l.Info("Release-Creation use case")

	githubRepo := githubrepo.NewGithubRepo(client, l) // init githubRepo struct
	repoList, err := githubRepo.ListRepositories(ctx, cfg.Owner, cfg.UseCase, cfg.IncludeRepositories, cfg.ExcludeRepositories, cfg.ExcludeProdReleaseRepositories)
	if err != nil {
		l.Fatal("Error listing repositories: %v", err)
	}

	prList, prUrls, err := ReleasePrCreator(ctx, l, githubRepo, cfg, repoList)
	if err != nil {
		l.Fatal("Error creating PR: %v", err)
	}

	slackPayload, err := utils.ReleasePrCreatorSlackPayloadBuilder(cfg.RCVersion, prList)
	if err != nil {
		l.Fatal("Error building slack payload: %v", err)
	}

	l.Info("PR details:\n%v", prUrls)
	safeSetOutput("pr_urls", strings.Join(prUrls, "\n"), l)
	safeSetOutput("slack_payload", slackPayload, l)

}

func ProductionReleaseUseCase(ctx context.Context, l utils.LogInterface, client *github.Client, cfg *configs.Config) {
	l.Info("Production-Release use case")

	githubRepo := githubrepo.NewGithubRepo(client, l) // init githubRepo struct
	repoList, err := githubRepo.ListRepositories(ctx, cfg.Owner, cfg.UseCase, cfg.IncludeRepositories, cfg.ExcludeRepositories, cfg.ExcludeProdReleaseRepositories)
	if err != nil {
		l.Fatal("Error listing repositories: %v", err)
	}
	var slackPayload string

	activePrs, err := PreReleaseCheck(ctx, l, githubRepo, cfg, repoList)
	if err != nil {
		slackPayload, err = utils.PreReleaseErrorSlackPayloadBuilder(cfg.RCVersion, activePrs)
		if err != nil {
			l.Fatal("Error building slack payload: %v", err)
		}
	} else {
		l.Info("Staring Production Pipeline Dispatch")
		slackPayload, err = ProductionWorkflowDispatch(ctx, l, githubRepo, cfg, repoList)
		if err != nil {
			l.Fatal("Error building slack payload: %v", err)
		}
	}

	safeSetOutput("slack_payload", slackPayload, l)
}
