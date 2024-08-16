package scene

import (
	"context"
	
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

)

type SceneManager struct {
	collection *mongo.Collection
}

func NewSceneManager(client *mongo.Client) *SceneManager {
	return &SceneManager{
		collection: client.Database("nerfdb").Collection("scenes"),
	}
}

func (sm *SceneManager) SetTrainingConfig(ctx context.Context, id string, config *TrainingConfig) error {
	_, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"config": config}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (sm *SceneManager) SetScene(ctx context.Context, id string, scene *Scene) error {
	_, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": scene},
		options.Update().SetUpsert(true),
	)
	return err
}

func (sm *SceneManager) SetVideo(ctx context.Context, id string, vid *Video) error {
	_, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"video": vid}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (sm *SceneManager) SetSfm(ctx context.Context, id string, sfm *Sfm) error {
	_, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"sfm": sfm}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (sm *SceneManager) SetNerf(ctx context.Context, id string, nerf *Nerf) error {
	_, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"nerf": nerf}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (sm *SceneManager) SetSceneName(ctx context.Context, id string, name string) error {
	_, err := sm.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"name": name}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (sm *SceneManager) GetSceneName(ctx context.Context, id string) (string, error) {
	var result struct {
		Name string `bson:"name"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	return result.Name, err
}

func (sm *SceneManager) GetTrainingConfig(ctx context.Context, id string) (*TrainingConfig, error) {
	var result struct {
		Config TrainingConfig `bson:"config"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	return &result.Config, err
}

func (sm *SceneManager) GetScene(ctx context.Context, id string) (*Scene, error) {
	var scene Scene
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&scene)
	return &scene, err
}

func (sm *SceneManager) GetVideo(ctx context.Context, id string) (*Video, error) {
	var result struct {
		Video Video `bson:"video"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	return &result.Video, err
}

func (sm *SceneManager) GetSfm(ctx context.Context, id string) (*Sfm, error) {
	var result struct {
		Sfm Sfm `bson:"sfm"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	return &result.Sfm, err
}

func (sm *SceneManager) GetNerf(ctx context.Context, id string) (*Nerf, error) {
	var result struct {
		Nerf Nerf `bson:"nerf"`
	}
	err := sm.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
	return &result.Nerf, err
}
