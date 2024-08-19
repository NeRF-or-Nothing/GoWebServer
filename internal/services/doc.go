// Package services contains the implementation of the services used by the web server.
// The services are responsible for interacting with the database and performing anything that is not strictly HTTP-related.
// The services are injected into the handlers, which are responsible for handling HTTP requests.
//   - AMPQService:
//     Is a ampq 0.9.1 broker-agnostic handler that is used to consume from / publish to additional workers (sfm, nerf, etc)
//   - ClientService:
//     Is the main handler for dispatched http requests to the client. It is responsible for handling requests to the client,
//     such as getting the user's scenes, starting a job, and much more
package services