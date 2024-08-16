package dbschema

import ()

/*
QueueList represents a list of items in a queue.
Used for reporting job processing progress.
*/
type QueueList struct {
	ID    string   `bson:"_id"`
	Queue []string `bson:"queue"`
}
