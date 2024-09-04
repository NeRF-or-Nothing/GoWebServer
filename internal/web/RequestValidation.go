// This file contains the actual validator implementation for incoming http requests.
//
// You can implement custom validators for each field in this file and reference them in the request structs.

package web

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"github.com/NeRF-or-Nothing/go-web-server/internal/models/scene"
)

var validate *validator.Validate

// Initialize the custom validator
func init() {
    validate = validator.New()
    validate.RegisterValidation("validOutputType", validateOutputType)
}

// ValidateRequest validates a request using a Fiber context and a request struct.
// It parses the request differently based on HTTP method.
func ValidateRequest(c *fiber.Ctx, req interface{}) error {
    // Check the HTTP method
    method := c.Method()

    switch method {
    case "GET":
        // For GET requests, we only need to parse query and path parameters
        if err := c.QueryParser(req); err != nil {
            return err
        }
        if err := c.ParamsParser(req); err != nil {
            return err
        }
    case "POST", "PUT", "PATCH":
        // For requests with potential body content
        if err := c.BodyParser(req); err != nil {
            return err
        }
        // Also parse query parameters for these methods if needed
        if err := c.QueryParser(req); err != nil {
            return err
        }
    default:
        // Unsupported HTTP method
    }

    return validate.Struct(req)
}

// ParseNewSceneRequest is a custom validator that parses a video upload request from a Fiber context.
//
// The default go-validator is not great with file uploads, so we need to handle the file upload here, and just 
// redundantly validate the other form fields.
//
// Returns a NewSceneRequest struct if successful, error otherwise.
func ParseNewSceneRequest(c *fiber.Ctx) (*NewSceneRequest, error) {
    var req NewSceneRequest

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

// ValidateOutputType is a custom validator for output types in a VideoUploadRequest.
func validateOutputType(fl validator.FieldLevel) bool {
    outputType := fl.Field().String()
    trainingMode := fl.Parent().FieldByName("TrainingMode").String()
    return scene.Nerf{}.IsValidOutputType(trainingMode, outputType)
}