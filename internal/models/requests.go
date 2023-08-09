package models

const (
	MessageTypeRefreshGiteaRepository      = "refresh_gitea_repository"
	MessageTypeRefreshGiteaRepositoryDone  = "refresh_gitea_repository_done"
	MessageTypeRefreshGitHubRepository     = "refresh_github_repository"
	MessageTypeRefreshGitHubRepositoryDone = "refresh_github_repository_done"
)

type CreateHook struct {
	Active              bool              `json:"active"`
	AuthorizationHeader string            `json:"authorization_header"`
	BranchFilter        string            `json:"branch_filter"`
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	Type                string            `json:"type"`
}

type RefreshGiteaRepositoryRequest struct {
	Repository     string `json:"repository"`
	Owner          string `json:"owner"`
	PullRequestID  int    `json:"pullRequestId"`
	CommentID      int    `json:"commentId"`
	CommentBody    string `json:"commentBody"`
	ReportProgress bool   `json:"reportProgress"`
}

type RefreshGiteaRepositoryDoneRequest struct {
	Repository     string `json:"repository"`
	Owner          string `json:"owner"`
	PullRequestID  int    `json:"pullRequestId"`
	CommentID      int    `json:"commentId"`
	CommentBody    string `json:"commentBody"`
	ReportProgress bool   `json:"reportProgress"`
	Status         string `json:"status"`
	Error          string `json:"error"`
}

type RefreshGitHubRepositoryRequest struct {
	Repository     string `json:"repository"`
	Owner          string `json:"owner"`
	PullRequestID  int    `json:"pullRequestId"`
	CommentID      int    `json:"commentId"`
	CommentBody    string `json:"commentBody"`
	ReportProgress bool   `json:"reportProgress"`
}

type RefreshGitHubRepositoryDoneRequest struct {
	Repository     string `json:"repository"`
	Owner          string `json:"owner"`
	PullRequestID  int    `json:"pullRequestId"`
	CommentID      int    `json:"commentId"`
	CommentBody    string `json:"commentBody"`
	ReportProgress bool   `json:"reportProgress"`
	Status         string `json:"status"`
	Error          string `json:"error"`
}

type AddCommentResponse struct {
	Body string `json:"body"`
	ID   int    `json:"id"`
}

type SupportedBackend string

const (
	SupportedBackendGitHub SupportedBackend = "github"
	SupportedBackendGitea  SupportedBackend = "gitea"
)
