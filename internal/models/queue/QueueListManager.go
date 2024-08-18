// This file contains the QueueListManager implementation, which is responsible for interacting with the MongoDB queues collection.
// The QueueListManager struct contains a pointer to the nerfdb.queues MongoDB collection and a logger. It provides methods to set and
// get queue data from the database. Interaction with queues is almost always by ID, as the ID will (almost always) be unique.

// Note that the only valid queues are those in the queueNames slice.
// Contrary to the rest of the mongodb code, the QueueListManager does not use the bson package, but rather strings and 
// slices of strings to interact with the database.

package queue

import (
	"fmt"
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/log"
)

// Custom errors
var (
	ErrInvalidQueueID     = errors.New("not a valid queue ID")
	ErrQueueAlreadyExists = errors.New("queue already exists")
	ErrIDAlreadyInQueue   = errors.New("ID is already in the queue")
	ErrIDNotFoundInQueue  = errors.New("ID not found in queue")
	ErrMultipleIDsInQueue = errors.New("same ID found multiple times in queue")
	ErrQueueEmpty         = errors.New("queue is empty")
)

type QueueListManager struct {
	collection *mongo.Collection
	queueNames []string
	logger     *log.Logger
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

// NewQueueListManager creates a new QueueListManager with the given MongoDB client and logger.
// By default, creates 'sfm_list', 'nerf_list', and 'queue_list' queues.
func NewQueueListManager(client *mongo.Client, logger *log.Logger, unittest bool) *QueueListManager {
	db := client.Database("nerfdb")
	return &QueueListManager{
		collection: db.Collection("queues"),
		queueNames: []string{"sfm_list", "nerf_list", "queue_list"},
		logger:     logger,
	}
}

// setQueue sets the data in queueList in the database by the queue ID.
// It is not intended to be used outside of the QueueListManager.
func (qlm *QueueListManager) setQueue(ctx context.Context, queueID string, queueList *QueueList) error {
	if !contains(qlm.queueNames, queueID) {
		qlm.logger.Info("Invalid queue ID")
		return ErrInvalidQueueID
	}

	_, err := qlm.collection.UpdateOne(
		ctx,
		bson.M{"_id": queueID},
		bson.M{"$set": queueList},
		options.Update().SetUpsert(true),
	)
	return err
}

// AddNewQueue adds a new queue to the database by the queue ID. The queueID is added to  qlm.queueNames slice.
// If the queue already exists, returns ErrQueueAlreadyExists.
func (qlm *QueueListManager) AddNewQueue(ctx context.Context, queueID string) error {
	if contains(qlm.queueNames, queueID) {
		return ErrQueueAlreadyExists
	}

	qlm.queueNames = append(qlm.queueNames, queueID)

	queueList := QueueList{ID: queueID, Queue: []string{}}
	return qlm.setQueue(ctx, queueID, &queueList)
}

// GetQueuePosition gets the position of itemID in the queue by the queue ID.
// Returns the position of the item in the queue and the total number of items in the queue.
func (qlm *QueueListManager) GetQueuePosition(ctx context.Context, queueID, itemID string) (int, int, error) {
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
		if id == itemID {
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

// GetQueueSize returns the number of items in the queue by the queue ID.
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


// AppendToQueue appends a item's ID to the queue by the queue ID.
// Returns ErrIDAlreadyInQueue if the itemID is already in the queue.
// If the queue does not exist, and queueID is valid, it is created.
func (qlm *QueueListManager) AppendToQueue(ctx context.Context, queueID, itemID string) error {
	if !contains(qlm.queueNames, queueID) {
		qlm.logger.Info("Invalid queue ID")
		return ErrInvalidQueueID
	}

	var queueList QueueList
	err := qlm.collection.FindOne(ctx, bson.M{"_id": queueID}).Decode(&queueList)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			queueList = QueueList{ID: queueID, Queue: []string{itemID}}
			return qlm.setQueue(ctx, queueID, &queueList)
		}
		return err
	}

	if contains(queueList.Queue, itemID) {
		qlm.logger.Info(fmt.Sprintf("Attemped to add %s to queue %s, but it is already in the queue", itemID, queueID))
		return ErrIDAlreadyInQueue
	}

	queueList.Queue = append(queueList.Queue, itemID)
	return qlm.setQueue(ctx, queueID, &queueList)
}


// PopFromQueue pops the itemID from the queue by the queue ID. 
// Returns ErrIDNotFoundInQueue if the itemID is not in the queue.
// If itemID is nil, the first item in the queue is popped.
func (qlm *QueueListManager) PopFromQueue(ctx context.Context, queueID string, itemID *string) error {
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

	if itemID != nil {
		index := -1
		for i, id := range queueList.Queue {
			if id == *itemID {
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
