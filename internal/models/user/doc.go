// Package user contains the implementation of interacting with the MongoDB user collection. 
// The UserManager struct is responsible for interacting with the MongoDB users collection. It is CRUD for the user collection.
// The User struct is used to represent a user and their scenes.
// Interaction is primarily by ID, as the ID will (almost always) be unique. BSON is used to interact with the database.
package user