package main

import (
	"context"

	"release-candidate/internal/configs"
	utils "release-candidate/internal/utils"
	"release-candidate/internal/usecases"
)



func main() {

	// ctx := context.Background()
	config, _ := configs.Variables()

	l := utils.NewLogger(config.LogLevel)

	l.Info("Starting release candidate process")
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
	default:
		l.Fatal("Invalid use case")

	}

}
