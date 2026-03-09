package cmd

import (
	"fmt"
	"strconv"

	"github.com/pdinh/gh-wrapup/internal/github"
	"github.com/pdinh/gh-wrapup/internal/util"
	"github.com/spf13/cobra"
)

var fromIssueCmd = &cobra.Command{
	Use:   "from-issue <issue-number>",
	Short: "Create a PR that closes an existing issue",
	Long: `Fetch an existing issue and create a PR that closes it.

The branch is created from the base branch HEAD (default: repo default branch).
The PR body will contain "Closes #N" linking it to the issue.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runFromIssue,
}

type fromIssueFlags struct {
	prTitle string
	prBody  string
	branch  string
	base    string
	draft   bool
	repo    string
}

var fromIssueFlagVals fromIssueFlags

func init() {
	f := fromIssueCmd.Flags()

	f.StringVar(&fromIssueFlagVals.prTitle, "pr-title", "", "PR title (default: issue title)")
	f.StringVar(&fromIssueFlagVals.prBody, "pr-body", "", "Additional PR body text (prepended before 'Closes #N')")
	f.StringVar(&fromIssueFlagVals.branch, "branch", "", "Branch name for PR head (default: auto-generated from issue title)")
	f.StringVar(&fromIssueFlagVals.base, "base", "", "Base branch for PR (default: repo default branch)")
	f.BoolVar(&fromIssueFlagVals.draft, "draft", false, "Create PR as draft")
	f.StringVar(&fromIssueFlagVals.repo, "repo", "", "Repository to use (format: owner/repo)")
}

func runFromIssue(cmd *cobra.Command, args []string) error {
	flags := fromIssueFlagVals

	issueNumber, err := strconv.Atoi(args[0])
	if err != nil || issueNumber <= 0 {
		return fmt.Errorf("invalid issue number %q: must be a positive integer", args[0])
	}

	owner, repoName, err := resolveRepo(flags.repo)
	if err != nil {
		return fmt.Errorf("resolving repository: %w", err)
	}

	client, err := github.NewClient(owner)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	// Step 1: Fetch issue.
	issue, err := client.GetIssue(owner, repoName, issueNumber)
	if err != nil {
		return fmt.Errorf("fetching issue #%d: %w", issueNumber, err)
	}
	util.Info("Found issue #%d: %s", issue.Number, issue.Title)

	// Step 2: Resolve base branch.
	base := flags.base
	if base == "" {
		base, err = client.GetDefaultBranch(owner, repoName)
		if err != nil {
			return fmt.Errorf("getting default branch: %w", err)
		}
	}

	// Step 3: Determine branch name.
	branchName := flags.branch
	if branchName == "" {
		branchName = util.Slugify(issue.Title, issue.Number)
	}

	// Step 4: Create branch from base HEAD.
	baseSHA, err := client.GetBranchSHA(owner, repoName, base)
	if err != nil {
		return fmt.Errorf("getting SHA for base branch %q: %w", base, err)
	}
	_, err = client.CreateBranch(owner, repoName, &github.BranchRequest{
		Ref: "refs/heads/" + branchName,
		SHA: baseSHA,
	})
	if err != nil {
		if github.IsAlreadyExists(err) {
			util.Reused("Branch %s (existing)", branchName)
		} else {
			return fmt.Errorf("creating branch %q: %w", branchName, err)
		}
	} else {
		util.Success("Branch %s created", branchName)
	}

	// Step 5: Build PR.
	prTitle := flags.prTitle
	if prTitle == "" {
		prTitle = issue.Title
	}
	prBody := buildPRBody(flags.prBody, issue.Number)

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
	util.Tree("Closes #%d", issue.Number)

	return nil
}
