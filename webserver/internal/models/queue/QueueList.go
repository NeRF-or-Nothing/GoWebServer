package queue

import "errors"

var (
	ErrInvalidQueueID     = errors.New("not a valid queue ID")
	ErrIDAlreadyInQueue   = errors.New("ID is already in the queue")
	ErrIDNotFoundInQueue  = errors.New("ID not found in queue")
	ErrMultipleIDsInQueue = errors.New("same ID found multiple times in queue")
	ErrQueueEmpty         = errors.New("queue is empty")
)

/*
QueueList represents a list of items in a queue.
Used for reporting job processing progress.
*/
type QueueList struct {
	ID    string   `bson:"_id"`
	Queue []string `bson:"queue"`
}

