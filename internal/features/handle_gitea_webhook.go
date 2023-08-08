package features

import (
	"context"
	"log"
	"strings"

	"git.front.kjuulh.io/kjuulh/contractor/internal/bot"
	"git.front.kjuulh.io/kjuulh/contractor/internal/models"
	"git.front.kjuulh.io/kjuulh/contractor/internal/queue"
)

type GiteaWebhook struct {
	botHandler *bot.BotHandler
	queue      *queue.GoQueue
}

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

func NewGiteaWebhook(botHandler *bot.BotHandler, queue *queue.GoQueue) *GiteaWebhook {
	return &GiteaWebhook{
		botHandler: botHandler,
		queue:      queue,
	}
}

func (gw *GiteaWebhook) HandleGiteaWebhook(ctx context.Context, request *GiteaWebhookRequest) error {
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
		)
		if err != nil {
			return err
		}

		if err := gw.queue.Insert(models.MessageTypeRefreshRepository, models.RefreshRepositoryRequest{
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

func validateBotComment(s string) (request string, ok bool) {
	if after, ok := strings.CutPrefix(s, "/contractor"); ok {
		return strings.TrimSpace(after), true
	}

	return "", false
}
