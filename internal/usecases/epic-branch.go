package usecases

import (
	"context"
	"fmt"
	"regexp"
	"release-candidate/internal/usecases/githubrepo"
	"release-candidate/internal/utils"
	"strings"
)

// ConvertToNamespace converts a string to namespace format
// Same logic as scripts/name_formatter.py from kube-deployment
func ConvertToNamespace(input string) string {
	// Convert to lowercase
	namespace := strings.ToLower(input)

	// Replace non-alphanumeric characters (except . and -) with -
	re := regexp.MustCompile(`[^a-z0-9.\-]`)
	namespace = re.ReplaceAllString(namespace, "-")

	// Limit to 30 characters
	if len(namespace) > 30 {
		namespace = namespace[:30]
	}

	// Strip trailing dashes
	namespace = strings.Trim(namespace, "-")

	return namespace
}

// FindMatchingBranches finds all branches that match the input epic after formatting
// Returns all matching branch names (there could be multiple matches)
func FindMatchingBranches(inputEpic string, epicBranches []string) []string {
	formattedInput := ConvertToNamespace(inputEpic)

	if formattedInput == "" {
		return nil
	}

	var matchedBranches []string

	for _, branch := range epicBranches {
		// Try exact match first (case insensitive)
		if strings.EqualFold(branch, inputEpic) {
			matchedBranches = append(matchedBranches, branch)
			continue
		}

		// Try match after formatting (format full branch name including epic- prefix)
		formattedBranch := ConvertToNamespace(branch)
		fmt.Printf("branch: %s\n", branch)
		fmt.Printf("formattedBranch: %s\n", formattedBranch)
		fmt.Printf("formattedInput: %s\n", formattedInput)
		if formattedBranch == formattedInput {
			matchedBranches = append(matchedBranches, branch)
		}
	}

	return matchedBranches
}

// EpicBranchMatch represents a match result for an epic in a repository
type EpicBranchMatch struct {
	Repo        string
	Epic        string
	BranchNames []string // Multiple branches can match a single epic
	Found       bool
}

// FindEpicBranchesInRepos checks each repo for matching epic branches
// Returns a map of repo -> []EpicBranchMatch for all active epics
func FindEpicBranchesInRepos(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, owner string, repoList []string, activeEpics []string) (map[string][]EpicBranchMatch, error) {
	results := make(map[string][]EpicBranchMatch)

	for _, repo := range repoList {
		l.Info("Checking epic branches in repo: %s", repo)

		epicBranches, err := githubRepo.ListEpicBranches(ctx, owner, repo)
		if err != nil {
			l.Error("Error listing epic branches for %s: %v", repo, err)
			return nil, err
		}

		if len(epicBranches) == 0 {
			l.Info("No epic branches found in %s", repo)
			continue
		}

		l.Debug("Found %d epic branches in %s: %v", len(epicBranches), repo, epicBranches)

		var matches []EpicBranchMatch
		for _, epic := range activeEpics {
			matchedBranches := FindMatchingBranches(epic, epicBranches)
			match := EpicBranchMatch{
				Repo:        repo,
				Epic:        epic,
				BranchNames: matchedBranches,
				Found:       len(matchedBranches) > 0,
			}
			matches = append(matches, match)

			if match.Found {
				l.Info("Found %d matching branch(es) %v for epic '%s' in repo '%s'", len(matchedBranches), matchedBranches, epic, repo)
			} else {
				l.Debug("No matching branch for epic '%s' in repo '%s'", epic, repo)
			}
		}

		if len(matches) > 0 {
			results[repo] = matches
		}
	}

	return results, nil
}

// GetReposWithEpicBranch returns repos that have a matching branch for the given epic
func GetReposWithEpicBranch(results map[string][]EpicBranchMatch, epic string) []string {
	var repos []string
	for repo, matches := range results {
		for _, match := range matches {
			if match.Epic == epic && match.Found {
				repos = append(repos, repo)
				break
			}
		}
	}
	return repos
}
