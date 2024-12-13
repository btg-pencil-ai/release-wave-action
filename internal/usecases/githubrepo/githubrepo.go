package githubrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"release-candidate/internal/utils"
	"strings"

	"github.com/google/go-github/v66/github"
)

type GitHubWebApis interface {
	CreateBranch(ctx context.Context, owner string, repo string, baseBranch string, newBranch string) error
	CreatePullRequest(ctx context.Context, owner string, repo string, fromBranch string, toBranch string, title string, body string) (prUrl string, prError string, err error)
	MergeBranchWithConflictPr(ctx context.Context, owner string, repo string, baseBranch string, mergeBranch string) (mergeConflictPr string, err error)
	ListRepositories(ctx context.Context, owner string, includeRepositories string, excludeRepositories string) ([]string, error)
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
			Head: toBranch,
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

func (g GithubRepo) ListRepositories(ctx context.Context, owner string, includeRepositories string, excludeRepositories string) ([]string, error) {
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
		repos, _, err := g.client.Repositories.ListByOrg(ctx, owner, &github.RepositoryListByOrgOptions{})
		if err != nil {
			g.l.Error("Error listing repositories: %v", err)
			return nil, fmt.Errorf("error listing repositories: %v", err)
		}

		for _, repo := range repos {
			if !strings.Contains(excludeRepositories, repo.GetName()) {
				repoList = append(repoList, repo.GetName())
			}
		}
	}
	g.l.Info("Repositories: %v", repoList)
	return repoList, nil
}
