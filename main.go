package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-github/v66/github"

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

	var client *github.Client
	if appID != "" && privateKey != "" && installationID != "" {
		appIDInt, err := strconv.ParseInt(appID, 10, 64)
		if err != nil {
			log.Fatalf("Error converting appID: %v", err)
		}
		installationIDInt, err := strconv.ParseInt(installationID, 10, 64)
		if err != nil {
			log.Fatalf("Error converting installationID: %v", err)
		}
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appIDInt, installationIDInt, privateKey)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		client = github.NewClient(&http.Client{Transport: itr})
	} else if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		log.Fatalf("Error: No authentication method provided")
	}

	repoList, err := rc.ListRepositories(ctx, client, owner)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	err = rc.RcValidate(rcVersion)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	var prList []string
	for _, repo := range repoList {
		err := rc.CreateRcBranch(ctx, client, owner, repo, productionBranch, rcVersion)
		if err != nil {
			log.Fatalf("Error creating branch: %v", err)
		}
		err = rc.MergeRcBranch(ctx, client, owner, repo, developmentBranch, rcVersion)
		if err != nil {
			log.Fatalf("Error merging branch: %v", err)
		}
		prUrl, err := rc.CreateRcPullRequest(ctx, client, owner, repo, productionBranch, rcVersion, prTitle, prBody)
		if err != nil {
			log.Fatalf("Error creating PR: %v", err)
		}
		prList = append(prList, prUrl)
	}

	log.Printf("PRs created successfully: %v", prList)
	fmt.Printf("::set-env name=PR_URLS::%v", prList)

}
