package user

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/log"
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
	fmt.Println("UserManager.GetUserByUsername")
	var user User
	err := um.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			fmt.Println("User not found")
			return nil, ErrUserNotFound
		}
		fmt.Println("GetUserByUsername Error")
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
