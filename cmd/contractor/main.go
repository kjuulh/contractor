package contractor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"dagger.io/dagger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type createHook struct {
	Active              bool              `json:"active"`
	AuthorizationHeader string            `json:"authorization_header"`
	BranchFilter        string            `json:"branch_filter"`
	Config              map[string]string `json:"config"`
	Events              []string          `json:"events"`
	Type                string            `json:"type"`
}

func installCmd() *cobra.Command {
	var (
		owner      string
		repository string
		serverType string

		url   string
		token string
	)

	cmd := &cobra.Command{
		Use: "install",

		Run: func(cmd *cobra.Command, args []string) {
			if err := NewGiteaClient(&url, &token).CreateWebhook(owner, repository); err != nil {
				log.Printf("failed to add create webhook: %s", err.Error())
			}
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "the owner for which the repository belongs")
	cmd.Flags().StringVarP(&repository, "repository", "p", "", "the repository to install")
	cmd.Flags().
		StringVar(&serverType, "server-type", "gitea", "the server type to use [gitea, github]")
	cmd.MarkFlagRequired("owner")
	cmd.MarkFlagRequired("repository")

	cmd.PersistentFlags().StringVar(&url, "url", "", "the api url of the server")
	cmd.PersistentFlags().StringVar(&token, "token", "", "the token to authenticate with")

	return cmd
}

func serverCmd() *cobra.Command {
	var (
		url   string
		token string
	)

	giteaClient := NewGiteaClient(&url, &token)
	renovateClient := NewRenovateClient("")
	queue := NewGoQueue()
	queue.Subscribe(
		MessageTypeRefreshRepository,
		func(ctx context.Context, item *QueueMessage) error {
			log.Printf("handling message: %s, content: %s", item.Type, item.Content)
			return nil
		},
	)
	queue.Subscribe(
		MessageTypeRefreshRepositoryDone,
		func(ctx context.Context, item *QueueMessage) error {
			log.Printf("handling message: %s, content: %s", item.Type, item.Content)
			return nil
		},
	)
	queue.Subscribe(
		MessageTypeRefreshRepository,
		func(ctx context.Context, item *QueueMessage) error {
			var request RefreshRepositoryRequest
			if err := json.Unmarshal([]byte(item.Content), &request); err != nil {
				log.Printf("failed to unmarshal request body: %s", err.Error())
				return err
			}

			cancelCtx, cancel := context.WithTimeout(ctx, time.Second*30)
			defer cancel()

			if err := renovateClient.RefreshRepository(cancelCtx, request.Owner, request.Repository); err != nil {
				queue.Insert(MessageTypeRefreshRepositoryDone, RefreshDoneRepositoryRequest{
					Repository:     request.Repository,
					Owner:          request.Owner,
					PullRequestID:  request.PullRequestID,
					CommentID:      request.CommentID,
					CommentBody:    request.CommentBody,
					ReportProgress: request.ReportProgress,
					Status:         "failed",
					Error:          err.Error(),
				})

				return err
			}

			queue.Insert(MessageTypeRefreshRepositoryDone, RefreshDoneRepositoryRequest{
				Repository:     request.Repository,
				Owner:          request.Owner,
				PullRequestID:  request.PullRequestID,
				CommentID:      request.CommentID,
				CommentBody:    request.CommentBody,
				ReportProgress: request.ReportProgress,
				Status:         "done",
				Error:          "",
			})

			return nil
		},
	)

	queue.Subscribe(
		MessageTypeRefreshRepositoryDone,
		func(ctx context.Context, item *QueueMessage) error {
			var doneRequest RefreshDoneRepositoryRequest
			if err := json.Unmarshal([]byte(item.Content), &doneRequest); err != nil {
				log.Printf("failed to unmarshal request body: %s", err.Error())
				return err
			}

			return giteaClient.EditComment(ctx, &doneRequest)
		},
	)

	cmd := &cobra.Command{
		Use: "server",
	}

	cmd.PersistentFlags().StringVar(&url, "url", "", "the api url of the server")
	cmd.PersistentFlags().StringVar(&token, "token", "", "the token to authenticate with")

	cmd.AddCommand(serverServeCmd(&url, &token, queue, giteaClient))

	return cmd
}

const (
	MessageTypeRefreshRepository     = "refresh_repository"
	MessageTypeRefreshRepositoryDone = "refresh_repository_done"
)

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

func serverServeCmd(
	url *string,
	token *string,
	queue *GoQueue,
	giteaClient *GiteaClient,
) *cobra.Command {
	cmd := &cobra.Command{
		Use: "serve",
		Run: func(cmd *cobra.Command, args []string) {
			engine := gin.Default()

			gitea := engine.Group("/gitea")
			{
				gitea.POST("/webhook", func(ctx *gin.Context) {
					log.Println("received")

					type GiteaWebhookRequest struct {
						Action string `json:"action"`
						Issue  struct {
							Id     int `json:"id"`
							Number int `json:"number"`
						} `json:"issue"`
						Comment struct {
							Body string `json:"body"`
						} `json:"comment"`
						Repository struct {
							FullName string `json:"full_name"`
						}
					}

					var request GiteaWebhookRequest

					if err := ctx.BindJSON(&request); err != nil {
						ctx.AbortWithError(500, err)
						return
					}

					command, ok := validateBotComment(request.Comment.Body)
					if ok {
						log.Printf("got webhook request: contractor %s", command)

						bot := NewBotHandler(giteaClient)
						output, err := bot.Handle(command)
						if err != nil {
							log.Printf("failed to run bot handler with error: %s", err.Error())
						}

						parts := strings.Split(request.Repository.FullName, "/")

						comment, err := bot.AppendComment(
							parts[0],
							parts[1],
							request.Issue.Number,
							output,
						)
						if err != nil {
							ctx.AbortWithError(500, err)
							return
						}

						if err := queue.Insert(MessageTypeRefreshRepository, RefreshRepositoryRequest{
							Repository:     parts[1],
							Owner:          parts[0],
							PullRequestID:  request.Issue.Number,
							CommentID:      comment.ID,
							CommentBody:    comment.Body,
							ReportProgress: true,
						}); err != nil {
							ctx.AbortWithError(500, err)
							return
						}

						ctx.Status(204)

					}
				})
			}

			engine.Run("0.0.0.0:8080")
		},
	}

	return cmd
}

func validateBotComment(s string) (request string, ok bool) {
	if after, ok := strings.CutPrefix(s, "/contractor"); ok {
		return strings.TrimSpace(after), true
	}

	return "", false
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "contractor"}

	cmd.AddCommand(installCmd(), serverCmd())

	return cmd
}

