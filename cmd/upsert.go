package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/fiuhq/gh-wrapup/internal/github"
	"github.com/fiuhq/gh-wrapup/internal/util"
	"github.com/spf13/cobra"
)

var upsertCmd = &cobra.Command{
	Use:   "upsert",
	Short: "Idempotent create-or-update an issue + PR",
	Long: `Idempotent create-or-update. Handles four modes via flag combinations:

  Mode 1 (default): Search-or-create issue, then create-or-update PR.
    Requires: --title

  Mode 2 (--issue=N): Use existing issue, create branch + PR.
    Fetches issue N, creates branch and PR with "Closes #N".

  Mode 3 (--pr=N): Create issue, link to existing PR.
    Creates issue (requires --title), prepends "Closes #N" to existing PR body.

  Mode 4 (--issue=N --pr=M): Both exist, link them.
    Prepends "Closes #N" to PR body. No branch creation.

Safe to run multiple times — will never create duplicate issues or PRs.`,
	RunE: runUpsert,
}

// upsertFlags holds all flags for the upsert command.
type upsertFlags struct {
	// Issue flags
	title     string
	body      string
	bodyFile  string
	labels    string
	assignee  string
	milestone string
	issue     int // --issue N: use existing issue instead of creating

	// PR flags
	prTitle string
	prBody  string
	branch  string
	base    string
	draft   bool
	pr      int // --pr N: use existing PR instead of creating

	// General
	repo        string
	issueSearch string
	jsonOutput  bool // --json: output as JSON
}

var flagVals upsertFlags

func init() {
	f := upsertCmd.Flags()

	f.StringVar(&flagVals.title, "title", "", "Issue title")
	f.StringVar(&flagVals.body, "body", "", "Issue body")
	f.StringVar(&flagVals.bodyFile, "body-file", "", "Read issue body from file (- for stdin)")
	f.StringVar(&flagVals.labels, "labels", "", "Comma-separated labels for the issue")
	f.StringVar(&flagVals.assignee, "assignee", "", "Issue assignee (GitHub username)")
	f.StringVar(&flagVals.milestone, "milestone", "", "Milestone name or number")
	f.IntVar(&flagVals.issue, "issue", 0, "Use existing issue number instead of creating one")

	f.StringVar(&flagVals.prTitle, "pr-title", "", "PR title (default: issue title)")
	f.StringVar(&flagVals.prBody, "pr-body", "", "Additional PR body text")
	f.StringVar(&flagVals.branch, "branch", "", "Branch name (default: auto-generated)")
	f.StringVar(&flagVals.base, "base", "", "Base branch (default: repo default)")
	f.BoolVar(&flagVals.draft, "draft", false, "Create PR as draft")
	f.IntVar(&flagVals.pr, "pr", 0, "Use existing PR number instead of creating one")

	f.StringVar(&flagVals.repo, "repo", "", "Target repository (owner/repo)")
	f.StringVar(&flagVals.issueSearch, "issue-search", "", "Custom search query to find existing issue")
	f.BoolVar(&flagVals.jsonOutput, "json", false, "Output as JSON")
}

// jsonResult is the structured output when --json is set.
type jsonResult struct {
	Issue struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"issue"`
	PR struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"pr"`
	Created bool `json:"created"`
}

