package features

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"git.front.kjuulh.io/kjuulh/contractor/internal/models"
	"git.front.kjuulh.io/kjuulh/contractor/internal/providers"
	"git.front.kjuulh.io/kjuulh/contractor/internal/queue"
	"git.front.kjuulh.io/kjuulh/contractor/internal/renovate"
)

func RegisterGitHubQueues(goqueue *queue.GoQueue, renovate *renovate.RenovateClient, giteaClient *providers.GitHubClient) {
	goqueue.Subscribe(
		models.MessageTypeRefreshGitHubRepository,
		func(ctx context.Context, item *queue.QueueMessage) error {
			log.Printf("handling message: %s, content: %s", item.Type, item.Content)
			return nil
		},
	)
	goqueue.Subscribe(
		models.MessageTypeRefreshGitHubRepositoryDone,
		func(ctx context.Context, item *queue.QueueMessage) error {
			log.Printf("handling message: %s, content: %s", item.Type, item.Content)
			return nil
		},
	)
	goqueue.Subscribe(
		models.MessageTypeRefreshGitHubRepository,
		func(ctx context.Context, item *queue.QueueMessage) error {
			var request models.RefreshGitHubRepositoryRequest
			if err := json.Unmarshal([]byte(item.Content), &request); err != nil {
				log.Printf("failed to unmarshal request body: %s", err.Error())
				return err
			}

			cancelCtx, cancel := context.WithTimeout(ctx, time.Minute*5)
			defer cancel()

			if err := renovate.RefreshRepository(cancelCtx, request.Owner, request.Repository); err != nil {
				goqueue.Insert(models.MessageTypeRefreshGitHubRepositoryDone, models.RefreshGitHubRepositoryDoneRequest{
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

			goqueue.Insert(models.MessageTypeRefreshGitHubRepositoryDone, models.RefreshGitHubRepositoryDoneRequest{
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

	goqueue.Subscribe(
		models.MessageTypeRefreshGitHubRepositoryDone,
		func(ctx context.Context, item *queue.QueueMessage) error {
			var doneRequest models.RefreshGitHubRepositoryDoneRequest
			if err := json.Unmarshal([]byte(item.Content), &doneRequest); err != nil {
				log.Printf("failed to unmarshal request body: %s", err.Error())
				return err
			}

			return giteaClient.EditComment(ctx, &doneRequest)
		},
	)
}