type BotHandler struct {
	giteaClient *GiteaClient
}

func NewBotHandler(gitea *GiteaClient) *BotHandler {
	return &BotHandler{giteaClient: gitea}
}

func (b *BotHandler) Handle(input string) (output string, err error) {
	innerHandle := func(input string) (output string, err error) {
		if strings.HasPrefix(input, "help") {
			return b.Help(), nil
		}

		if strings.HasPrefix(input, "refresh") {
			return `
<h3>Contractor triggered renovate refresh on this repository</h3>
This comment will be updated with status

<!-- Status update start -->
<!-- Status update end -->
`, nil
		}

		return b.Help(), errors.New("could not recognize command")
	}

	output, err = innerHandle(input)
	output = fmt.Sprintf(
		"%s\n<small>This comment was generated by <a href='https://git.front.kjuulh.io/kjuulh/contractor'>Contractor</a></small>",
		output,
	)
	return output, err
}

func (b *BotHandler) Help() string {
	return `<details open>
	<summary><h3>/contractor [command]</h3></summary>

<strong>Commands:</strong>

* /contractor help
  *  triggers the help menu
* /contractor refresh
  *  triggers renovate to refresh the current pull request
</details>`
}

type AddCommentResponse struct {
	Body string `json:"body"`
	ID   int    `json:"id"`
}

func (b *BotHandler) AppendComment(
	owner string,
	repository string,
	pullRequest int,
	comment string,
) (*AddCommentResponse, error) {
	return b.giteaClient.AddComment(owner, repository, pullRequest, comment)
}

type QueueMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type GoQueue struct {
	queue           []*QueueMessage
	queueLock       sync.Mutex
	subscribers     map[string]map[string]func(ctx context.Context, item *QueueMessage) error
	subscribersLock sync.RWMutex
}

func NewGoQueue() *GoQueue {
	return &GoQueue{
		queue: make([]*QueueMessage, 0),
		subscribers: make(
			map[string]map[string]func(ctx context.Context, item *QueueMessage) error,
		),
	}
}

func (gq *GoQueue) Subscribe(
	messageType string,
	callback func(ctx context.Context, item *QueueMessage) error,
) string {
	gq.subscribersLock.Lock()
	defer gq.subscribersLock.Unlock()

	uid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	id := uid.String()

	_, ok := gq.subscribers[messageType]
	if !ok {
		messageTypeSubscribers := make(
			map[string]func(ctx context.Context, item *QueueMessage) error,
		)
		messageTypeSubscribers[id] = callback
		gq.subscribers[messageType] = messageTypeSubscribers
	} else {
		gq.subscribers[messageType][id] = callback
	}

	return id
}

