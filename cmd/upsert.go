package cmd

import (
	"fmt"
	"strings"

	"github.com/pdinh/gh-wrapup/internal/github"
	"github.com/pdinh/gh-wrapup/internal/util"
	"github.com/spf13/cobra"
)

var upsertCmd = &cobra.Command{
	Use:   "upsert",
	Short: "Idempotent create-or-update an issue + PR",
	Long: `Idempotent version of "create". Searches for an existing open issue matching
the title (or --issue-search query) and reuses it if found. Similarly finds and
updates an existing PR for the branch rather than creating a duplicate.

Safe to run multiple times — will never create duplicate issues or PRs.`,
	RunE: runUpsert,
}

type upsertFlags struct {
	title       string
	body        string
	bodyFile    string
	labels      string
	assignee    string
	milestone   string
	prTitle     string
	prBody      string
	branch      string
	base        string
	draft       bool
	repo        string
	issueSearch string
}

var upsertFlagVals upsertFlags

func init() {
	f := upsertCmd.Flags()

	f.StringVar(&upsertFlagVals.title, "title", "", "Issue title (required)")
	f.StringVar(&upsertFlagVals.body, "body", "", "Issue body")
	f.StringVar(&upsertFlagVals.bodyFile, "body-file", "", "Read issue body from file (use - for stdin)")
	f.StringVar(&upsertFlagVals.labels, "labels", "", "Comma-separated labels to apply to the issue")
	f.StringVar(&upsertFlagVals.assignee, "assignee", "@me", "Issue assignee (default: @me)")
	f.StringVar(&upsertFlagVals.milestone, "milestone", "", "Milestone name or number")
	f.StringVar(&upsertFlagVals.prTitle, "pr-title", "", "PR title (default: same as issue title)")
	f.StringVar(&upsertFlagVals.prBody, "pr-body", "", "Additional PR body text (prepended before 'Closes #N')")
	f.StringVar(&upsertFlagVals.branch, "branch", "", "Branch name for PR head (default: auto-generated from title)")
	f.StringVar(&upsertFlagVals.base, "base", "", "Base branch for PR (default: repo default branch)")
	f.BoolVar(&upsertFlagVals.draft, "draft", false, "Create PR as draft")
	f.StringVar(&upsertFlagVals.repo, "repo", "", "Repository to use (format: owner/repo)")
	f.StringVar(&upsertFlagVals.issueSearch, "issue-search", "", "Search query to find existing issue (default: exact title match)")

	_ = upsertCmd.MarkFlagRequired("title")
}

func runUpsert(cmd *cobra.Command, args []string) error {
	flags := upsertFlagVals

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

	base := flags.base
	if base == "" {
		base, err = client.GetDefaultBranch(owner, repoName)
		if err != nil {
			return fmt.Errorf("getting default branch: %w", err)
		}
	}

	// --- Issue: find or create ---
	searchQuery := flags.issueSearch
	if searchQuery == "" {
		// Default: exact title match within this repo.
		searchQuery = fmt.Sprintf(`is:issue repo:%s/%s in:title "%s"`, owner, repoName, flags.title)
	}

	var issueNumber int
	var issueURL string

	existingIssues, err := client.SearchIssues(searchQuery)
	if err != nil {
		return fmt.Errorf("searching issues: %w", err)
	}

	if len(existingIssues) > 0 {
		// Reuse the first matching issue. Filter to exact title match if using default search.
		found := existingIssues[0]
		for _, iss := range existingIssues {
			if strings.EqualFold(iss.Title, flags.title) {
				found = iss
				break
			}
		}
		issueNumber = found.Number
		issueURL = found.HTMLURL
		util.Reused("Issue #%d (existing): %s", issueNumber, issueURL)

		// Update body/labels if provided.
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
			util.Info("Issue #%d updated", issueNumber)
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
		util.Success("Issue #%d created: %s", issueNumber, issueURL)
	}

	// --- Branch: ensure it exists ---
	branchName := flags.branch
	if branchName == "" {
		branchName = util.Slugify(flags.title, issueNumber)
	}

	baseSHA, err := client.GetBranchSHA(owner, repoName, base)
	if err != nil {
		return fmt.Errorf("getting SHA for base branch %q: %w", base, err)
	}

	// Try to create; if it already exists the API returns 422 which we handle gracefully.
	_, branchErr := client.CreateBranch(owner, repoName, &github.BranchRequest{
		Ref: "refs/heads/" + branchName,
		SHA: baseSHA,
	})
	if branchErr != nil {
		if github.IsAlreadyExists(branchErr) {
			util.Reused("Branch %s (existing)", branchName)
		} else {
			return fmt.Errorf("creating branch %q: %w", branchName, branchErr)
		}
	} else {
		util.Success("Branch %s created", branchName)
	}

	// --- PR: find or create ---
	prTitle := flags.prTitle
	if prTitle == "" {
		prTitle = flags.title
	}
	prBody := buildPRBody(flags.prBody, issueNumber)

	existingPR, err := client.GetPRForBranch(owner, repoName, owner, branchName)
	if err != nil {
		return fmt.Errorf("checking for existing PR: %w", err)
	}

	if existingPR != nil {
		util.Reused("PR #%d (existing): %s", existingPR.Number, existingPR.HTMLURL)
		// Update title/body if provided by flags.
		if flags.prTitle != "" || flags.prBody != "" {
			updateReq := &github.PRUpdateRequest{
				Title: prTitle,
				Body:  prBody,
			}
			if _, err = client.UpdatePR(owner, repoName, existingPR.Number, updateReq); err != nil {
				return fmt.Errorf("updating PR #%d: %w", existingPR.Number, err)
			}
			util.Info("PR #%d updated", existingPR.Number)
		}
		util.Tree("Closes #%d", issueNumber)
	} else {
		pr, err := client.CreatePR(owner, repoName, &github.PRRequest{
			Title: prTitle,
			Head:  branchName,
			Base:  base,
			Body:  prBody,
			Draft: flags.draft,
		})
		if err != nil {
			return fmt.Errorf("creating PR: %w", err)
		}
		util.Success("PR #%d created: %s", pr.Number, pr.HTMLURL)
		util.Tree("Closes #%d", issueNumber)
	}

	return nil
}
