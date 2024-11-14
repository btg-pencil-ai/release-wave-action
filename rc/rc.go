package rc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"

	"github.com/google/go-github/v66/github"
)

var (
	infoLogger  = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	errorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)
)

func CreateRcBranch(ctx context.Context, client *github.Client, owner, repo, branch, rcVersion string) error {
	rcBranch := fmt.Sprintf("rc/%s", rcVersion)
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
	if err != nil {
		errorLogger.Printf("Error getting ref: %v", err)
		return fmt.Errorf("error getting ref: %v", err)
	}

	newRCBranchRef := &github.Reference{
		Ref: github.String("refs/heads/" + rcBranch),
		Object: &github.GitObject{
			SHA: ref.Object.SHA,
		},
	}

	_, _, err = client.Git.GetRef(ctx, owner, repo, "refs/heads/"+rcBranch)
	if err != nil {
		if _, _, err := client.Git.CreateRef(ctx, owner, repo, newRCBranchRef); err != nil {
			errorLogger.Printf("Error creating ref: %v", err)
			return fmt.Errorf("error creating ref: %v", err)
		}
	}
	infoLogger.Printf("Created branch %s on %s", rcBranch, repo)
	return nil
}

func MergeRcBranch(ctx context.Context, client *github.Client, owner, repo, branch, rcVersion string) error {
	rcBranch := fmt.Sprintf("rc/%s", rcVersion)
	merge, _, err := client.Repositories.Merge(ctx, owner, repo, &github.RepositoryMergeRequest{
		Base:          github.String(rcBranch),
		Head:          github.String(branch),
		CommitMessage: github.String(fmt.Sprintf("Merge branch '%s' into '%s' on %s", rcBranch, branch, repo)),
	})
	if err != nil {
		errorLogger.Printf("Error merging branch: %v", err)
		return fmt.Errorf("error merging branch: %v", err)
	}
	infoLogger.Printf("Merged branch %s from %s", branch, rcBranch)
	infoLogger.Printf("Branch %s merged successfully: %s on %s", branch, merge.GetHTMLURL(), repo)
	return nil
}

func CreateRcPullRequest(ctx context.Context, client *github.Client, owner, repo, branch, rcVersion, prTitle, prBody string) (prUrl string, err error) {
	rcBranch := fmt.Sprintf("rc/%s", rcVersion)
	pr, res, err := client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: github.String(prTitle),
		Head:  github.String(rcBranch),
		Base:  github.String(branch),
		Body:  github.String(prBody),
	})
	var prError string
	if err != nil {
		if res != nil && res.StatusCode == 422 {
			body, _ := io.ReadAll(res.Body)
			var responseBody map[string]interface{}
			if err := json.Unmarshal(body, &responseBody); err != nil {
				errorLogger.Printf("Error unmarshalling response body: %v", err)
				return "", fmt.Errorf("error unmarshalling response body: %v", err)
			}
			if errors, ok := responseBody["errors"].([]interface{}); ok && len(errors) > 0 {
				if message, ok := errors[0].(map[string]interface{})["message"].(string); ok {
					errorLogger.Printf("Response message: %s", message)
					prError = message
				} else {
					errorLogger.Printf("Response body: %s", body)
				}
			} else {
				errorLogger.Printf("Response body: %s", body)
			}
		} else {
			errorLogger.Printf("Error creating PR: %v", err)
			return "", fmt.Errorf("error %s creating PR: %v", res.Status, err)
		}
	} else {
		infoLogger.Printf("Created PR for branch %s on Repo %s", rcBranch, repo)
	}

	prUrl = pr.GetHTMLURL()
	if pr.GetHTMLURL() == "" {
		prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
			Head: fmt.Sprintf("%s:%s", owner, rcBranch),
		})
		if err != nil {
			errorLogger.Printf("Error getting PR: %v", err)
			return "", fmt.Errorf("error getting PR: %v", err)
		}
		if len(prs) > 0 {
			prUrl = prs[0].GetHTMLURL()
		}
	}
	fmtPrUrl := fmt.Sprintf("%s: %s  --> %s ", repo, prUrl, prError)
	return fmtPrUrl, nil
}

func RcValidate(rcVersion string) error {
	if rcVersion == "" {
		errorLogger.Printf("rcVersion is required")
		return fmt.Errorf("rcVersion is required")
	}
	if match, _ := regexp.MatchString(`^v\d+\.\d+\.\d+$`, rcVersion); !match {
		errorLogger.Printf("rcVersion should be in the format v*.*.*")
		return fmt.Errorf("rcVersion should be in the format v*.*.*")
	}

	infoLogger.Printf("Validated rcVersion: %s", rcVersion)
	return nil
}

func ListRepositories(ctx context.Context, client *github.Client, owner string) ([]string, error) {
	repos, _, err := client.Repositories.ListByOrg(ctx, owner, nil)
	if err != nil {
		errorLogger.Printf("Error listing repositories: %v", err)
		return nil, fmt.Errorf("error listing repositories: %v", err)
	}
	var repoNames []string
	for _, r := range repos {
		repoNames = append(repoNames, r.GetName())
	}

	infoLogger.Printf("Repositories: %v", repoNames)
	return repoNames, nil
}
