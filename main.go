package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"release-candidate/rc"
	"strconv"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v66/github"
	"github.com/sethvargo/go-githubactions"
)

func main() {
	ctx := context.Background()
	variables := rc.Variables()

	client, err := createGitHubClient(variables)
	if err != nil {
		log.Fatalf("Error creating GitHub client: %v", err)
	}

	repoList, err := rc.ListRepositories(ctx, client, variables.Owner, variables.ExcludeRepositories)
	if err != nil {
		log.Fatalf("Error listing repositories: %v", err)
	}

	if err := rc.RcValidate(variables.RCVersion); err != nil {
		log.Fatalf("Error validating RC version: %v", err)
	}

	prList, prUrls := processRepositories(ctx, client, variables, repoList)

	slackPayload, err := rc.SlackPayloadBuilder(variables.RCVersion, prList)
	if err != nil {
		log.Fatalf("Error building Slack payload: %v", err)
	}

	log.Printf("PR details:\n%v", strings.Join(prUrls, "\n"))
	githubactions.SetOutput("pr_urls", strings.Join(prUrls, "\n"))
	githubactions.SetOutput("slack_payload", slackPayload)
}

func createGitHubClient(variables rc.Config) (*github.Client, error) {
	if variables.AppID != "" && variables.PrivateKey != "" && variables.InstallationID != "" {
		log.Println("Using GitHub App authentication")
		appIDInt, err := strconv.ParseInt(variables.AppID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting appID: %v", err)
		}
		installationIDInt, err := strconv.ParseInt(variables.InstallationID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting installationID: %v", err)
		}
		itr, err := ghinstallation.New(http.DefaultTransport, appIDInt, installationIDInt, []byte(variables.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("error creating GitHub installation transport: %v", err)
		}
		return github.NewClient(&http.Client{Transport: itr}), nil
	} else if variables.Token != "" {
		log.Println("Using personal access token authentication")
		return github.NewClient(nil).WithAuthToken(variables.Token), nil
	} else {
		return nil, fmt.Errorf("no authentication method provided")
	}
}

func processRepositories(ctx context.Context, client *github.Client, variables rc.Config, repoList []string) ([]map[string]interface{}, []string) {
	var prList []map[string]interface{}
	var prUrls []string
	rcBranch := "rc/" + variables.RCVersion

	for _, repo := range repoList {
		if err := rc.CreateRcBranch(ctx, client, variables.Owner, repo, variables.ProductionBranch, variables.RCVersion); err != nil {
			log.Fatalf("Error creating branch for repo %s: %v", repo, err)
		}

		conflictMergePr, err := rc.MergeRcBranch(ctx, client, variables.Owner, repo, variables.DevelopmentBranch, variables.RCVersion)
		if err != nil {
			log.Fatalf("Error merging branch for repo %s: %v", repo, err)
		}

		prUrl, prError, err := rc.CreatePullRequest(ctx, client, variables.Owner, repo, rcBranch, variables.ProductionBranch, variables.PRTitle, variables.PRBody)
		if err != nil {
			log.Fatalf("Error creating PR for repo %s: %v", repo, err)
		}

		prMap := map[string]interface{}{
			"repo":            repo,
			"url":             prUrl,
			"error":           prError,
			"conflictMergePr": conflictMergePr,
		}
		prList = append(prList, prMap)
		prDetails := fmt.Sprintf("%s:%s:%s\n", repo, prUrl, prError)
		prUrls = append(prUrls, prDetails)
	}

	return prList, prUrls
}
