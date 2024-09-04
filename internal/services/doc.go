// Package services contains the implementation of all services used by the web server.
//
// The services are responsible for interacting with the database and performing anything that is not strictly HTTP-related.
// The services are injected into the web server, and are used to handle requests dispatched by it.
//
// Current services include:
//   - AMPQService:
//     Is a ampq 0.9.1 broker-agnostic handler that is used to consume from / publish to additional workers (sfm, nerf, etc)
//   - ClientService:
//     Is the main handler for dispatched http requests to the client. It is responsible for handling requests to the client,
//     such as getting the user's scenes, starting a job, and much more
package services