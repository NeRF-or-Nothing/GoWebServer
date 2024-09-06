// This file contains the UserManager implementation, which is responsible for interacting with the MongoDB users collection.
// The UserManager struct contains a pointer to the nerfdb.users MongoDB collection and a logger. It provides methods to set, get
// and update user data in the database. Interaction with users is almost always by ID, as the ID will (almost always) be unique.
// There is limited functionality for updating user data, as the only fields that can be updated are the username and password.

package user

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/NeRF-or-Nothing/go-web-server/internal/log"
)

var (
	// ErrUserNotFound is returned when a requested user is not found in the database.
	ErrUserNotFound = errors.New("user not found")
	// ErrUsernameTaken is returned when a username is already taken.
	ErrUsernameTaken = errors.New("username is already taken")
	// ErrUserNoAccess is returned when a user does not have access to a scene (i.e, scene ID not found in user's scene list).
	ErrUserNoAccess = errors.New("user does not have access to this scene")
)


type UserManager struct {
	collection *mongo.Collection
	logger     *log.Logger
}

// NewUserManager creates a new instance of UserManager.
func NewUserManager(client *mongo.Client, logger *log.Logger, unittest bool) *UserManager {
	db := client.Database("nerfdb")
	return &UserManager{
		collection: db.Collection("users"),
		logger:     logger,
	}
}

// SetUser updates or inserts a user document in the database.
// Returns nil if successful, or an error if an error occurred while updating the user.
func (um *UserManager) SetUser(ctx context.Context, user *User) error {
	_, err := um.collection.UpdateOne(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": user},
		options.Update().SetUpsert(true),
	)
	return err
}

// UpdateUser updates an existing user document in the database.
func (um *UserManager) UpdateUser(ctx context.Context, user *User) error {
	result, err := um.collection.UpdateOne(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": user},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrUserNotFound
	}
	return nil
}

// GenerateUser generates a new user document with the given username and password,
// and inserts it into the database. Returns the User, nil if successful.
// Returns nil, error if the username is already taken or an error occurred while inserting the user.
func (um *UserManager) GenerateUser(ctx context.Context, username, password string) (*User, error) {
	// Check if username is already taken
	_, err := um.GetUserByUsername(ctx, username)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			return nil, err
		}
	} else {
		return nil, ErrUsernameTaken
	}

	id := primitive.NewObjectID()
	user := &User{
		ID:       id,
		Username: username,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	if err := um.SetUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves a user from the database based on the given ID.
func (um *UserManager) GetUserByID(ctx context.Context, userID primitive.ObjectID) (*User, error) {
	var user User
	err := um.collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user from the database based on the given username.
// Returns the User, nil if successful. Returns nil, error if the user is not found.
func (um *UserManager) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := um.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			fmt.Println("User not found")
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// UserHasJobAccess checks if a user has access to a job by searching for the job ID in the user's sceneIDs.
func (um *UserManager) UserHasJobAccess(ctx context.Context, userID, jobID primitive.ObjectID) (bool, error) {
	user, err := um.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, sceneID := range user.SceneIDs {
		if sceneID == jobID {
			return true, nil
		}
	}
	return false, nil
}

// UpdatePassword updates the user's password. Verifies the old password before setting the new password.
// Returns nil if successful, or an error if the old password is incorrect or an error occurred while updating the password.
func (um *UserManager) UpdatePassword(ctx context.Context, userID primitive.ObjectID, oldPassword, newPassword string) error {
	user, err := um.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	
	err = user.CheckPassword(oldPassword)
	if err != nil {
		return err
	}
	return user.SetPassword(newPassword)
}

// UpdateUsername updates the user's username. Checks if the new username is already taken.
// Requires the user's password to verify the change.
// Returns nil if successful, or an error if the new username is already taken or an error occurred while updating the username.
func (um *UserManager) UpdateUsername(ctx context.Context, userID primitive.ObjectID, userPassword, newUsername string) error {
	_, err := um.GetUserByUsername(ctx, newUsername)
	if err == nil {
		return ErrUsernameTaken
	}
	
	user, err := um.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	err = user.CheckPassword(userPassword)
	if err != nil {
		return err
	}

	user.Username = newUsername
	return um.UpdateUser(ctx, user)
}
