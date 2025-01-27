package configs

import (
	"github.com/sethvargo/go-githubactions"
)

type Config struct {
	LogLevel                       string
	UseCase                        string
	Environment                    string
	Owner                          string
	Token                          string
	AppID                          string
	InstallationID                 string
	PrivateKey                     string
	RCVersion                      string
	ProductionBranch               string
	DevelopmentBranch              string
	PRTitle                        string
	PRBody                         string
	IncludeRepositories            string
	ExcludeRepositories            string
	ExcludeProdReleaseRepositories string
	RCBranch                       string
}

func Variables() (*Config, error) {

	logLevel := githubactions.GetInput("log_level")
	if logLevel == "" {
		logLevel = "info"
	}

	owner := githubactions.GetInput("owner")
	if owner == "" {
		githubactions.Fatalf("owner is required")
	}

	token := githubactions.GetInput("github_token")
	if token != "" {
		githubactions.AddMask(token)
	}

	appID := githubactions.GetInput("app_id")
	if appID != "" {
		githubactions.AddMask(appID)
	}

	installationId := githubactions.GetInput("installation_id")
	if installationId != "" {
		githubactions.AddMask(installationId)
	}

	privateKey := githubactions.GetInput("private_key")
	if privateKey != "" {

		githubactions.AddMask(privateKey)
	}

	rcVersion := githubactions.GetInput("rc_version")
	if rcVersion == "" {
		githubactions.Fatalf("rc_version is required")
	}

	productionBranch := githubactions.GetInput("production_branch")
	if productionBranch == "" {
		githubactions.Fatalf("production_branch is required")
	}

	developmentBranch := githubactions.GetInput("development_branch")
	if developmentBranch == "" {
		githubactions.Fatalf("development_branch is required")
	}

	usecase := githubactions.GetInput("use_case")
	environment := githubactions.GetInput("environment")
	prTitle := githubactions.GetInput("pr_title")
	if prTitle == "" {
		prTitle = "Release"
	}
	prBody := githubactions.GetInput("pr_body")
	if prBody == "" {
		prBody = "Release"
	}

	excludeRepositories := githubactions.GetInput("exclude_repositories")
	includeRepositories := githubactions.GetInput("include_repositories")

	return &Config{
		LogLevel:            logLevel,
		UseCase:             usecase,
		Owner:               owner,
		Token:               token,
		AppID:               appID,
		PrivateKey:          privateKey,
		InstallationID:      installationId,
		RCVersion:           rcVersion,
		ProductionBranch:    productionBranch,
		DevelopmentBranch:   developmentBranch,
		PRTitle:             prTitle,
		PRBody:              prBody,
		Environment:         environment,
		IncludeRepositories: includeRepositories,
		ExcludeRepositories: excludeRepositories,
	}, nil
}