func outputJSON(issueNum int, issueURL string, prNum int, prURL string, created bool) {
	result := jsonResult{}
	result.Issue.Number = issueNum
	result.Issue.URL = issueURL
	result.PR.Number = prNum
	result.PR.URL = prURL
	result.Created = created
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func runUpsert(cmd *cobra.Command, args []string) error {
	flags := flagVals
	silent := flags.jsonOutput

	owner, repoName, err := resolveRepo(flags.repo)
	if err != nil {
		return fmt.Errorf("resolving repository: %w", err)
	}

	issueBody, err := resolveBody(flags.body, flags.bodyFile)
	if err != nil {
		return fmt.Errorf("reading issue body: %w", err)
	}

	client, err := github.NewClient(owner)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	hasIssue := flags.issue > 0
	hasPR := flags.pr > 0

	// Validation: title required unless an existing issue is provided.
	if !hasIssue && flags.title == "" {
		return fmt.Errorf("--title is required when not using --issue")
	}

	// --- Resolve or create issue ---
	var issueNumber int
	var issueURL string
	var issueTitle string

	if hasIssue {
		// Mode 2 or 4: use existing issue.
		issue, err := client.GetIssue(owner, repoName, flags.issue)
		if err != nil {
			return fmt.Errorf("fetching issue #%d: %w", flags.issue, err)
		}
		issueNumber = issue.Number
		issueURL = issue.HTMLURL
		issueTitle = issue.Title
		if !silent {
			util.Info("Found issue #%d: %s", issueNumber, issueTitle)
		}
	} else {
		// Mode 1 or 3: search-or-create issue.
		issueTitle = flags.title

		searchQuery := flags.issueSearch
		if searchQuery == "" {
			searchQuery = fmt.Sprintf(`is:issue repo:%s/%s in:title "%s"`, owner, repoName, flags.title)
		}

		existingIssues, err := client.SearchIssues(searchQuery)
		if err != nil {
			return fmt.Errorf("searching issues: %w", err)
		}

		if len(existingIssues) > 0 {
			// Reuse the first matching issue; prefer exact title match.
			found := existingIssues[0]
			for _, iss := range existingIssues {
				if strings.EqualFold(iss.Title, flags.title) {
					found = iss
					break
				}
			}
			issueNumber = found.Number
			issueURL = found.HTMLURL
			if !silent {
				util.Reused("Issue #%d (existing): %s", issueNumber, issueURL)
			}

			// Update body/labels if explicitly provided.
			if issueBody != "" || flags.labels != "" {
				updateReq := &github.IssueUpdateRequest{}
				if issueBody != "" {
					updateReq.Body = &issueBody
				}
				if flags.labels != "" {
					lbls := splitTrimmed(flags.labels)
					updateReq.Labels = &lbls
				}
				if _, err = client.UpdateIssue(owner, repoName, issueNumber, updateReq); err != nil {
					return fmt.Errorf("updating issue #%d: %w", issueNumber, err)
				}
				if !silent {
					util.Info("Issue #%d updated", issueNumber)
				}
			}
		} else {
			// Create new issue.
			issueReq := &github.IssueRequest{
				Title: flags.title,
				Body:  issueBody,
			}
			if flags.assignee != "" {
				issueReq.Assignees = []string{flags.assignee}
			}
			if flags.labels != "" {
				issueReq.Labels = splitTrimmed(flags.labels)
			}
			if flags.milestone != "" {
				issueReq.Milestone = flags.milestone
			}

			issue, err := client.CreateIssue(owner, repoName, issueReq)
			if err != nil {
				return fmt.Errorf("creating issue: %w", err)
			}
			issueNumber = issue.Number
			issueURL = issue.HTMLURL
			if !silent {
				util.Success("Issue #%d created: %s", issueNumber, issueURL)
			}
		}
	}

	// --- Resolve or create/update PR ---
	var prNumber int
	var prURL string
	var prCreated bool

	if hasPR {
		// Mode 3 or 4: use existing PR, update its body to add "Closes #N".
		existingPR, err := client.GetPR(owner, repoName, flags.pr)
		if err != nil {
			return fmt.Errorf("fetching PR #%d: %w", flags.pr, err)
		}

		closesLink := fmt.Sprintf("Closes #%d", issueNumber)
		newBody := existingPR.Body
		if !strings.Contains(newBody, closesLink) {
			if newBody == "" {
				newBody = closesLink
			} else {
				newBody = closesLink + "\n\n" + newBody
			}
			_, err = client.UpdatePR(owner, repoName, existingPR.Number, &github.PRUpdateRequest{
				Body: newBody,
			})
			if err != nil {
				return fmt.Errorf("updating PR #%d: %w", existingPR.Number, err)
			}
			if !silent {
				util.Success("PR #%d updated: %s", existingPR.Number, existingPR.HTMLURL)
			}
		} else {
			if !silent {
				util.Reused("PR #%d already linked to issue #%d", existingPR.Number, issueNumber)
			}
		}
		if !silent {
			util.Tree("Closes #%d", issueNumber)
		}

		prNumber = existingPR.Number
		prURL = existingPR.HTMLURL
	} else {
		// Mode 1 or 2: create branch + create-or-update PR.

		// Resolve base branch.
		base := flags.base
		if base == "" {
			base, err = client.GetDefaultBranch(owner, repoName)
			if err != nil {
				return fmt.Errorf("getting default branch: %w", err)
			}
		}

		// Determine branch name.
		branchName := flags.branch
		if branchName == "" {
			branchName = util.Slugify(issueTitle, issueNumber)
		}

		// Create branch (idempotent — 422 "already exists" is handled gracefully).
		baseSHA, err := client.GetBranchSHA(owner, repoName, base)
		if err != nil {
			return fmt.Errorf("getting SHA for base branch %q: %w", base, err)
		}
		_, branchErr := client.CreateBranch(owner, repoName, &github.BranchRequest{
			Ref: "refs/heads/" + branchName,
			SHA: baseSHA,
		})
		if branchErr != nil {
			if github.IsAlreadyExists(branchErr) {
				if !silent {
					util.Reused("Branch %s (existing)", branchName)
				}
			} else {
				return fmt.Errorf("creating branch %q: %w", branchName, branchErr)
			}
		} else {
			if !silent {
				util.Success("Branch %s created", branchName)
			}
		}

		// Build PR title and body.
		prTitle := flags.prTitle
		if prTitle == "" {
			prTitle = issueTitle
		}
		prBodyStr := buildPRBody(flags.prBody, issueNumber)

		// Create or update PR for this branch.
		existingPR, err := client.GetPRForBranch(owner, repoName, owner, branchName)
		if err != nil {
			return fmt.Errorf("checking for existing PR: %w", err)
		}

		if existingPR != nil {
			if !silent {
				util.Reused("PR #%d (existing): %s", existingPR.Number, existingPR.HTMLURL)
			}
			// Update title/body if explicitly provided by flags.
			if flags.prTitle != "" || flags.prBody != "" {
				updateReq := &github.PRUpdateRequest{
					Title: prTitle,
					Body:  prBodyStr,
				}
				if _, err = client.UpdatePR(owner, repoName, existingPR.Number, updateReq); err != nil {
					return fmt.Errorf("updating PR #%d: %w", existingPR.Number, err)
				}
				if !silent {
					util.Info("PR #%d updated", existingPR.Number)
				}
			}
			if !silent {
				util.Tree("Closes #%d", issueNumber)
			}
			prNumber = existingPR.Number
			prURL = existingPR.HTMLURL
		} else {
			pr, err := client.CreatePR(owner, repoName, &github.PRRequest{
				Title: prTitle,
				Head:  branchName,
				Base:  base,
				Body:  prBodyStr,
				Draft: flags.draft,
			})
			if err != nil {
				return fmt.Errorf("creating PR: %w", err)
			}
			if !silent {
				util.Success("PR #%d created: %s", pr.Number, pr.HTMLURL)
				util.Tree("Closes #%d", issueNumber)
			}
			prNumber = pr.Number
			prURL = pr.HTMLURL
			prCreated = true
		}
	}

	if flags.jsonOutput {
		outputJSON(issueNumber, issueURL, prNumber, prURL, prCreated)
	}

	return nil
}

// resolveRepo splits "owner/repo" from the flag or falls back to gh.CurrentRepository().
func resolveRepo(repoFlag string) (owner, name string, err error) {
	if repoFlag != "" {
		parts := strings.SplitN(repoFlag, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid --repo format %q, expected owner/repo", repoFlag)
		}
		return parts[0], parts[1], nil
	}
	repo, err := repository.Current()
	if err != nil {
		return "", "", fmt.Errorf("could not determine current repository (use --repo flag): %w", err)
	}
	return repo.Owner, repo.Name, nil
}

// resolveBody returns the body string, reading from file if --body-file is set.
func resolveBody(body, bodyFile string) (string, error) {
	if bodyFile == "" {
		return body, nil
	}
	if bodyFile == "-" {
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return "", fmt.Errorf("reading from stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(bodyFile)
	if err != nil {
		return "", fmt.Errorf("reading file %q: %w", bodyFile, err)
	}
	return string(data), nil
}

// buildPRBody constructs the PR body with a Closes link.
func buildPRBody(extra string, issueNumber int) string {
	closes := fmt.Sprintf("Closes #%d", issueNumber)
	if extra == "" {
		return closes
	}
	return extra + "\n\n" + closes
}

// splitTrimmed splits a comma-separated string and trims whitespace.
func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
