// Package queue contains the implementation of processing queues of jobs in the MongoDB database.
// The QueueListManager struct is responsible for interacting with the MongoDB queues collection.
// The QueueList struct is used to represent a list of items in a queue, and is (currently) used for reporting job processing progress.
// Strings are used to interact with the database.
package queue