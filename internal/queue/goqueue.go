package queue

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
)

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
