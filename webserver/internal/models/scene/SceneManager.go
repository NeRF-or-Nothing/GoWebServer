package scene

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Custom errors
var (
	ErrSceneNotFound       = errors.New("scene not found")
	ErrVideoNotFound       = errors.New("video not found")
	ErrSfmNotFound         = errors.New("sfm not found")
	ErrNerfNotFound        = errors.New("nerf not found")
	ErrTrainingConfigNotFound = errors.New("training config not found")
)

type SceneManager struct {
	collection *mongo.Collection
}

func NewSceneManager(client *mongo.Client) *SceneManager {
	return &SceneManager{
		collection: client.Database("nerfdb").Collection("scenes"),
	}
}

func (sm *SceneManager) SetTrainingConfig(ctx context.Context, id primitive.ObjectID, config *TrainingConfig) error {
	result, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"config": config}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return ErrSceneNotFound
	}
	return nil
}

func (sm *SceneManager) SetScene(ctx context.Context, id primitive.ObjectID, scene *Scene) error {
	result, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": scene},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return ErrSceneNotFound
	}
	return nil
}

func (sm *SceneManager) SetVideo(ctx context.Context, id primitive.ObjectID, vid *Video) error {
	result, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"video": vid}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return ErrSceneNotFound
	}
	return nil
}

func (sm *SceneManager) SetSfm(ctx context.Context, id primitive.ObjectID, sfm *Sfm) error {
	result, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"sfm": sfm}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return ErrSceneNotFound
	}
	return nil
}

func (sm *SceneManager) SetNerf(ctx context.Context, id primitive.ObjectID, nerf *Nerf) error {
	result, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"nerf": nerf}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return ErrSceneNotFound
	}
	return nil
}

func (sm *SceneManager) SetSceneName(ctx context.Context, id primitive.ObjectID, name string) error {
	result, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"name": name}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 && result.UpsertedCount == 0 {
		return ErrSceneNotFound
	}
	return nil
}

func (sm *SceneManager) GetSceneName(ctx context.Context, id primitive.ObjectID) (string, error) {
	var result struct {
		Name string `bson:"name"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", ErrSceneNotFound
		}
		return "", err
	}
	return result.Name, nil
}

func (sm *SceneManager) GetTrainingConfig(ctx context.Context, id primitive.ObjectID) (*TrainingConfig, error) {
	var result struct {
		Config *TrainingConfig `bson:"config"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSceneNotFound
		}
		return nil, err
	}
	if result.Config == nil {
		return nil, ErrTrainingConfigNotFound
	}
	return result.Config, nil
}

func (sm *SceneManager) GetScene(ctx context.Context, id primitive.ObjectID) (*Scene, error) {
	var scene Scene
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&scene)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSceneNotFound
		}
		return nil, err
	}
	return &scene, nil
}

func (sm *SceneManager) GetVideo(ctx context.Context, id primitive.ObjectID) (*Video, error) {
	var result struct {
		Video *Video `bson:"video"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSceneNotFound
		}
		return nil, err
	}
	if result.Video == nil {
		return nil, ErrVideoNotFound
	}
	return result.Video, nil
}

func (sm *SceneManager) GetSfm(ctx context.Context, id primitive.ObjectID) (*Sfm, error) {
	var result struct {
		Sfm *Sfm `bson:"sfm"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSceneNotFound
		}
		return nil, err
	}
	if result.Sfm == nil {
		return nil, ErrSfmNotFound
	}
	return result.Sfm, nil
}

func (sm *SceneManager) GetNerf(ctx context.Context, id primitive.ObjectID) (*Nerf, error) {
	var result struct {
		Nerf *Nerf `bson:"nerf"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrSceneNotFound
		}
		return nil, err
	}
	if result.Nerf == nil {
		return nil, ErrNerfNotFound
	}
	return result.Nerf, nil
}

func (sm *SceneManager) DeleteScene(ctx context.Context, id primitive.ObjectID) error {
	result, err := sm.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrSceneNotFound
	}
	return nil
}