// This file contains the expected structure of incoming requests to the API. These structs are used to
// validate incoming requests, provide a consistent interface for handling requests, and to pass data to the
// appropriate handlers.

// Note that all structs are indepedent of the user id. This is because the user id is extracted from the JWT token
// There are a few api endpoints that are not covered by these structs, such as the /user/scene/history endpoint.
// This is because its really only requires the userID, which comes from the JWT token. Worker data is also not included
// but should probably be included in the future.

package common

import (
	"mime/multipart"
)

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

type UpdateUsernameRequest struct {
	Password    string `json:"password" validate:"required"`
	NewUsername string `json:"new_username" validate:"required"`
}

type DeleteSceneRequest struct {
	SceneID string `params:"scene_id" validate:"required"`
}

type DeleteUserRequest struct {
	Password string `json:"password" validate:"required"`
}

type NewSceneRequest struct {
	File            *multipart.FileHeader `form:"file" validate:"required"`
	TrainingMode    string                `form:"training_mode" validate:"required,oneof=gaussian tensorf"`
	OutputTypes     []string              `form:"output_types" validate:"required,dive,validOutputType"`
	SaveIterations  []int                 `form:"save_iterations" validate:"required,dive,min=1,max=30000"`
	TotalIterations int                   `form:"total_iterations" validate:"required,min=1,max=30000"`
	SceneName       string                `form:"scene_name"`
}

type GetSceneMetadataRequest struct {
	SceneID string `params:"scene_id" validate:"required"`
}

type GetSceneOutputRequest struct {
	SceneID    string `params:"scene_id" validate:"required"`
	OutputType string `params:"output_type" validate:"required,oneof=splat_cloud point_cloud video model"`
	Iteration  string `query:"iteration"`
}

type GetSceneThumbnailRequest struct {
	SceneID string `params:"scene_id" validate:"required"`
}

type GetSceneNameRequest struct {
	SceneID string `params:"scene_id" validate:"required"`
}

type GetQueuePositionRequest struct {
	QueueID string `query:"queueid" validate:"required"`
	TaskID  string `query:"id" validate:"required"`
}