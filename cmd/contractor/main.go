package contractor

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"git.front.kjuulh.io/kjuulh/contractor/internal/bot"
	"git.front.kjuulh.io/kjuulh/contractor/internal/features"
	"git.front.kjuulh.io/kjuulh/contractor/internal/providers"
	"git.front.kjuulh.io/kjuulh/contractor/internal/queue"
	"git.front.kjuulh.io/kjuulh/contractor/internal/renovate"
)

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
			if err := providers.NewGiteaClient(&url, &token).CreateWebhook(owner, repository); err != nil {
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

		githubAppID          int64
		githubInstallationID int64
		githubPrivateKeyPath string
	)

	giteaClient := providers.NewGiteaClient(&url, &token)
	githubClient := providers.NewGitHubClient(&githubAppID, &githubInstallationID, &githubPrivateKeyPath)
	renovateClient := renovate.NewRenovateClient("")
	queue := queue.NewGoQueue()
	botHandler := bot.NewBotHandler(giteaClient, githubClient)

	giteaWebhook := features.NewGiteaWebhook(botHandler, queue)
	githubWebhook := features.NewGitHubWebhook(botHandler, queue)

	features.RegisterGiteaQueues(queue, renovateClient, giteaClient)
	features.RegisterGitHubQueues(queue, renovateClient, githubClient)

	cmd := &cobra.Command{
		Use: "server",
	}

	cmd.PersistentFlags().StringVar(&url, "url", "", "the api url of the server")
	cmd.PersistentFlags().StringVar(&token, "token", "", "the token to authenticate with")

	cmd.PersistentFlags().Int64Var(&githubAppID, "github-app-id", 0, "github app id to authenticate with")
	cmd.PersistentFlags().Int64Var(&githubInstallationID, "github-installation-id", 0, "github installation id to authenticate with")
	cmd.PersistentFlags().StringVar(&githubPrivateKeyPath, "github-private-key-path", "", "path to the github app private key")

	cmd.AddCommand(serverServeCmd(&url, &token, giteaWebhook, githubWebhook))

	return cmd
}

func serverServeCmd(
	url *string,
	token *string,
	giteaWebhook *features.GiteaWebhook,
	githubWebhook *features.GitHubWebhook,
) *cobra.Command {
	cmd := &cobra.Command{
		Use: "serve",
		Run: func(cmd *cobra.Command, args []string) {
			engine := gin.Default()

			github := engine.Group("/github")
			{
				github.POST("/webhook", func(ctx *gin.Context) {
					var request features.GitHubWebhookRequest
					if err := ctx.BindJSON(&request); err != nil {
						ctx.AbortWithError(500, err)
						return
					}

					if err := githubWebhook.HandleGitHubWebhook(ctx.Request.Context(), &request); err != nil {
						ctx.AbortWithError(500, err)
						return
					}

					ctx.Status(204)
				})
			}

			gitea := engine.Group("/gitea")
			{
				gitea.POST("/webhook", func(ctx *gin.Context) {
					var request features.GiteaWebhookRequest
					if err := ctx.BindJSON(&request); err != nil {
						ctx.AbortWithError(500, err)
						return
					}

					if err := giteaWebhook.HandleGiteaWebhook(ctx.Request.Context(), &request); err != nil {
						ctx.AbortWithError(500, err)
						return
					}

					ctx.Status(204)
				})
			}

			engine.Run("0.0.0.0:9111")
		},
	}

	return cmd
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "contractor"}

	cmd.AddCommand(installCmd(), serverCmd())

	return cmd
}
