// RequestValidation.go

package web

import (
    "errors"
    "strconv"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/go-playground/validator/v10"

    
    "github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/scene"
    "github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/common"
)

var validate *validator.Validate

func init() {
    validate = validator.New()
    validate.RegisterValidation("validOutputType", validateOutputType)
}

func validateOutputType(fl validator.FieldLevel) bool {
    outputType := fl.Field().String()
    trainingMode := fl.Parent().FieldByName("TrainingMode").String()
    return scene.Nerf{}.IsValidOutputType(trainingMode, outputType)
}


func ValidateRequest(c *gin.Context, req interface{}) error {
    if err := c.ShouldBind(req); err != nil {
        return err
    }
    return validate.Struct(req)
}

func ParseVideoUploadRequest(c *gin.Context) (*common.VideoUploadRequest, error) {
    var req common.VideoUploadRequest

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