// This file contains the User struct and its members
// User is used to represent a user in the system, and is used for authentication and authorization.
// The User struct contains the user's ID, username, encrypted password, and a list of scene IDs.
// The scene IDs are used to associate a user with the scenes they have access to.
// Passwords are encrypted and checked using bcrypt.

package user

import (
	"errors"
	"slices"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrSceneIDNotFound is returned when a scene ID is not found in the user's scene list
	ErrSceneIDNotFound = errors.New("scene ID not found in User scene list")
	// ErrSceneIDAlreadyExists is returned when a scene ID is already in the user's scene list
	ErrSceneIDAlreadyExists = errors.New("scene ID already exists in user scene list")
)

// User represents a user in the system
type User struct {
	ID                primitive.ObjectID   `bson:"_id,omitempty"`
	Username          string               `bson:"username"`
	EncryptedPassword string               `bson:"encrypted_password"`
	SceneIDs          []primitive.ObjectID `bson:"scene_ids"`
}

// AddScene adds a scene ID to the user's list of scenes
// Returns an ErrSceneIDAlreadyExists if the scene ID is already in the user's scene list
func (u *User) AddScene(sceneID primitive.ObjectID) error {
	if slices.Contains(u.SceneIDs, sceneID) {
		return ErrSceneIDAlreadyExists
	}
	u.SceneIDs = append(u.SceneIDs, sceneID)
	return nil
}

// RemoveScene removes a scene ID from the user's list of scenes
// Returns an ErrSceneIDNotFound if the scene ID is not found in the user's scene list
func (u *User) RemoveScene(sceneID primitive.ObjectID) error {
	for i, id := range u.SceneIDs {
		if id == sceneID {
			u.SceneIDs = slices.Delete(u.SceneIDs, i, i+1)
			return nil
		}
	}
	return ErrSceneIDNotFound
}

// SetPassword sets a new password for the user. Encrypts the password using bcrypt.
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.EncryptedPassword = string(hashedPassword)
	return nil
}

// CheckPassword verifies if the provided password is correct.
// Returns nil on success, or error on failure
func (u *User) CheckPassword(password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(u.EncryptedPassword), []byte(password))
	return err
}
