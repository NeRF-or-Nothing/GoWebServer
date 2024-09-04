// This file contains the QueueListManager implementation, which is responsible for interacting with the MongoDB queues collection.
// The QueueListManager struct contains a pointer to the nerfdb.queues MongoDB collection and a logger. It provides methods to set and
// get queue data from the database. Interaction with queues is almost always by ID, as the ID will (almost always) be unique.

// Note that the only valid queues are those in the queueNames slice.

package queue

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/NeRF-or-Nothing/go-web-server/internal/log"
)

// Custom errors
var (
	// ErrInvalidQueueID is returned when an invalid queue ID is used.
	ErrInvalidQueueID = errors.New("not a valid queue ID")
	// ErrQueueAlreadyExists is returned when a queue with the same ID already exists.
	ErrQueueAlreadyExists = errors.New("queue already exists")
	// ErrIDAlreadyInQueue is returned when itemID is already in the queue.
	ErrIDAlreadyInQueue = errors.New("ID is already in the queue")
	// ErrIDNotFoundInQueue is returned when itemID is not in the queue.
	ErrIDNotFoundInQueue = errors.New("ID not found in queue")
	// ErrMultipleIDsInQueue is returned when the same ID is found multiple times in the queue.
	ErrMultipleIDsInQueue = errors.New("same ID found multiple times in queue")
	// ErrInvalidOpOnEmptyQueue is returned when an invalid operation occurs on an empty queue.
	ErrInvalidOpOnEmptyQueue = errors.New("invalid operation on empty queue")
)

type QueueListManager struct {
	collection *mongo.Collection
	queueNames []string
	logger     *log.Logger
}

// NewQueueListManager creates a new QueueListManager with the given MongoDB client and logger.
// By default, creates 'sfm_list', 'nerf_list', and 'queue_list' queues.
func NewQueueListManager(client *mongo.Client, logger *log.Logger, unittest bool) *QueueListManager {
	db := client.Database("nerfdb")
	return &QueueListManager{
		collection: db.Collection("queues"),
		queueNames: []string{"queue_list", "sfm_list", "nerf_list"},
		logger:     logger,
	}
}

// GetQueueNames returns the list of valid queue names.
func (qlm *QueueListManager) GetQueueNames() []string {
	return qlm.queueNames
}

// setQueue sets the data in queueList in the database by the queue ID.
// It is not intended to be used outside of the QueueListManager.
func (qlm *QueueListManager) setQueue(ctx context.Context, queueID string, queueList *QueueList) error {
	if !slices.Contains(qlm.queueNames, queueID) {
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
	if slices.Contains(qlm.queueNames, queueID) {
		return ErrQueueAlreadyExists
	}

	qlm.queueNames = append(qlm.queueNames, queueID)

	queueList := QueueList{ID: queueID, Queue: []primitive.ObjectID{}}
	return qlm.setQueue(ctx, queueID, &queueList)
}

// GetQueuePosition gets the position of itemID in the queue by the queue ID.
// Returns the position of the item in the queue and the total number of items in the queue.
func (qlm *QueueListManager) GetQueuePosition(ctx context.Context, queueID string, itemID primitive.ObjectID) (int, int, error) {
	if !slices.Contains(qlm.queueNames, queueID) {
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
	if !slices.Contains(qlm.queueNames, queueID) {
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
// If the queue does not exist, and queueID is valid, it is created, and the item is added.
func (qlm *QueueListManager) AppendToQueue(ctx context.Context, queueID string, itemID primitive.ObjectID) error {
	if !slices.Contains(qlm.queueNames, queueID) {
		qlm.logger.Info("Invalid queue ID")
		return ErrInvalidQueueID
	}

	var queueList QueueList
	err := qlm.collection.FindOne(ctx, bson.M{"_id": queueID}).Decode(&queueList)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			queueList = QueueList{ID: queueID, Queue: []primitive.ObjectID{itemID}}
			return qlm.setQueue(ctx, queueID, &queueList)
		}
		return err
	}

	if slices.Contains(queueList.Queue, itemID) {
		qlm.logger.Info(fmt.Sprintf("Attemped to add %s to queue %s, but it is already in the queue", itemID, queueID))
		return ErrIDAlreadyInQueue
	}

	queueList.Queue = append(queueList.Queue, itemID)
	return qlm.setQueue(ctx, queueID, &queueList)
}

// PopFromQueue pops the itemID from the queue by the queue ID.
// Returns ErrIDNotFoundInQueue if the itemID is not in the queue.
func (qlm *QueueListManager) DeleteFromQueue(ctx context.Context, queueID string, itemID primitive.ObjectID) error {
	if !slices.Contains(qlm.queueNames, queueID) {
		return ErrInvalidQueueID
	}

	var queueList QueueList
	err := qlm.collection.FindOne(ctx, bson.M{"_id": queueID}).Decode(&queueList)
	if err != nil {
		return err
	}

	if len(queueList.Queue) == 0 {
		return ErrInvalidOpOnEmptyQueue
	}

	index := slices.IndexFunc(queueList.Queue, func(id primitive.ObjectID) bool {
		return id == itemID
	})

	if index == -1 {
		return ErrIDNotFoundInQueue
	}

	queueList.Queue = slices.Delete(queueList.Queue, index, index+1)

	return qlm.setQueue(ctx, queueID, &queueList)
}