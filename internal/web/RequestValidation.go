// RequestValidation.go

package web

import (
    "errors"
    "strconv"
    "strings"

    "github.com/go-playground/validator/v10"
    "github.com/gofiber/fiber/v2"
    
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

func ValidateRequest(c *fiber.Ctx, req interface{}) error {
    // For JSON payloads
    if err := c.BodyParser(req); err != nil {
        return err
    }
    
    // For query parameters
    if err := c.QueryParser(req); err != nil {
        return err
    }
    
    // For path parameters
    if err := c.ParamsParser(req); err != nil {
        return err
    }

    return validate.Struct(req)
}

func ParseVideoUploadRequest(c *fiber.Ctx) (*common.VideoUploadRequest, error) {
    var req common.VideoUploadRequest

    // Handle file upload
    file, err := c.FormFile("file")
    if err != nil {
        return nil, errors.New("file upload error: " + err.Error())
    }
    req.File = file

    // Parse other form fields
    req.TrainingMode = c.FormValue("training_mode")
    req.SceneName = c.FormValue("scene_name")

    // Parse total iterations
    totalIterationsStr := c.FormValue("total_iterations")
    if totalIterationsStr != "" {
        totalIterations, err := strconv.Atoi(totalIterationsStr)
        if err != nil {
            return nil, errors.New("invalid total iterations")
        }
        req.TotalIterations = totalIterations
    }

    // Parse output types
    outputTypesStr := c.FormValue("output_types")
    if outputTypesStr != "" {
        req.OutputTypes = strings.Split(outputTypesStr, ",")
    }

    // Parse save iterations
    saveIterationsStr := c.FormValue("save_iterations")
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