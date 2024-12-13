package usecases

import (
	"context"
	"fmt"
	"release-candidate/internal/configs"
	"release-candidate/internal/usecases/githubrepo"
	"release-candidate/internal/utils"

	"github.com/google/go-github/v66/github"
)

func ReleaseCreationUseCase(ctx context.Context, l utils.LogInterface, client *github.Client, cfg *configs.Config) {
	l.Info("Release-Creation use case")
	l.Info("RCVersion: %s", cfg.RCVersion)
	cfg.RCBranch = "rc/" + cfg.RCVersion
	err := utils.RcVersionValidate(l, cfg.RCVersion)
	if err != nil {
		l.Fatal("Error validating RC version: %v", err)
	}

	githubRepo := githubrepo.NewGithubRepo(client, l) // init githubRepo struct
	repoList, err := githubRepo.ListRepositories(ctx, cfg.Owner, cfg.IncludeRepositories, cfg.ExcludeRepositories)
	if err != nil {
		l.Fatal("Error listing repositories: %v", err)
	}

	// l.Info("Repo list: %v", repoList)

	prList, prUrls, err := ReleasePrCreator(ctx, l, githubRepo, cfg, repoList)
	if err != nil {
		l.Fatal("Error creating PR: %v", err)
	}

	slackPayload, err := utils.ReleasePrCreatorSlackPayloadBuilder(cfg.RCVersion, prList)
	if err != nil {
		l.Fatal("Error building slack payload: %v", err)
	}

	l.Info("PR details:\n%v", prUrls)
	// l.Info("Slack payload:\n%v", prList)
	fmt.Println(slackPayload)

}

func ProductionReleaseUseCase(ctx context.Context, l utils.LogInterface, client *github.Client, cfg *configs.Config) {
	l.Info("Production-Release use case")
}
