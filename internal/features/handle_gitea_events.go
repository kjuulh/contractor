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

func RegisterGiteaQueues(goqueue *queue.GoQueue, renovate *renovate.RenovateClient, giteaClient *providers.GiteaClient) {
	goqueue.Subscribe(
		models.MessageTypeRefreshRepository,
		func(ctx context.Context, item *queue.QueueMessage) error {
			log.Printf("handling message: %s, content: %s", item.Type, item.Content)
			return nil
		},
	)
	goqueue.Subscribe(
		models.MessageTypeRefreshRepositoryDone,
		func(ctx context.Context, item *queue.QueueMessage) error {
			log.Printf("handling message: %s, content: %s", item.Type, item.Content)
			return nil
		},
	)
	goqueue.Subscribe(
		models.MessageTypeRefreshRepository,
		func(ctx context.Context, item *queue.QueueMessage) error {
			var request models.RefreshRepositoryRequest
			if err := json.Unmarshal([]byte(item.Content), &request); err != nil {
				log.Printf("failed to unmarshal request body: %s", err.Error())
				return err
			}

			cancelCtx, cancel := context.WithTimeout(ctx, time.Minute*5)
			defer cancel()

			if err := renovate.RefreshRepository(cancelCtx, request.Owner, request.Repository); err != nil {
				goqueue.Insert(models.MessageTypeRefreshRepositoryDone, models.RefreshDoneRepositoryRequest{
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

			goqueue.Insert(models.MessageTypeRefreshRepositoryDone, models.RefreshDoneRepositoryRequest{
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
		models.MessageTypeRefreshRepositoryDone,
		func(ctx context.Context, item *queue.QueueMessage) error {
			var doneRequest models.RefreshDoneRepositoryRequest
			if err := json.Unmarshal([]byte(item.Content), &doneRequest); err != nil {
				log.Printf("failed to unmarshal request body: %s", err.Error())
				return err
			}

			return giteaClient.EditComment(ctx, &doneRequest)
		},
	)
}
