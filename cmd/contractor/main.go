package contractor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
			client := http.DefaultClient
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
				return
			}
			bodyReader := bytes.NewReader(body)

			request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/hooks", strings.TrimSuffix(url, "/"), owner, repository), bodyReader)
			if err != nil {
				log.Println("failed to form create hook request: %s", err.Error())
				return
			}
			request.Header.Add("Authorization", fmt.Sprintf("token %s", token))
			request.Header.Add("Content-Type", "application/json")

			resp, err := client.Do(request)
			if err != nil {
				log.Printf("failed to register hook: %s", err.Error())
				return
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
		},
	}

	cmd.Flags().StringVarP(&owner, "owner", "o", "", "the owner for which the repository belongs")
	cmd.Flags().StringVarP(&repository, "repository", "p", "", "the repository to install")
	cmd.Flags().StringVar(&serverType, "server-type", "gitea", "the server type to use [gitea, github]")
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

	cmd := &cobra.Command{
		Use: "server",
	}

	cmd.PersistentFlags().StringVar(&url, "url", "", "the api url of the server")
	cmd.PersistentFlags().StringVar(&token, "token", "", "the token to authenticate with")

	cmd.AddCommand(serverServeCmd())

	return cmd
}

func serverServeCmd() *cobra.Command {
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
					}

					ctx.Status(204)
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
