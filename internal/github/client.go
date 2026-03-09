// Package github provides a thin wrapper around the go-gh v2 REST client
// with typed helpers for the GitHub API operations used by gh-wrapup.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	gogheapi "github.com/cli/go-gh/v2/pkg/api"
)

// Client wraps the go-gh REST client.
type Client struct {
	rest *gogheapi.RESTClient
}

// NewClient returns a Client using the ambient gh auth for the given host.
// host can be "github.com" or a GHE hostname; pass "" for github.com.
func NewClient(owner string) (*Client, error) {
	// go-gh picks up auth from ~/.config/gh/hosts.yml automatically.
	rest, err := gogheapi.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("initialising REST client: %w", err)
	}
	return &Client{rest: rest}, nil
}

// jsonBody JSON-encodes a struct into an io.Reader for use with Post/Patch.
func jsonBody(v interface{}) (io.Reader, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshalling request body: %w", err)
	}
	return bytes.NewReader(b), nil
}

// ---- Repo ----

// GetDefaultBranch returns the default branch name for the given repo.
func (c *Client) GetDefaultBranch(owner, repo string) (string, error) {
	var result RepoResponse
	path := fmt.Sprintf("repos/%s/%s", owner, repo)
	if err := c.rest.Get(path, &result); err != nil {
		return "", fmt.Errorf("GET %s: %w", path, err)
	}
	return result.DefaultBranch, nil
}

// ---- Issues ----

// CreateIssue creates a new issue and returns the response.
func (c *Client) CreateIssue(owner, repo string, req *IssueRequest) (*IssueResponse, error) {
	var result IssueResponse
	path := fmt.Sprintf("repos/%s/%s/issues", owner, repo)
	body, err := jsonBody(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	if err := c.rest.Post(path, body, &result); err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	return &result, nil
}

// GetIssue fetches a single issue by number.
func (c *Client) GetIssue(owner, repo string, number int) (*IssueResponse, error) {
	var result IssueResponse
	path := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, number)
	if err := c.rest.Get(path, &result); err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	return &result, nil
}

// UpdateIssue patches an existing issue.
func (c *Client) UpdateIssue(owner, repo string, number int, req *IssueUpdateRequest) (*IssueResponse, error) {
	var result IssueResponse
	path := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, number)
	body, err := jsonBody(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	if err := c.rest.Patch(path, body, &result); err != nil {
		return nil, fmt.Errorf("PATCH %s: %w", path, err)
	}
	return &result, nil
}

// SearchIssues searches issues/PRs using the GitHub search API.
// query is the full search query string (e.g. "is:issue repo:owner/repo in:title foo").
func (c *Client) SearchIssues(query string) ([]IssueResponse, error) {
	var result SearchResult
	path := "search/issues?q=" + url.QueryEscape(query)
	if err := c.rest.Get(path, &result); err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	return result.Items, nil
}

// ---- Pull Requests ----

// CreatePR creates a new pull request.
func (c *Client) CreatePR(owner, repo string, req *PRRequest) (*PRResponse, error) {
	var result PRResponse
	path := fmt.Sprintf("repos/%s/%s/pulls", owner, repo)
	body, err := jsonBody(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	if err := c.rest.Post(path, body, &result); err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	return &result, nil
}

// GetPRForBranch returns the open PR whose head branch matches, or nil if none found.
// headOwner is the owner of the fork (usually same as repo owner for non-forks).
func (c *Client) GetPRForBranch(owner, repo, headOwner, branch string) (*PRResponse, error) {
	var results []PRResponse
	path := fmt.Sprintf(
		"repos/%s/%s/pulls?state=open&head=%s:%s",
		owner, repo,
		url.QueryEscape(headOwner),
		url.QueryEscape(branch),
	)
	if err := c.rest.Get(path, &results); err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

// GetPR fetches a single pull request by number.
func (c *Client) GetPR(owner, repo string, number int) (*PRResponse, error) {
	var result PRResponse
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number)
	if err := c.rest.Get(path, &result); err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	return &result, nil
}

// UpdatePR patches an existing pull request.
func (c *Client) UpdatePR(owner, repo string, number int, req *PRUpdateRequest) (*PRResponse, error) {
	var result PRResponse
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number)
	body, err := jsonBody(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	if err := c.rest.Patch(path, body, &result); err != nil {
		return nil, fmt.Errorf("PATCH %s: %w", path, err)
	}
	return &result, nil
}

// ---- Branches / Git Refs ----

// GetBranchSHA returns the commit SHA at the tip of the named branch.
func (c *Client) GetBranchSHA(owner, repo, branch string) (string, error) {
	var result BranchDetails
	path := fmt.Sprintf("repos/%s/%s/branches/%s", owner, repo, url.QueryEscape(branch))
	if err := c.rest.Get(path, &result); err != nil {
		return "", fmt.Errorf("GET %s: %w", path, err)
	}
	return result.Commit.SHA, nil
}

// CreateBranch creates a new git ref (branch).
func (c *Client) CreateBranch(owner, repo string, req *BranchRequest) (*BranchResponse, error) {
	var result BranchResponse
	path := fmt.Sprintf("repos/%s/%s/git/refs", owner, repo)
	body, err := jsonBody(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	if err := c.rest.Post(path, body, &result); err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, wrappedBranchError(err))
	}
	return &result, nil
}

// ---- Error helpers ----

// alreadyExistsErr is a sentinel type we wrap 422 "Reference already exists" errors into.
type alreadyExistsErr struct{ cause error }

func (e *alreadyExistsErr) Error() string { return e.cause.Error() }
func (e *alreadyExistsErr) Unwrap() error { return e.cause }

// IsAlreadyExists returns true if the error represents a 422 "Reference already exists" response.
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*alreadyExistsErr)
	return ok
}

// wrappedBranchError maps the "Reference already exists" API error to alreadyExistsErr.
func wrappedBranchError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "Reference already exists") ||
		strings.Contains(msg, "reference already exists") {
		return &alreadyExistsErr{cause: err}
	}
	return err
}
