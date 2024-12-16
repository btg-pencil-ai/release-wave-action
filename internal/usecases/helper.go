package usecases

import (
	"context"
	"fmt"
	"release-candidate/internal/configs"
	"release-candidate/internal/usecases/githubrepo"
	"release-candidate/internal/utils"
)

func ReleasePrCreator(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, variables *configs.Config, repoList []string) ([]map[string]interface{}, []string, error) {
	var prList []map[string]interface{}
	var prUrls []string

	for _, repo := range repoList {
		if err := githubRepo.CreateBranch(ctx, variables.Owner, repo, variables.ProductionBranch, variables.RCBranch); err != nil {
			l.Error("Error creating branch for repo %s: %v", repo, err)
			return nil, nil, fmt.Errorf("error creating branch for repo %s: %v", repo, err)
		}

		conflictMergePr, err := githubRepo.MergeBranchWithConflictPr(ctx, variables.Owner, repo, variables.DevelopmentBranch, variables.RCBranch)
		if err != nil {
			l.Error("Error merging branch for repo %s: %v", repo, err)
			return nil, nil, fmt.Errorf("error merging branch for repo %s: %v", repo, err)
		}

		prUrl, prError, err := githubRepo.CreatePullRequest(ctx, variables.Owner, repo, variables.RCBranch, variables.ProductionBranch, variables.PRTitle, variables.PRBody)
		if err != nil {
			l.Error("Error creating PR for repo %s: %v", repo, err)
			return nil, nil, fmt.Errorf("error creating PR for repo %s: %v", repo, err)
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

	return prList, prUrls, nil
}

func PreReleaseCheck(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, variables *configs.Config, repoList []string) (activePrs []map[string]interface{}, err error) {
	for _, repo := range repoList {
		prs, err := githubRepo.ListPullRequests(ctx, variables.Owner, repo, variables.RCBranch, variables.ProductionBranch, "open")
		if err != nil {
			l.Error("Error listing PRs for repo %s: %v", repo, err)
			return nil, fmt.Errorf("error listing PRs for repo %s: %v", repo, err)
		}
		activePrs = append(activePrs, prs...)
	}
	if len(activePrs) > 0 {
		l.Error("There are active PRs: %v", activePrs)
		return activePrs, fmt.Errorf("there are active PRs: %v", activePrs)
	}
	return activePrs, nil
}

func ProductionWorkflowDispatch(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, variables *configs.Config, repoList []string) (slackpayload string, err error) {
	payload := map[string]interface{}{
		"environment":     variables.Environment,
		"release_version": variables.RCVersion,
	}
	prodWorkflowFilter := "prod-release.*"

	for _, repo := range repoList {
		workflows, err := githubRepo.ListWorkFlowsByRepoFileFilter(ctx, variables.Owner, repo, prodWorkflowFilter)
		if err != nil {
			l.Error("Error listing workflows for repo %s: %v", repo, err)
			return "", fmt.Errorf("error listing workflows for repo %s: %v", repo, err)
		}

		for _, workflow := range workflows {
			err = githubRepo.CreateWorkflowDispatchEventByID(ctx, variables.Owner, repo, variables.ProductionBranch, workflow.ID, payload)
			if err != nil {
				l.Error("Error dispatching workflow for repo %s: %v", repo, err)
				return "", fmt.Errorf("error dispatching workflow for repo %s: %v", repo, err)
			}
		}
		l.Info("Production workflow dispatched for repo %s to Environment %s", repo, variables.Environment)
	}
	slackpayload, err = utils.ProductionWorkflowDispatchSlackPayloadBuilder(variables.RCVersion, repoList, variables.Environment)
	if err != nil {
		l.Error("Error building slack payload: %v", err)
		return "", fmt.Errorf("error building slack payload: %v", err)
	}
	l.Info("Production workflow dispatched")

	return slackpayload, nil
}
