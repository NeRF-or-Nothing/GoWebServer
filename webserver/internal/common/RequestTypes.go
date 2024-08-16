package common

import "net/http"

type LoginRequest struct {
    Username string `form:"username" binding:"required"`
    Password string `form:"password" binding:"required"`
}

type RegisterRequest struct {
    Username string `form:"username" binding:"required"`
    Password string `form:"password" binding:"required"`
}

type VideoUploadRequest struct {
    File           *http.File `form:"file" binding:"required"`
    TrainingMode   string     `form:"training_mode" binding:"required,oneof=gaussian tensorf"`
    OutputTypes    []string   `form:"output_types" binding:"required,dive,validOutputType"`
    SaveIterations []int      `form:"save_iterations" binding:"required,dive,min=1,max=30000"`
    TotalIterations int       `form:"total_iterations" binding:"required,min=1,max=30000"`
    SceneName      string     `form:"scene_name"`
}

type GetNerfMetadataRequest struct {
    SceneID     string `uri:"scene_id" binding:"required"`
    OutputType  string `form:"output_type,omitempty"`
}

type GetNerfTypeMetadataRequest struct {
    OutputType string `uri:"output_type" binding:"required"`
    SceneID    string `uri:"scene_id" binding:"required"`
}

type GetNerfResourceRequest struct {
    OutputType string `uri:"output_type" binding:"required"`
    SceneID    string `uri:"scene_id" binding:"required"`
    Iteration  string `form:"iteration"`
}

type GetPreviewRequest struct {
    SceneID string `uri:"scene_id" binding:"required"`
}

type GetQueuePositionRequest struct {
    QueueID string `form:"queueid" binding:"required"`
    TaskID  string `form:"id" binding:"required"`
}