func (gq *GoQueue) Unsubscribe(messageType string, id string) {
	gq.subscribersLock.Lock()
	defer gq.subscribersLock.Unlock()
	_, ok := gq.subscribers[messageType]
	if !ok {
		// No work to be done
		return
	} else {
		delete(gq.subscribers[messageType], id)
	}
}

func (gq *GoQueue) Insert(messageType string, content any) error {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()

	contents, err := json.Marshal(content)
	if err != nil {
		return err
	}

	gq.queue = append(gq.queue, &QueueMessage{
		Type:    messageType,
		Content: string(contents),
	})

	go func() {
		gq.handle(context.Background())
	}()

	return nil
}

func (gq *GoQueue) handle(ctx context.Context) {
	gq.queueLock.Lock()
	defer gq.queueLock.Unlock()

	for {
		if len(gq.queue) == 0 {
			return
		}

		item := gq.queue[0]
		gq.queue = gq.queue[1:]

		gq.subscribersLock.RLock()
		defer gq.subscribersLock.RUnlock()

		for id, callback := range gq.subscribers[item.Type] {
			log.Printf("sending message to %s", id)
			go callback(ctx, item)
		}
	}
}

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
	doneRequest *RefreshDoneRepositoryRequest,
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
			content = fmt.Sprintf("<p>%s</p>", doneRequest.Status)
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
	createHookOptions := createHook{
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
) (*AddCommentResponse, error) {
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

	var response AddCommentResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

type RenovateClient struct {
	config string
}

func NewRenovateClient(config string) *RenovateClient {
	return &RenovateClient{config: config}
}

func (rc *RenovateClient) RefreshRepository(ctx context.Context, owner, repository string) error {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}

	envRenovateToken := os.Getenv("GITEA_RENOVATE_TOKEN")
	log.Println(envRenovateToken)

	renovateToken := client.SetSecret("RENOVATE_TOKEN", envRenovateToken)
	githubComToken := client.SetSecret("GITHUB_COM_TOKEN", os.Getenv("GITHUB_COM_TOKEN"))
	renovateSecret := client.SetSecret("RENOVATE_SECRETS", os.Getenv("RENOVATE_SECRETS"))

	output, err := client.Container().
		From("renovate/renovate:latest").
		WithNewFile("/opts/renovate/config.json", dagger.ContainerWithNewFileOpts{
			Contents: `{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "platform": "gitea",
  "endpoint": "https://git.front.kjuulh.io/api/v1/",
  "automerge": true,
  "automergeType": "pr",
  "extends": [
    "config:base"
  ],
  "hostRules": [
    {
      "hostType": "docker",
      "matchHost": "harbor.front.kjuulh.io",
      "username": "service",
      "password": "{{ secrets.HARBOR_SERVER_PASSWORD }}"
    }
  ],
  "packageRules": [
    {
      "matchDatasources": ["docker"],
      "registryUrls": ["https://harbor.front.kjuulh.io/docker-proxy/library/"]
    },
    {
      "groupName": "all dependencies",
      "separateMajorMinor": false,
      "groupSlug": "all",
      "packageRules": [
        {
          "matchPackagePatterns": [
            "*"
          ],
          "groupName": "all dependencies",
          "groupSlug": "all"
        }
      ],
      "lockFileMaintenance": {
        "enabled": false
      }
    }
  ]
}`,
			Permissions: 755,
			Owner:       "root",
		}).
		WithSecretVariable("RENOVATE_TOKEN", renovateToken).
		WithSecretVariable("GITHUB_COM_TOKEN", githubComToken).
		WithSecretVariable("RENOVATE_SECRETS", renovateSecret).
		WithEnvVariable("LOG_LEVEL", "warn").
		WithEnvVariable("RENOVATE_CONFIG_FILE", "/opts/renovate/config.json").
		WithExec([]string{
			fmt.Sprintf("%s/%s", owner, repository),
		}).
		Sync(ctx)

	stdout, outerr := output.Stdout(ctx)
	if outerr == nil {
		log.Printf("stdout: %s", stdout)
	}
	stderr, outerr := output.Stderr(ctx)
	if outerr == nil {
		log.Printf("stderr: %s", stderr)
	}
	if err != nil {
		return fmt.Errorf("error: %w, \nstderr: %s\nstdout: %s", err, stderr, stdout)
	}

	return nil
}
