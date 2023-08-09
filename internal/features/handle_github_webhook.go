package features

import (
	"context"
	"log"
	"strings"

	"git.front.kjuulh.io/kjuulh/contractor/internal/bot"
	"git.front.kjuulh.io/kjuulh/contractor/internal/models"
	"git.front.kjuulh.io/kjuulh/contractor/internal/queue"
)

type GitHubWebhook struct {
	botHandler *bot.BotHandler
	queue      *queue.GoQueue
}

type GitHubWebhookRequest struct {
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

func NewGitHubWebhook(botHandler *bot.BotHandler, queue *queue.GoQueue) *GitHubWebhook {
	return &GitHubWebhook{
		botHandler: botHandler,
		queue:      queue,
	}
}

func (gw *GitHubWebhook) HandleGitHubWebhook(ctx context.Context, request *GitHubWebhookRequest) error {
	command, ok := validateBotComment(request.Comment.Body)
	if ok {
		log.Printf("got webhook request: contractor %s", command)

		bot := gw.botHandler
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
			models.SupportedBackendGitHub,
		)
		if err != nil {
			return err
		}

		if err := gw.queue.Insert(models.MessageTypeRefreshGitHubRepository, models.RefreshGitHubRepositoryRequest{
			Repository:     parts[1],
			Owner:          parts[0],
			PullRequestID:  request.Issue.Number,
			CommentID:      comment.ID,
			CommentBody:    comment.Body,
			ReportProgress: true,
		}); err != nil {
			return err
		}
	}

	return nil
}
