
// Package scene contains the implementation of interacting with the MongoDB scene collection.
// The SceneManager struct is responsible for interacting with the MongoDB scenes collection.
// The Scene, TrainingConfig, Video, Sfm, and Nerf structs are used to represent the data stored in the MongoDB database.
// Interaction is primarily by ID, as the ID will (almost always) be unique. BSON is used to interact with the database.
package scene