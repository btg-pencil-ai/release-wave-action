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

// FindMatchingBranches finds all branches that match the input epic
// inputEpic is expected to already be formatted (from Hydra webhook)
// Returns all matching branch names (there could be multiple matches)
func FindMatchingBranches(inputEpic string, epicBranches []string) []string {
	if inputEpic == "" {
		return nil
	}

	var matchedBranches []string

	for _, branch := range epicBranches {
		// Try exact match first (case insensitive)
		if strings.EqualFold(branch, inputEpic) {
			matchedBranches = append(matchedBranches, branch)
			continue
		}

		// Try match after formatting the branch name (branch from GitHub might have different casing/format)
		formattedBranch := ConvertToNamespace(branch)
		if formattedBranch == inputEpic {
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
				l.Info("No matching branch for epic '%s' in repo '%s'", epic, repo)
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

// SyncBranchResult represents the result of creating a sync branch
type SyncBranchResult struct {
	Repo            string
	Epic            string
	BranchName      string   // sync branch name (e.g., sync/v7.0.0-epic-beta-022)
	EpicBranchNames []string // target epic branch names (e.g., epic-beta-022, epic-BETA-022)
	Created         bool
	Error           string
}

// CreateSyncBranchesForEpics creates sync branches for each epic in repos where the epic branch exists
// Branch name format: sync/{release-version}-{formatted-epic-name}
func CreateSyncBranchesForEpics(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, owner string, baseBranch string, releaseVersion string, epicBranchResults map[string][]EpicBranchMatch) ([]SyncBranchResult, error) {
	var results []SyncBranchResult
	var errs []string

	for repo, matches := range epicBranchResults {
		for _, match := range matches {
			if !match.Found {
				continue
			}

			// Epic name is already formatted from Hydra webhook
			syncBranchName := fmt.Sprintf("sync/%s-%s", releaseVersion, match.Epic)

			l.Info("Creating sync branch '%s' in repo '%s' from '%s'", syncBranchName, repo, baseBranch)

			err := githubRepo.CreateBranch(ctx, owner, repo, baseBranch, syncBranchName)

			result := SyncBranchResult{
				Repo:            repo,
				Epic:            match.Epic,
				BranchName:      syncBranchName,
				EpicBranchNames: match.BranchNames,
			}

			if err != nil {
				l.Error("Error creating sync branch '%s' in repo '%s': %v", syncBranchName, repo, err)
				result.Created = false
				result.Error = err.Error()
				errs = append(errs, fmt.Sprintf("%s/%s: %v", repo, syncBranchName, err))
			} else {
				l.Info("Successfully created sync branch '%s' in repo '%s'", syncBranchName, repo)
				result.Created = true

			}

			results = append(results, result)
		}
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("failed to create some sync branches: %s", strings.Join(errs, "; "))
	}

	return results, nil
}

// SyncToEpicPRResult represents the result of creating a PR from sync branch to epic branch
type SyncToEpicPRResult struct {
	Repo       string
	SyncBranch string
	EpicBranch string
	PRURL      string
	Created    bool
	Error      string
}

// CreatePRsFromSyncToEpic creates PRs from sync branches to their corresponding epic branches
func CreatePRsFromSyncToEpic(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, owner string, releaseVersion string, syncResults []SyncBranchResult) ([]SyncToEpicPRResult, error) {
	var results []SyncToEpicPRResult
	var errs []string

	for _, syncResult := range syncResults {
		// Skip if sync branch wasn't created successfully
		if !syncResult.Created {
			l.Debug("Skipping PR creation for %s/%s - sync branch was not created", syncResult.Repo, syncResult.BranchName)
			continue
		}

		// Create PR to each matching epic branch for each epic in each repo
		for _, epicBranch := range syncResult.EpicBranchNames {
			prTitle := fmt.Sprintf("Sync %s to %s", releaseVersion, epicBranch)
			prBody := fmt.Sprintf("Syncing release %s changes to epic branch %s", releaseVersion, epicBranch)

			l.Info("Creating PR from '%s' to '%s' in repo '%s'", syncResult.BranchName, epicBranch, syncResult.Repo)

			prURL, prError, err := githubRepo.CreatePullRequest(ctx, owner, syncResult.Repo, syncResult.BranchName, epicBranch, prTitle, prBody)

			result := SyncToEpicPRResult{
				Repo:       syncResult.Repo,
				SyncBranch: syncResult.BranchName,
				EpicBranch: epicBranch,
			}

			if err != nil {
				l.Error("Error creating PR from '%s' to '%s' in repo '%s': %v", syncResult.BranchName, epicBranch, syncResult.Repo, err)
				result.Created = false
				result.Error = err.Error()
				errs = append(errs, fmt.Sprintf("%s: %s->%s: %v", syncResult.Repo, syncResult.BranchName, epicBranch, err))
			} else if prError != "" {
				l.Warn("PR created with warning from '%s' to '%s' in repo '%s': %s", syncResult.BranchName, epicBranch, syncResult.Repo, prError)
				result.Created = true
				result.PRURL = prURL
				result.Error = prError
			} else {
				l.Info("Successfully created PR from '%s' to '%s' in repo '%s': %s", syncResult.BranchName, epicBranch, syncResult.Repo, prURL)
				result.Created = true
				result.PRURL = prURL
			}

			results = append(results, result)
		}
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("failed to create some PRs: %s", strings.Join(errs, "; "))
	}

	return results, nil
}

// CleanupOldSyncBranches checks for open PRs targeting the epic branches and closes them if they are from old sync branches and deletes the sync branch
func CleanupOldSyncBranches(ctx context.Context, l utils.LogInterface, githubRepo githubrepo.GithubRepo, owner string, releaseVersion string, epicBranchResults map[string][]EpicBranchMatch) error {
	for repo, matches := range epicBranchResults {
		for _, match := range matches {
			if !match.Found {
				continue
			}

			// Regex to match sync branches for THIS specific epic: sync/{any-version}-{epic-name}
			// We use QuoteMeta to ensure the epic name is treated as a literal string
			patternStr := fmt.Sprintf(`^sync/v\d+\.\d+\.\d+-%s$`, regexp.QuoteMeta(match.Epic))
			syncBranchPattern, err := regexp.Compile(patternStr)
			if err != nil {
				l.Error("Error compiling regex for epic '%s': %v", match.Epic, err)
				return fmt.Errorf("error compiling regex for epic '%s': %v", match.Epic, err)
			}

			for _, epicBranch := range match.BranchNames {
				l.Info("Checking for old sync PRs targeting '%s' in repo '%s' for epic '%s'", epicBranch, repo, match.Epic)

				prs, err := githubRepo.ListOpenPullRequestsByBase(ctx, owner, repo, epicBranch)
				if err != nil {
					l.Error("Error listing PRs for repo %s base %s: %v", repo, epicBranch, err)
					return fmt.Errorf("error listing PRs for repo %s base %s: %v", repo, epicBranch, err)
				}

				for _, pr := range prs {
					headBranch := pr.Head.GetRef()
					if syncBranchPattern.MatchString(headBranch) {
						l.Info("Found old sync PR #%d from branch '%s' targeting '%s'", pr.GetNumber(), headBranch, epicBranch)

						// Close PR
						comment := "Closing old sync PR as a new sync process is starting for release " + releaseVersion + "."
						if err := githubRepo.ClosePullRequest(ctx, owner, repo, pr.GetNumber(), comment); err != nil {
							l.Error("Failed to close PR #%d: %v", pr.GetNumber(), err)
							return fmt.Errorf("failed to close PR #%d: %v", pr.GetNumber(), err)
						}

						// Delete branch
						if err := githubRepo.DeleteBranch(ctx, owner, repo, headBranch); err != nil {
							l.Error("Failed to delete branch '%s': %v", headBranch, err)
							return fmt.Errorf("failed to delete branch '%s': %v", headBranch, err)
						}
					}
				}
			}
		}
	}
	return nil
}
