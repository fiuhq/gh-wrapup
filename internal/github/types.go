package github

// ---- Issue types ----

// IssueRequest is the body sent to POST /repos/{owner}/{repo}/issues.
type IssueRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	// Milestone can be a number (as string) or name; we convert to int ID via API if needed.
	Milestone string `json:"-"`
}

// IssueUpdateRequest is the body sent to PATCH /repos/{owner}/{repo}/issues/{number}.
// All fields are pointers so we only send what's explicitly set.
type IssueUpdateRequest struct {
	Title     *string   `json:"title,omitempty"`
	Body      *string   `json:"body,omitempty"`
	Assignees *[]string `json:"assignees,omitempty"`
	Labels    *[]string `json:"labels,omitempty"`
	State     *string   `json:"state,omitempty"`
}

// IssueResponse represents a GitHub issue returned by the API.
type IssueResponse struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ---- PR types ----

// PRRequest is the body sent to POST /repos/{owner}/{repo}/pulls.
type PRRequest struct {
	Title               string `json:"title"`
	Head                string `json:"head"`
	Base                string `json:"base"`
	Body                string `json:"body,omitempty"`
	Draft               bool   `json:"draft"`
	MaintainerCanModify bool   `json:"maintainer_can_modify"`
}

// PRUpdateRequest is the body sent to PATCH /repos/{owner}/{repo}/pulls/{number}.
type PRUpdateRequest struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	State string `json:"state,omitempty"`
	Base  string `json:"base,omitempty"`
}

// PRResponse represents a GitHub pull request returned by the API.
type PRResponse struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
	Head    struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
}

// ---- Branch types ----

// BranchRequest is the body sent to POST /repos/{owner}/{repo}/git/refs.
type BranchRequest struct {
	// Ref is the full ref name, e.g. "refs/heads/my-branch".
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// BranchResponse represents a git ref returned by the API.
type BranchResponse struct {
	Ref    string `json:"ref"`
	NodeID string `json:"node_id"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

// BranchDetails is returned by GET /repos/{owner}/{repo}/branches/{branch}.
type BranchDetails struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

// ---- Repo types ----

// RepoResponse is a minimal representation of GET /repos/{owner}/{repo}.
type RepoResponse struct {
	DefaultBranch string `json:"default_branch"`
}

// ---- Search types ----

// SearchResult wraps the GitHub issue search API response.
type SearchResult struct {
	TotalCount int             `json:"total_count"`
	Items      []IssueResponse `json:"items"`
}

// ---- Error types ----

// apiError holds a GitHub API error response body.
type apiError struct {
	Message string `json:"message"`
	Errors  []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field"`
	} `json:"errors"`
}
