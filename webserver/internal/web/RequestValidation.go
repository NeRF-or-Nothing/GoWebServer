// RequestValidation.go

package web

import (
    "errors"
    "net/http"
    "strconv"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/go-playground/validator/v10"

    "github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/dbschema"
)

var validate *validator.Validate

func init() {
    validate = validator.New()
    validate.RegisterValidation("validOutputType", validateOutputType)
}

func validateOutputType(fl validator.FieldLevel) bool {
    outputType := fl.Field().String()
    trainingMode := fl.Parent().FieldByName("TrainingMode").String()
    return dbschema.Nerf{}.IsValidOutputType(trainingMode, outputType)
}

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
    SceneID string `uri:"scene_id" binding:"required"`
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

func ValidateRequest(c *gin.Context, req interface{}) error {
    if err := c.ShouldBind(req); err != nil {
        return err
    }
    return validate.Struct(req)
}

func ParseVideoUploadRequest(c *gin.Context) (*VideoUploadRequest, error) {
    var req VideoUploadRequest

    if err := c.ShouldBind(&req); err != nil {
        return nil, err
    }

    // Parse output types
    outputTypesStr := c.PostForm("output_types")
    if outputTypesStr != "" {
        req.OutputTypes = strings.Split(outputTypesStr, ",")
    }

    // Parse save iterations
    saveIterationsStr := c.PostForm("save_iterations")
    if saveIterationsStr != "" {
        saveIterationsSlice := strings.Split(saveIterationsStr, ",")
        req.SaveIterations = make([]int, len(saveIterationsSlice))
        for i, s := range saveIterationsSlice {
            val, err := strconv.Atoi(strings.TrimSpace(s))
            if err != nil {
                return nil, errors.New("invalid save iterations")
            }
            req.SaveIterations[i] = val
        }
    }

    // Validate the request
    if err := validate.Struct(req); err != nil {
        return nil, err
    }

    return &req, nil
}