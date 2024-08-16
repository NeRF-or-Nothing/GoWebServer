package dbschema

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserManager struct {
	collection *mongo.Collection
}

func NewUserManager(client *mongo.Client, unittest bool) *UserManager {
	db := client.Database("nerfdb")
	return &UserManager{
		collection: db.Collection("users"),
	}
}

func (um *UserManager) SetUser(ctx context.Context, user *User) error {
	_, err := um.collection.UpdateOne(
		ctx,
		bson.M{"_id": user.ID},
		bson.M{"$set": user},
		options.Update().SetUpsert(true),
	)
	return err
}

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

func (um *UserManager) GenerateUser(ctx context.Context, username, password string) (*User, error) {
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

func (um *UserManager) GetUserByID(ctx context.Context, id primitive.ObjectID) (*User, error) {
	var user User
	err := um.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (um *UserManager) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := um.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (um *UserManager) UserHasJobAccess(ctx context.Context, userID primitive.ObjectID, jobID string) (bool, error) {
	user, err := um.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, sceneID := range user.SceneIDs {
		if sceneID.Hex() == jobID {
			return true, nil
		}
	}
	return false, nil
}
