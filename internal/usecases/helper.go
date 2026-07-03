package usecases

import (
	"context"
	"fmt"
	"release-candidate/internal/configs"
	"release-candidate/internal/usecases/githubrepo"
	"release-candidate/internal/utils"
)

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
