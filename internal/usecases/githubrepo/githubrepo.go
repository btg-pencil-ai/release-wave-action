package githubrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"release-candidate/internal/utils"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
)

type GitHubWebApis interface {
	CreateBranch(ctx context.Context, owner string, repo string, baseBranch string, newBranch string) error
	CreatePullRequest(ctx context.Context, owner string, repo string, fromBranch string, toBranch string, title string, body string) (prUrl string, prError string, hasConflicts bool, err error)
	ListRepositories(ctx context.Context, owner string, includeRepositories string, excludeRepositories string) ([]string, error)
	CreateRepositoryDispatches(ctx context.Context, owner string, repo string, eventType string, clientPayload map[string]interface{}) error
	ListWorkFlowsByRepoFileFilter(ctx context.Context, owner string, repo string, fileFilterRegex string) ([]RespWorkflow, error)
	CreateWorkflowDispatchEventByID(ctx context.Context, owner string, repo string, workflowID int64, clientPayload map[string]interface{}) error
	ListEpicBranches(ctx context.Context, owner string, repo string) ([]string, error)
	DeleteBranch(ctx context.Context, owner string, repo string, branchName string) error
	ClosePullRequest(ctx context.Context, owner string, repo string, prNumber int, comment string) error
	ListOpenPullRequestsByBase(ctx context.Context, owner string, repo string, baseBranch string) ([]*github.PullRequest, error)
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

func (g GithubRepo) CreatePullRequest(ctx context.Context, owner string, repo string, fromBranch string, toBranch string, title string, body string) (prUrl string, prError string, hasConflicts bool, err error) {
	prInfo := &github.NewPullRequest{
		Title: github.String(title),
		Body:  github.String(body),
		Head:  github.String(fromBranch),
		Base:  github.String(toBranch),
	}
	pr, resp, err := g.client.PullRequests.Create(ctx, owner, repo, prInfo)
	if err != nil {
		if resp != nil && resp.StatusCode == 422 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			var responseBody map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &responseBody); err != nil {
				g.l.Error("Error unmarshalling response body: %v", err)
				return "", "", false, fmt.Errorf("error unmarshalling response body: %v", err)
			}
			if errors, ok := responseBody["errors"].([]interface{}); ok && len(errors) > 0 {
				if message, ok := errors[0].(map[string]interface{})["message"].(string); ok {
					g.l.Error("Response message: %s", message)
					prError = message
				} else {
					g.l.Error("Response body: %s", string(bodyBytes))
				}
			} else {
				g.l.Error("Response body: %s", string(bodyBytes))
			}
		} else {
			g.l.Error("Error creating PR: %v", err)
			return "", "", false, fmt.Errorf("error %s creating PR: %v", resp.Status, err)
		}
	} else {
		g.l.Info("Created PR for branch %s on Repo %s", toBranch, repo)
	}

	if pr != nil {
		prUrl = pr.GetHTMLURL()
		
		// Poll to check for merge conflicts.
		// GitHub calculates PR mergeability asynchronously in the background.
		// When a PR is first created, prCheck.Mergeable is often nil while GitHub computes it.
		// This loop polls the API until Mergeable is no longer nil (meaning the calculation is done),
		// or until maxRetries is reached.
		maxRetries := 5
		for attempt := 1; attempt <= maxRetries; attempt++ {
			time.Sleep(2 * time.Second) // Wait for GitHub to calculate mergeability
			
			prCheck, _, checkErr := g.client.PullRequests.Get(ctx, owner, repo, pr.GetNumber())
			if checkErr != nil {
				g.l.Error("Error checking PR mergeability: %v", checkErr)
				break
			}
			
			if prCheck.Mergeable != nil {
				hasConflicts = !*prCheck.Mergeable
				if hasConflicts {
					g.l.Warn("PR %d has merge conflicts", pr.GetNumber())
				}
				break
			}
			
			if attempt == maxRetries {
				g.l.Warn("Could not determine mergeability for PR %d after %d attempts", pr.GetNumber(), maxRetries)
			}
		}
	}

	if prUrl == "" {
		prs, _, err := g.client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
			Head: owner + ":" + fromBranch,
		})
		if err != nil {
			g.l.Error("Error getting PR: %v", err)
			return "", "", false, fmt.Errorf("error getting PR: %v", err)
		}
		if len(prs) > 0 {
			prUrl = prs[0].GetHTMLURL()
		}
		print(prUrl)
	}
	g.l.Info("PR URL: %s", prUrl)
	return prUrl, prError, hasConflicts, nil
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

