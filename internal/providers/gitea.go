package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"

	"git.front.kjuulh.io/kjuulh/contractor/internal/models"
)

type GiteaClient struct {
	url   *string
	token *string

	client *http.Client
}

func NewGiteaClient(url, token *string) *GiteaClient {
	return &GiteaClient{
		url:    url,
		token:  token,
		client: http.DefaultClient,
	}
}

func (gc *GiteaClient) EditComment(
	ctx context.Context,
	doneRequest *models.RefreshGiteaRepositoryDoneRequest,
) error {
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

	editComment := struct {
		Body string `json:"body"`
	}{
		Body: doneRequest.CommentBody,
	}

	body, err := json.Marshal(editComment)
	if err != nil {
		log.Println("failed to marshal request body: %w", err)
		return err
	}
	bodyReader := bytes.NewReader(body)

	request, err := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf(
			"%s/repos/%s/%s/issues/comments/%d",
			strings.TrimSuffix(*gc.url, "/"),
			doneRequest.Owner,
			doneRequest.Repository,
			doneRequest.CommentID,
		),
		bodyReader,
	)
	if err != nil {
		log.Printf("failed to form update comment request: %s", err.Error())
		return err
	}
	request.Header.Add("Authorization", fmt.Sprintf("token %s", *gc.token))
	request.Header.Add("Content-Type", "application/json")

	resp, err := gc.client.Do(request)
	if err != nil {
		log.Printf("failed to update comment: %s", err.Error())
		return err
	}

	if resp.StatusCode > 299 {
		log.Printf("failed to update comment  with status code: %d", resp.StatusCode)
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read body of error response: %s", err.Error())
		} else {
			log.Printf("request body: %s", string(respBody))
		}
	}

	return nil
}

func (gc *GiteaClient) CreateWebhook(owner, repository string) error {
	createHookOptions := models.CreateHook{
		Active:              true,
		AuthorizationHeader: "",
		BranchFilter:        "*",
		Config: map[string]string{
			"url":          "http://10.0.9.1:8080/gitea/webhook",
			"content_type": "json",
		},
		Events: []string{
			"pull_request_comment",
		},
		Type: "gitea",
	}

	body, err := json.Marshal(createHookOptions)
	if err != nil {
		log.Println("failed to marshal request body: %w", err)
		return err
	}
	bodyReader := bytes.NewReader(body)
	request, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(
			"%s/repos/%s/%s/hooks",
			strings.TrimSuffix(*gc.url, "/"),
			owner,
			repository,
		),
		bodyReader,
	)
	if err != nil {
		log.Printf("failed to form create hook request: %s", err.Error())
		return err
	}
	request.Header.Add("Authorization", fmt.Sprintf("token %s", *gc.token))
	request.Header.Add("Content-Type", "application/json")

	resp, err := gc.client.Do(request)
	if err != nil {
		log.Printf("failed to register hook: %s", err.Error())
		return err
	}

	if resp.StatusCode > 299 {
		log.Printf("failed to register with status code: %d", resp.StatusCode)
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read body of error response: %s", err.Error())
		} else {
			log.Printf("request body: %s", string(respBody))
		}
	}

	return nil
}

func (gc *GiteaClient) AddComment(
	owner, repository string,
	pullRequest int,
	comment string,
) (*models.AddCommentResponse, error) {
	addComment := struct {
		Body string `json:"body"`
	}{
		Body: comment,
	}

	body, err := json.Marshal(addComment)
	if err != nil {
		return nil, err
	}
	bodyReader := bytes.NewReader(body)

	request, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(
			"%s/repos/%s/%s/issues/%d/comments",
			strings.TrimSuffix(*gc.url, "/"),
			owner,
			repository,
			pullRequest,
		),
		bodyReader,
	)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("token %s", *gc.token))
	request.Header.Add("Content-Type", "application/json")

	resp, err := gc.client.Do(request)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode > 299 {
		log.Printf("failed to register with status code: %d", resp.StatusCode)
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		} else {
			log.Printf("request body: %s", string(respBody))
		}
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response models.AddCommentResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
