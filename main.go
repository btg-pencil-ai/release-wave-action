package main

import (
	"context"

	"release-candidate/internal/configs"
	"release-candidate/internal/usecases"
	"release-candidate/internal/usecases/githubrepo"
	utils "release-candidate/internal/utils"
)

func main() {

	// ctx := context.Background()
	config, _ := configs.Variables()

	l := utils.NewLogger(config.LogLevel)
	l.Info("Starting release candidate process")
	l.Info("RCVersion: %s", config.RCVersion)

	if err := utils.RcVersionValidate(l, config.RCVersion); err != nil {
		l.Fatal("Error validating RC version: %v", err)
	}

	config.RCBranch = "rc/" + config.RCVersion

	githubClient, err := utils.CreateGitHubClient(config)
	if err != nil {
		l.Fatal("Error creating GitHub client: %v", err)
	}

	switch config.UseCase {
	case "Release-Creation":
		l.Info("Release-Creation use case")
		usecases.ReleaseCreationUseCase(context.Background(), l, githubClient, config)
	case "Production-Release":
		l.Info("Production-Release use case")
		usecases.ProductionReleaseUseCase(context.Background(), l, githubClient, config)
	case "Main-To-Epic-Sync":
		l.Info("Main-To-Epic-Sync use case")
		githubRepo := githubrepo.NewGithubRepo(githubClient, l)
		// repoList is nil because it will be fetched later from the github repo
		usecases.MainToEpicSyncUseCase(context.Background(), l, githubRepo, config, nil)
	default:
		l.Fatal("Invalid use case")

	}

}
