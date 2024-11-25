package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v66/github"
	"github.com/sethvargo/go-githubactions"

	"net/http"
	"release-candidate/rc"
	"strconv"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

func main() {

	ctx := context.Background()
	variables := rc.Variables() // This is a function from rc/configurations.go

	owner := variables.Owner
	token := variables.Token
	appID := variables.AppID
	privateKey := variables.PrivateKey
	installationID := variables.InstallationID
	rcVersion := variables.RCVersion
	productionBranch := variables.ProductionBranch
	developmentBranch := variables.DevelopmentBranch
	prTitle := variables.PRTitle
	prBody := variables.PRBody
	excludeRepos := variables.ExcludeRepositories

	var client *github.Client
	if appID != "" && privateKey != "" && installationID != "" {
		log.Println("Using GitHub App authentication")
		appIDInt, err := strconv.ParseInt(appID, 10, 64)
		if err != nil {
			log.Fatalf("Error converting appID: %v", err)
		}
		installationIDInt, err := strconv.ParseInt(installationID, 10, 64)
		if err != nil {
			log.Fatalf("Error converting installationID: %v", err)
		}
		itr, err := ghinstallation.New(http.DefaultTransport, appIDInt, installationIDInt, []byte(privateKey))
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		client = github.NewClient(&http.Client{Transport: itr})
	} else if token != "" {
		log.Println("Using personal access token authentication")
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		log.Fatalf("Error: No authentication method provided")
	}

	repoList, err := rc.ListRepositories(ctx, client, owner, excludeRepos)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	err = rc.RcValidate(rcVersion)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	var prList []map[string]interface{}
	var prUrls []string
	for _, repo := range repoList {
		err := rc.CreateRcBranch(ctx, client, owner, repo, productionBranch, rcVersion)
		if err != nil {
			log.Fatalf("Error creating branch: %v", err)
		}
		err = rc.MergeRcBranch(ctx, client, owner, repo, developmentBranch, rcVersion)
		if err != nil {
			log.Fatalf("Error merging branch: %v", err)
		}
		prUrl, prError, err := rc.CreateRcPullRequest(ctx, client, owner, repo, productionBranch, rcVersion, prTitle, prBody)
		if err != nil {
			log.Fatalf("Error creating PR: %v", err)
		}

		prMap := map[string]interface{}{
			"repo":  repo,
			"url":   prUrl,
			"error": prError,
		}
		prList = append(prList, prMap)
		prDetails := fmt.Sprintf("%s:%s:%s\n", repo, prUrl, prError)
		prUrls = append(prUrls, prDetails)
	}

	slackPayload, err := rc.SlackPayloadBuilder(rcVersion, prList)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Printf("Pr details:\n%v", strings.Join(prUrls, "\n"))
	githubactions.SetOutput("pr_urls", strings.Join(prUrls, "\n"))
	githubactions.SetOutput("slack_payload", slackPayload)

}
