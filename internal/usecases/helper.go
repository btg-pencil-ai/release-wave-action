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
