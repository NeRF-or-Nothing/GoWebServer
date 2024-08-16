package dbschema

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrInvalidQueueID     = errors.New("not a valid queue ID")
	ErrIDAlreadyInQueue   = errors.New("ID is already in the queue")
	ErrIDNotFoundInQueue  = errors.New("ID not found in queue")
	ErrMultipleIDsInQueue = errors.New("same ID found multiple times in queue")
	ErrQueueEmpty         = errors.New("queue is empty")
)

type QueueListManager struct {
	collection *mongo.Collection
	queueNames []string
}

// contains checks if a string is in a slice of strings
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

func NewQueueListManager(client *mongo.Client, unittest bool) *QueueListManager {
	db := client.Database("nerfdb")
	return &QueueListManager{
		collection: db.Collection("queues"),
		queueNames: []string{"sfm_list", "nerf_list", "queue_list"},
	}
}

func (qlm *QueueListManager) setQueue(ctx context.Context, id string, queueList *QueueList) error {
	if !contains(qlm.queueNames, id) {
		return ErrInvalidQueueID
	}

	_, err := qlm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": queueList},
		options.Update().SetUpsert(true),
	)
	return err
}

func (qlm *QueueListManager) AppendQueue(ctx context.Context, queueID, uuid string) error {
	if !contains(qlm.queueNames, queueID) {
		return ErrInvalidQueueID
	}

	var queueList QueueList
	err := qlm.collection.FindOne(ctx, bson.M{"_id": queueID}).Decode(&queueList)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			queueList = QueueList{ID: queueID, Queue: []string{uuid}}
			return qlm.setQueue(ctx, queueID, &queueList)
		}
		return err
	}

	if contains(queueList.Queue, uuid) {
		return ErrIDAlreadyInQueue
	}

	queueList.Queue = append(queueList.Queue, uuid)
	return qlm.setQueue(ctx, queueID, &queueList)
}

func (qlm *QueueListManager) GetQueuePosition(ctx context.Context, queueID, uuid string) (int, int, error) {
	if !contains(qlm.queueNames, queueID) {
		return 0, 0, ErrInvalidQueueID
	}

	var queueList QueueList
	err := qlm.collection.FindOne(ctx, bson.M{"_id": queueID}).Decode(&queueList)
	if err != nil {
		return 0, 0, err
	}

	position := -1
	for i, id := range queueList.Queue {
		if id == uuid {
			if position != -1 {
				return 0, 0, ErrMultipleIDsInQueue
			}
			position = i
		}
	}

	if position == -1 {
		return 0, 0, ErrIDNotFoundInQueue
	}

	return position, len(queueList.Queue), nil
}

func (qlm *QueueListManager) GetQueueSize(ctx context.Context, queueID string) (int, error) {
	if !contains(qlm.queueNames, queueID) {
		return 0, ErrInvalidQueueID
	}

	var queueList QueueList
	err := qlm.collection.FindOne(ctx, bson.M{"_id": queueID}).Decode(&queueList)
	if err != nil {
		return 0, err
	}

	return len(queueList.Queue), nil
}

func (qlm *QueueListManager) PopQueue(ctx context.Context, queueID string, uuid *string) error {
	if !contains(qlm.queueNames, queueID) {
		return ErrInvalidQueueID
	}

	var queueList QueueList
	err := qlm.collection.FindOne(ctx, bson.M{"_id": queueID}).Decode(&queueList)
	if err != nil {
		return err
	}

	if len(queueList.Queue) == 0 {
		return ErrQueueEmpty
	}

	if uuid != nil {
		index := -1
		for i, id := range queueList.Queue {
			if id == *uuid {
				index = i
				break
			}
		}
		if index == -1 {
			return ErrIDNotFoundInQueue
		}
		queueList.Queue = append(queueList.Queue[:index], queueList.Queue[index+1:]...)
	} else {
		queueList.Queue = queueList.Queue[1:]
	}

	return qlm.setQueue(ctx, queueID, &queueList)
}
