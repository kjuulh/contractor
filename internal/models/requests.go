package models

const (
	MessageTypeRefreshRepository     = "refresh_repository"
	MessageTypeRefreshRepositoryDone = "refresh_repository_done"
)

type CreateHook struct {
	Active              bool              `json:"active"`
	AuthorizationHeader string            `json:"authorization_header"`
	BranchFilter        string            `json:"branch_filter"`
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	Type                string            `json:"type"`
}

type RefreshRepositoryRequest struct {
	Repository     string `json:"repository"`
	Owner          string `json:"owner"`
	PullRequestID  int    `json:"pullRequestId"`
	CommentID      int    `json:"commentId"`
	CommentBody    string `json:"commentBody"`
	ReportProgress bool   `json:"reportProgress"`
}

type RefreshDoneRepositoryRequest struct {
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
