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

type VideoUploadRequest struct {
    File            *multipart.FileHeader `form:"file" validate:"required"`
    TrainingMode    string                `form:"training_mode" validate:"required,oneof=gaussian tensorf"`
    OutputTypes     []string              `form:"output_types" validate:"required,dive,validOutputType"`
    SaveIterations  []int                 `form:"save_iterations" validate:"required,dive,min=1,max=30000"`
    TotalIterations int                   `form:"total_iterations" validate:"required,min=1,max=30000"`
    SceneName       string                `form:"scene_name"`
}

type GetNerfJobMetadataRequest struct {
    SceneID    string `params:"scene_id" validate:"required"`
    OutputType string `query:"output_type,omitempty"`
}

type GetNerfResourceRequest struct {
    OutputType string `params:"output_type" validate:"required"`
    SceneID    string `params:"scene_id" validate:"required"`
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