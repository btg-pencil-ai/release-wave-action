package rc

import (
	"github.com/sethvargo/go-githubactions"
)

type Config struct {
	Owner             string
	Token             string
	AppID             string
	InstallationID    string
	PrivateKey        string
	RCVersion         string
	ProductionBranch  string
	DevelopmentBranch string
	PRTitle           string
	PRBody            string
}

func Variables() Config {

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

	prTitle := githubactions.GetInput("pr_title")
	if prTitle == "" {
		githubactions.Fatalf("pr_title is required")
	}

	prBody := githubactions.GetInput("pr_body")
	if prBody == "" {
		githubactions.Fatalf("pr_body is required")
	}
	return Config{
		Owner:             owner,
		Token:             token,
		AppID:             appID,
		PrivateKey:        privateKey,
		InstallationID:    installationId,
		RCVersion:         rcVersion,
		ProductionBranch:  productionBranch,
		DevelopmentBranch: developmentBranch,
		PRTitle:           prTitle,
		PRBody:            prBody,
	}
}
