package githubrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"release-candidate/internal/utils"
	"strings"

	"github.com/google/go-github/v66/github"
)

type GitHubWebApis interface {
	CreateBranch(ctx context.Context, owner string, repo string, baseBranch string, newBranch string) error
	CreatePullRequest(ctx context.Context, owner string, repo string, fromBranch string, toBranch string, title string, body string) (prUrl string, prError string, err error)
	MergeBranchWithConflictPr(ctx context.Context, owner string, repo string, baseBranch string, mergeBranch string) (mergeConflictPr string, err error)
	ListRepositories(ctx context.Context, owner string, includeRepositories string, excludeRepositories string) ([]string, error)
	CreateRepositoryDispatches(ctx context.Context, owner string, repo string, eventType string, clientPayload map[string]interface{}) error
	ListPullRequests(ctx context.Context, owner string, repo string, fromBranch string, toBranch string, state string) ([]map[string]interface{}, error)
	ListWorkFlowsByRepoFileFilter(ctx context.Context, owner string, repo string, fileFilterRegex string) ([]RespWorkflow, error)
	CreateWorkflowDispatchEventByID(ctx context.Context, owner string, repo string, workflowID int64, clientPayload map[string]interface{}) error
}

type GithubRepo struct {
	client *github.Client
	l      utils.LogInterface
}

func NewGithubRepo(client *github.Client, logger utils.LogInterface) GithubRepo {
	return GithubRepo{
		client: client,
		l:      logger,
	}
}

func (g GithubRepo) CreateBranch(ctx context.Context, owner string, repo string, baseBranch string, newBranch string) error {
	ref, _, err := g.client.Git.GetRef(ctx, owner, repo, "refs/heads/"+baseBranch)
	if err != nil {
		g.l.Error("Error getting ref: %v", err)
		return fmt.Errorf("error getting ref: %v", err)
	}

	newRCBranchRef := &github.Reference{
		Ref:    github.String("refs/heads/" + newBranch),
		Object: &github.GitObject{SHA: ref.Object.SHA},
	}

	if _, _, err := g.client.Git.GetRef(ctx, owner, repo, "refs/heads/"+newBranch); err != nil {
		if _, _, err := g.client.Git.CreateRef(ctx, owner, repo, newRCBranchRef); err != nil {
			g.l.Error("Error creating branch %s on %s: %v", newBranch, repo, err)
			return fmt.Errorf("error creating branch %s on %s: %v", newBranch, repo, err)
		} else {
			g.l.Info("Created branch %s on %s", newBranch, repo)
		}
	}
	return nil
}

func (g GithubRepo) MergeBranchWithConflictPr(ctx context.Context, owner string, repo string, baseBranch string, mergeBranch string) (mergeConflictPr string, err error) {
	_, res, err := g.client.Repositories.Merge(ctx, owner, repo, &github.RepositoryMergeRequest{
		Base:          github.String(mergeBranch),
		Head:          github.String(baseBranch),
		CommitMessage: github.String(fmt.Sprintf("Merge branch '%s' into '%s' on %s", mergeBranch, baseBranch, repo)),
	})
	if err != nil {
		if res != nil && res.StatusCode == 409 {
			g.l.Error("Merge conflict: %v", err)
			prURL, _, err := g.CreatePullRequest(ctx, owner, repo, baseBranch, mergeBranch, "Merge conflict to "+mergeBranch, "Merge conflict to "+mergeBranch)
			if err != nil {
				g.l.Error("Error creating merge PR: %v", err)
				return "", fmt.Errorf("error creating merge PR: %v", err)
			}
			return prURL, nil
		}
		g.l.Error("Error merging branch: %v", err)
		return "", fmt.Errorf("error merging branch: %v", err)
	}
	g.l.Info("Merged branch %s into %s on %s", baseBranch, mergeBranch, repo)
	return "", nil
}

func (g GithubRepo) CreatePullRequest(ctx context.Context, owner string, repo string, fromBranch string, toBranch string, title string, body string) (prUrl string, prError string, err error) {
	prInfo := &github.NewPullRequest{
		Title: github.String(title),
		Body:  github.String(body),
		Head:  github.String(fromBranch),
		Base:  github.String(toBranch),
	}
	pr, resp, err := g.client.PullRequests.Create(ctx, owner, repo, prInfo)
	if err != nil {
		if resp != nil && resp.StatusCode == 422 {
			body, _ := io.ReadAll(resp.Body)
			var responseBody map[string]interface{}
			if err := json.Unmarshal(body, &responseBody); err != nil {
				g.l.Error("Error unmarshalling response body: %v", err)
				return "", "", fmt.Errorf("error unmarshalling response body: %v", err)
			}
			if errors, ok := responseBody["errors"].([]interface{}); ok && len(errors) > 0 {
				if message, ok := errors[0].(map[string]interface{})["message"].(string); ok {
					g.l.Error("Response message: %s", message)
					prError = message
				} else {
					g.l.Error("Response body: %s", body)
				}
			} else {
				g.l.Error("Response body: %s", body)
			}
		} else {
			g.l.Error("Error creating PR: %v", err)
			return "", "", fmt.Errorf("error %s creating PR: %v", resp.Status, err)
		}
	} else {
		g.l.Info("Created PR for branch %s on Repo %s", toBranch, repo)
	}

	prUrl = pr.GetHTMLURL()
	if prUrl == "" {
		prs, _, err := g.client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
			Head: owner + ":" + fromBranch,
		})
		if err != nil {
			g.l.Error("Error getting PR: %v", err)
			return "", "", fmt.Errorf("error getting PR: %v", err)
		}
		if len(prs) > 0 {
			prUrl = prs[0].GetHTMLURL()
		}
		print(prUrl)
	}
	g.l.Info("PR URL: %s", prUrl)
	return prUrl, prError, nil
}

