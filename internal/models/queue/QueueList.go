// This file contains the QueueList struct and its members
// QueueList is used to represent a list of items in a queue, and is (currently) used for reporting job processing progress.

package queue

// QueueList represents a list of items in a queue.
// Used for reporting job processing progress.
type QueueList struct {
	ID    string   `bson:"_id"`
	Queue []string `bson:"queue"`
}

