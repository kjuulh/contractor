package providers

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	"git.front.kjuulh.io/kjuulh/contractor/internal/models"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v53/github"
)

type GitHubClient struct {
	appID          *int64
	installationID *int64
	privateKeyPath *string

	client *github.Client
}

func NewGitHubClient(appID, installationID *int64, privateKeyPath *string) *GitHubClient {
	return &GitHubClient{
		appID:          appID,
		installationID: installationID,
		privateKeyPath: privateKeyPath,
	}
}

func (gc *GitHubClient) makeSureClientExist() {
	if gc.client != nil {
		return
	}

	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, *gc.appID, *gc.installationID, *gc.privateKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	client := github.NewClient(&http.Client{Transport: itr})

	gc.client = client
}

func (gc *GitHubClient) EditComment(
	ctx context.Context,
	doneRequest *models.RefreshGitHubRepositoryDoneRequest,
) error {
	gc.makeSureClientExist()

	commentBody := html.UnescapeString(doneRequest.CommentBody)
	startCmnt := "<!-- Status update start -->"
	startIdx := strings.Index(commentBody, startCmnt)
	endIdx := strings.Index(commentBody, "<!-- Status update end -->")
	if startIdx >= 0 && endIdx >= 0 {
		log.Println("found comment to replace")

		var content string

		if doneRequest.Error != "" {
			content = fmt.Sprintf("<pre>ERROR: %s</pre><br>", doneRequest.Error)
		}
		if doneRequest.Status != "" {
			content = fmt.Sprintf("%s<p>%s</p>", content, doneRequest.Status)
		}

		doneRequest.CommentBody = fmt.Sprintf(
			"%s<br><hr>%s<hr><br>%s",
			commentBody[:startIdx+len(startCmnt)],
			content,
			commentBody[endIdx:],
		)
	}

	_, _, err := gc.client.Issues.EditComment(ctx, doneRequest.Owner, doneRequest.Repository, int64(doneRequest.CommentID), &github.IssueComment{
		Body: &doneRequest.CommentBody,
	})
	if err != nil {
		log.Printf("failed to update comment: %s", err.Error())
		return err
	}

	return nil
}

func (gc *GitHubClient) CreateWebhook(owner, repository string) error {
	gc.makeSureClientExist()

	// TODO: support for personal access tokens
	// We implicitly get support via. github apps

	return nil
}

func (gc *GitHubClient) AddComment(
	owner, repository string,
	pullRequest int,
	comment string,
) (*models.AddCommentResponse, error) {
	gc.makeSureClientExist()

	resp, _, err := gc.client.Issues.CreateComment(context.Background(), owner, repository, pullRequest, &github.IssueComment{
		Body: &comment,
	})
	if err != nil {
		return nil, err
	}

	return &models.AddCommentResponse{
		Body: *resp.Body,
		ID:   int(*resp.ID),
	}, nil
}
