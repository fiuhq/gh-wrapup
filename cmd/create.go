package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/pdinh/gh-wrapup/internal/github"
	"github.com/pdinh/gh-wrapup/internal/util"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an issue and a PR that closes it",
	Long: `Create a GitHub issue and a PR that closes it, atomically.

The PR branch is automatically created from the base branch HEAD.
The PR body will contain "Closes #N" linking it to the issue.`,
	RunE: runCreate,
}

// createFlags holds all the flags for the create command.
type createFlags struct {
	title     string
	body      string
	bodyFile  string
	labels    string
	assignee  string
	milestone string
	prTitle   string
	prBody    string
	branch    string
	base      string
	draft     bool
	repo      string
}

var createFlagVals createFlags

func init() {
	f := createCmd.Flags()

	f.StringVar(&createFlagVals.title, "title", "", "Issue title (required)")
	f.StringVar(&createFlagVals.body, "body", "", "Issue body")
	f.StringVar(&createFlagVals.bodyFile, "body-file", "", "Read issue body from file (use - for stdin)")
	f.StringVar(&createFlagVals.labels, "labels", "", "Comma-separated labels to apply to the issue")
	f.StringVar(&createFlagVals.assignee, "assignee", "@me", "Issue assignee (default: @me)")
	f.StringVar(&createFlagVals.milestone, "milestone", "", "Milestone name or number")
	f.StringVar(&createFlagVals.prTitle, "pr-title", "", "PR title (default: same as issue title)")
	f.StringVar(&createFlagVals.prBody, "pr-body", "", "Additional PR body text (prepended before 'Closes #N')")
	f.StringVar(&createFlagVals.branch, "branch", "", "Branch name for PR head (default: auto-generated from title)")
	f.StringVar(&createFlagVals.base, "base", "", "Base branch for PR (default: repo default branch)")
	f.BoolVar(&createFlagVals.draft, "draft", false, "Create PR as draft")
	f.StringVar(&createFlagVals.repo, "repo", "", "Repository to use (format: owner/repo)")

	_ = createCmd.MarkFlagRequired("title")
}

func runCreate(cmd *cobra.Command, args []string) error {
	flags := createFlagVals

	// Resolve repo owner/name.
	owner, repoName, err := resolveRepo(flags.repo)
	if err != nil {
		return fmt.Errorf("resolving repository: %w", err)
	}

	// Build issue body: --body-file overrides --body.
	issueBody, err := resolveBody(flags.body, flags.bodyFile)
	if err != nil {
		return fmt.Errorf("reading issue body: %w", err)
	}

	// Create the GitHub REST client.
	client, err := github.NewClient(owner)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	// Resolve base branch.
	base := flags.base
	if base == "" {
		base, err = client.GetDefaultBranch(owner, repoName)
		if err != nil {
			return fmt.Errorf("getting default branch: %w", err)
		}
	}

	// Build issue request.
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

	// Step 1: Create the issue.
	issue, err := client.CreateIssue(owner, repoName, issueReq)
	if err != nil {
		return fmt.Errorf("creating issue: %w", err)
	}
	util.Success("Issue #%d created: %s", issue.Number, issue.HTMLURL)

	// Step 2: Determine branch name.
	branchName := flags.branch
	if branchName == "" {
		branchName = util.Slugify(flags.title, issue.Number)
	}

	// Step 3: Create the branch from base HEAD.
	baseSHA, err := client.GetBranchSHA(owner, repoName, base)
	if err != nil {
		return fmt.Errorf("getting SHA for base branch %q: %w", base, err)
	}
	_, err = client.CreateBranch(owner, repoName, &github.BranchRequest{
		Ref: "refs/heads/" + branchName,
		SHA: baseSHA,
	})
	if err != nil {
		return fmt.Errorf("creating branch %q: %w", branchName, err)
	}
	util.Success("Branch %s created", branchName)

	// Step 4: Build PR title and body.
	prTitle := flags.prTitle
	if prTitle == "" {
		prTitle = flags.title
	}
	prBody := buildPRBody(flags.prBody, issue.Number)

	// Step 5: Create the PR.
	prReq := &github.PRRequest{
		Title: prTitle,
		Head:  branchName,
		Base:  base,
		Body:  prBody,
		Draft: flags.draft,
	}
	pr, err := client.CreatePR(owner, repoName, prReq)
	if err != nil {
		return fmt.Errorf("creating PR: %w", err)
	}
	util.Success("PR #%d created: %s", pr.Number, pr.HTMLURL)
	util.Tree("Closes #%d", issue.Number)

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