// ListEpicBranches returns all branches matching epic-* pattern (case insensitive)
func (g GithubRepo) ListEpicBranches(ctx context.Context, owner string, repo string) ([]string, error) {
	var epicBranches []string
	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		// Epic branches are protected
		Protected: github.Bool(true),
	}

	for {
		branches, resp, err := g.client.Repositories.ListBranches(ctx, owner, repo, opts)
		if err != nil {
			g.l.Error("Error listing branches for %s: %v", repo, err)
			return nil, fmt.Errorf("error listing branches for %s: %v", repo, err)
		}

		for _, branch := range branches {
			branchName := branch.GetName()
			if strings.HasPrefix(strings.ToLower(branchName), "epic-") {
				epicBranches = append(epicBranches, branchName)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return epicBranches, nil
}

func (g GithubRepo) DeleteBranch(ctx context.Context, owner string, repo string, branchName string) error {
	resp, err := g.client.Git.DeleteRef(ctx, owner, repo, "refs/heads/"+branchName)
	if err != nil {
		// The branch may already be gone - GitHub can auto-delete the head branch
		// when its PR is closed/merged, or a previous run already removed it. In
		// that case DeleteRef returns 422 "Reference does not exist" (sometimes
		// 404). Treat it as a successful no-op so cleanup stays idempotent.
		if resp != nil && (resp.StatusCode == 422 || resp.StatusCode == 404) {
			g.l.Info("Branch %s in repo %s already gone (HTTP %d: %v) - likely auto-deleted with its PR; skipping intentionally", branchName, repo, resp.StatusCode, err)
			return nil
		}
		g.l.Error("Error deleting branch %s in repo %s: %v", branchName, repo, err)
		return fmt.Errorf("error deleting branch %s in repo %s: %v", branchName, repo, err)
	}
	g.l.Info("Deleted branch %s in repo %s", branchName, repo)
	return nil
}

func (g GithubRepo) ClosePullRequest(ctx context.Context, owner string, repo string, prNumber int, comment string) error {
	// Add comment
	if comment != "" {
		comment := &github.IssueComment{
			Body: github.String(comment),
		}
		_, _, err := g.client.Issues.CreateComment(ctx, owner, repo, prNumber, comment)
		if err != nil {
			g.l.Error("Error adding comment to PR %d in repo %s: %v", prNumber, repo, err)
			// Continue to close PR even if comment fails
		}
	}

	// Close PR
	state := "closed"
	prUpdate := &github.PullRequest{
		State: &state,
	}
	_, _, err := g.client.PullRequests.Edit(ctx, owner, repo, prNumber, prUpdate)
	if err != nil {
		g.l.Error("Error closing PR %d in repo %s: %v", prNumber, repo, err)
		return fmt.Errorf("error closing PR %d in repo %s: %v", prNumber, repo, err)
	}
	g.l.Info("Closed PR %d in repo %s", prNumber, repo)
	return nil
}

func (g GithubRepo) ListOpenPullRequestsByBase(ctx context.Context, owner string, repo string, baseBranch string) ([]*github.PullRequest, error) {
	opts := &github.PullRequestListOptions{
		Base:  baseBranch,
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allPRs []*github.PullRequest
	for {
		prs, resp, err := g.client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			g.l.Error("Error listing PRs for repo %s base %s: %v", repo, baseBranch, err)
			return nil, fmt.Errorf("error listing PRs: %v", err)
		}
		allPRs = append(allPRs, prs...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}