func (g GithubRepo) ListRepositories(ctx context.Context, owner string, usecase string, includeRepositories string, excludeRepositories string, excludeProdReleaseRepostories string) ([]string, error) {
	var repoList []string
	if includeRepositories != "" {
		repoList = strings.Split(includeRepositories, ",")
		for _, repo := range repoList {
			_, _, err := g.client.Repositories.Get(ctx, owner, repo)
			if err != nil {
				g.l.Error("Error getting repository: %v", err)
				return nil, fmt.Errorf("error getting repository: %v", err)
			}
		}

	} else {
		repos, _, err := g.client.Repositories.ListByOrg(ctx, owner, &github.RepositoryListByOrgOptions{
			ListOptions: github.ListOptions{
				PerPage: 500,
			},
		})
		if err != nil {
			g.l.Error("Error listing repositories: %v", err)
			return nil, fmt.Errorf("error listing repositories: %v", err)
		}

		for _, repo := range repos {
			if repo.GetArchived() {
				continue
			}
			repoName := repo.GetName()
			if strings.Contains(excludeRepositories, repoName) {
				continue
			}
			if usecase == "Production-Release" && strings.Contains(excludeProdReleaseRepostories, repoName) {
				continue
			}
			repoList = append(repoList, repoName)
		}
	}
	g.l.Info("Repositories: %v", repoList)
	return repoList, nil
}

func (g GithubRepo) CreateRepositoryDispatches(ctx context.Context, owner string, repo string, eventType string, clientPayload map[string]interface{}) error {

	payloadBytes, err := json.Marshal(clientPayload)
	if err != nil {
		g.l.Error("Error marshalling client payload: %v", err)
		return fmt.Errorf("error marshalling client payload: %v", err)
	}
	payload := json.RawMessage(payloadBytes)

	dispatchOptions := &github.DispatchRequestOptions{
		EventType:     eventType,
		ClientPayload: &payload,
	}
	_, _, err = g.client.Repositories.Dispatch(ctx, owner, repo, *dispatchOptions)
	if err != nil {
		g.l.Error("Error dispatching event: %v", err)
		return fmt.Errorf("error dispatching event: %v", err)
	}
	g.l.Info("Dispatched event %s to %s", eventType, repo)

	return nil
}

func (g GithubRepo) ListPullRequests(ctx context.Context, owner string, repo string, fromBranch string, toBranch string, state string) ([]map[string]interface{}, error) {
	prs, _, err := g.client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
		Base:  toBranch,
		Head:  owner + "/" + fromBranch,
		State: state,
	})

	if err != nil {
		g.l.Error("Error listing PRs: %v", err)
		return nil, fmt.Errorf("error listing PRs: %v", err)
	}

	response := make([]map[string]interface{}, 0, len(prs))
	for _, pr := range prs {
		if pr.Head.Ref != nil {
			prHeadBranch:= *pr.Head.Ref
			if prHeadBranch == fromBranch {
				g.l.Info("Listing pr from current rc branch: %v",prHeadBranch)
				response = append(response, map[string]interface{}{
					"url":        pr.GetHTMLURL(),
					"id":         pr.GetID(),
					"repository": repo,
					"state":      pr.GetState(),
				})
			} else{
				g.l.Info("skipping pr from non current rc branch %v",prHeadBranch)
			}

		} else {
			g.l.Info("Listing pr as Head info can't be obtained")
			response = append(response, map[string]interface{}{
				"url":        pr.GetHTMLURL(),
				"id":         pr.GetID(),
				"repository": repo,
				"state":      pr.GetState(),
			})
		}
	}

	return response, nil
}

func (g GithubRepo) ListWorkFlowsByRepoFileFilter(ctx context.Context, owner string, repo string, fileFilterRegex string) ([]RespWorkflow, error) {
	workflowList, _, err := g.client.Actions.ListWorkflows(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		g.l.Error("Error listing workflows: %v", err)
		return nil, fmt.Errorf("error listing workflows: %v", err)
	}

	response := make([]RespWorkflow, 0, len(workflowList.Workflows))
	for _, workflow := range workflowList.Workflows {
		response = append(response, RespWorkflow{
			ID:   workflow.GetID(),
			Name: workflow.GetName(),
			URL:  workflow.GetURL(),
			Path: workflow.GetPath(),
			Repo: repo,
		})
	}
	filteredWorkflows := make([]RespWorkflow, 0)
	re, err := regexp.Compile(fileFilterRegex)
	if err != nil {
		g.l.Error("Error compiling regex: %v", err)
		return nil, fmt.Errorf("error compiling regex: %v", err)
	}
	for _, workflow := range response {
		if re.MatchString(workflow.Path) {
			filteredWorkflows = append(filteredWorkflows, workflow)
		}
	}

	fmt.Printf("Workflows: %v", filteredWorkflows)

	return filteredWorkflows, nil
}

func (g GithubRepo) CreateWorkflowDispatchEventByID(ctx context.Context, owner string, repo string, ref string, workflowID int64, clientPayload map[string]interface{}) error {

	dispatchOptions := &github.CreateWorkflowDispatchEventRequest{
		Ref:    "refs/heads/" + ref,
		Inputs: clientPayload,
	}

	_, err := g.client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, workflowID, *dispatchOptions)
	if err != nil {
		g.l.Error("Error dispatching event: %v", err)
		return fmt.Errorf("error dispatching event: %v", err)
	}
	g.l.Info("Dispatched event to %s", repo)

	return nil
}
