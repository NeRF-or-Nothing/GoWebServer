// This file contains the WebServer implementation. The WebServer is responsible for handling/validating HTTP requests and
// dispatching them to the appropriate handler.
//
// Access to the database should be through the ClientService.
// The only direct access to filesystem should be for reading files when sending data between
// workers, thumbnail retrieval, and output retrieval.

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/NeRF-or-Nothing/go-web-server/internal/services"
	"github.com/NeRF-or-Nothing/go-web-server/internal/log"
)

type WebServer struct {
	jwtSecret     string
	app           *fiber.App
	clientService *services.ClientService
	logger        *log.Logger
}

// NewWebServer creates a new WebServer instance.
func NewWebServer(jwtSecret string, clientService *services.ClientService, logger *log.Logger) *WebServer {
	logger.Debug("Creating new web server instance")

	app := fiber.New(fiber.Config{
		BodyLimit: 16 * 1024 * 1024, // Max Single Request Body Size: 16MB
		StreamRequestBody: true,     // Stream request body to disk
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Authorization, Content-Type",
	}))

	return &WebServer{
		jwtSecret:     jwtSecret,
		app:           app,
		clientService: clientService,
		logger:        logger,
	}
}

// Run starts the web server on the given IP and port.
func (s *WebServer) Run(ip string, port int) error {
	s.SetupRoutes()
	s.SetupFileStructure()
	return s.app.Listen(ip + ":" + strconv.Itoa(port))
}

// SetupRoutes sets up the routes for the web server.
func (s *WebServer) SetupRoutes() {
	// External Account Routes
	s.app.Post("/user/account/login", s.loginUser)
	s.app.Post("/user/account/register", s.registerUser)
	s.app.Patch("/user/account/update/username", s.tokenRequired(s.updateUserUsername))
	s.app.Patch("/user/account/update/password", s.tokenRequired(s.updateUserPassword))
	s.app.Delete("/user/account/delete", s.tokenRequired(s.deleteUser))

	// External Scene Routes
	s.app.Delete("/user/scene/delete/:scene_id", s.tokenRequired(s.deleteUserScene))
	s.app.Post("/user/scene/new", s.tokenRequired(s.postNewScene))
	s.app.Get("/user/scene/metadata/:scene_id", s.tokenRequired(s.getSceneMetadata))
	s.app.Get("/user/scene/thumbnail/:scene_id", s.tokenRequired(s.getSceneThumbnail))
	s.app.Get("/user/scene/name/:scene_id", s.tokenRequired(s.getSceneName))
	s.app.Get("/user/scene/progress/:scene_id", s.tokenRequired(s.getSceneProgress))
	s.app.Get("/user/scene/history", s.tokenRequired(s.getUserSceneHistory))
	s.app.Get("/user/scene/output/:output_type/:scene_id", s.tokenRequired(s.getSceneOutput))

	// Internal routes
	s.app.Get("/worker-data/*", s.getWorkerData)

	// Debug routes
	s.app.Get("/routes", s.getRoutes)
	s.app.Get("/health", s.healthCheck)
}

// SetupFileStructure creates the necessary directories for storing data files.
// Due to docker volume mapping, this should be mostly redundant, but it is included for completeness.
func (s *WebServer) SetupFileStructure() {
	dataDir := "/data"
	sfmDir := filepath.Join(dataDir, "sfm")
	nerfDir := filepath.Join(dataDir, "nerf")
	rawDir := filepath.Join(dataDir, "raw")

	err := os.MkdirAll(sfmDir, os.ModePerm)
	if err != nil {
		s.logger.Info("Failed to create sfm directory:", err.Error())
	}

	err = os.MkdirAll(nerfDir, os.ModePerm)
	if err != nil {
		s.logger.Info("Failed to create nerf directory:", err.Error())
	}

	err = os.MkdirAll(rawDir, os.ModePerm)
	if err != nil {
		s.logger.Info("Failed to create raw directory:", err.Error())
	}
}

// tokenRequired is a middleware that checks for a valid JWT token in the Authorization header.
//
// The token is expected to be in the format: `Bearer <token>`.   
// A valid token will decode to a user ID (of type String(primitive.ObjectID)).
// It is expected that the user ID is stored in the token's `sub` claim. 
//
// Validation of the user's existence is not performed here.
// and instead the user ID is stored in the fiber context for use in request handlers,
// which is then validated by ClientService.
func (s *WebServer) tokenRequired(handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			s.logger.Debug("Missing Authorization header")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Missing Authorization header"})
		}

		s.logger.Debugf("\nAuthorization header: %s", authHeader)
		parts := strings.Split(authHeader, " ")

		if len(parts) != 2 || parts[0] != "Bearer" {
			s.logger.Debug("Invalid Authorization header format. Expected: `Bearer <token>`")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid Authorization header format. Expected: `Bearer <token>`"})
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(s.jwtSecret), nil
		})

		if err != nil || !token.Valid {
			s.logger.Debug("Invalid token")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			s.logger.Debug("Invalid token claims")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
		}
		userID, ok := claims["sub"].(string)
		if !ok {
			s.logger.Debug("Invalid user ID in token")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user ID in token"})
		}

		c.Locals("userID", userID)
		return handler(c)
	}
}

// loginUser handles the login request.
//
// It expects a JSON payload with the following format:
//	{
//	    "username": "username",
//	    "password": "password"
//	}
func (s *WebServer) loginUser(c *fiber.Ctx) error {
	s.logger.Debug("Login request received")

	var req LoginRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Login request validation failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	s.logger.Debug("Login request validated")

	userID, err := s.clientService.LoginUser(context.TODO(), req.Username, req.Password)
	if err != nil {
		s.logger.Debug("User login failed: ", err.Error())
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	s.logger.Debug("User logged in")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
	})
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		s.logger.Debug("Failed to generate token")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}
	s.logger.Debugf("JWT token generated, userID %s\n", userID)

	return c.Status(http.StatusOK).JSON(fiber.Map{"jwtToken": tokenString})
}

// registerUser handles the registration request. 
// 
// It expects a JSON payload with the following format:
//	{
//	    "username": "username",
//	    "password": "password"
//	}
func (s *WebServer) registerUser(c *fiber.Ctx) error {
	s.logger.Debug("Register request received")

	var req RegisterRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Register request validation failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "success": false})
	}

	err := s.clientService.RegisterUser(context.TODO(), req.Username, req.Password)
	if err != nil {
		s.logger.Debug("User registration failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error(), "success": false})
	}

	s.logger.Debug("User registered successfully")
	return c.Status(http.StatusCreated).JSON(fiber.Map{"success": true})
}

// updateUserUsername handles the request to update the username of a user. It is a JWT protected route.
//
// It expects a JSON payload with the following format:
//	{
//	    "password": "password",
//	    "new_username": "new_username"
//	}
func (s *WebServer) updateUserUsername(c *fiber.Ctx) error {
	s.logger.Debug("Update username request received")

	var req UpdateUsernameRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Update username request validation failed: ", err.Error())
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return fiber.NewError(http.StatusBadRequest, "Invalid user ID")
	}

	err = s.clientService.UpdateUserUsername(context.TODO(), userID, req.Password, req.NewUsername)
	if err != nil {
		s.logger.Debug("Failed to update username: ", err.Error())
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "Username updated"})
}

// updateUserPassword handles the request to update the password of a user. It is a JWT protected route.
// It expects a JSON payload with the following format:
//
//	{
//	    "old_password": "old_password",
//	    "new_password": "new_password"
//	}
func (s *WebServer) updateUserPassword(c *fiber.Ctx) error {
	s.logger.Debug("Update password request received")

	var req UpdatePasswordRequest
	if err := ValidateRequest(c, &req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return fiber.NewError(http.StatusBadRequest, "Invalid user ID")
	}

	err = s.clientService.UpdateUserPassword(context.TODO(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		s.logger.Debug("Failed to update password: ", err.Error())
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "Password updated"})
}

// Must be careful in implementing these two functions.
// Figure our how to gracefully handle deletion of scenes since they might be processing.
//
// Planning: Confirmation at the client level. In progress scenes are denied deletion
// (This should be redundant ui-wise since the client doesnt display )
// If a completed scene is deleted, we do not need to delete the output files. FOR NOW. Instead just 
// remove from DB.
//
// This allows us to implement a goroutine that periodically cleans up old output files. (we can just append to a list of files to delete)
func (s *WebServer) deleteUserScene(c *fiber.Ctx) error {
	return fiber.NewError(http.StatusNotImplemented, "Not implemented")
}

// Should deleting a user also delete all their scenes? How to handle this?
// Do we need to expand auth for confirmation, or leave that at the client level?
func (s *WebServer) deleteUser(c *fiber.Ctx) error {
	return fiber.NewError(http.StatusNotImplemented, "Not implemented")
}

// postNewScene handles the new scene request. It is a JWT protected route.
//
// It expects a multipart form with the following fields:
//   - file: required,
//     the video file to upload
//   - training_mode: optional,
//     the training mode to use (gaussian or tensorf)
//   - output_types: optional,
//     a comma-separated list of output types to save (e.g. splat_cloud, point_cloud, etc.)
//   - save_iterations: optional,
//     a comma-separated list of iterations to save the output at (0 <= x <= 30000)
//   - total_iterations: optional,
//     the total number of iterations to run (0 <= x <= 30000)
//   - scene_name: optional,
//     the name of the scene
func (s *WebServer) postNewScene(c *fiber.Ctx) error {
	s.logger.Debug("New Scene Request received")
	var req *NewSceneRequest
	var err error

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	req, err = ParseNewSceneRequest(c)
	if err != nil {
		s.logger.Debug("Video upload request parsing failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if req.TrainingMode == "tensorf" {
		s.logger.Debug("Tensorf training mode is now deprecated. Please use gaussian training mode.")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Tensorf training mode is now deprecated. Please use gaussian training mode."})
	}

	sceneID, err := s.clientService.HandleIncomingVideo(
		context.TODO(),
		userID,
		req.File,
		req.TrainingMode,
		req.OutputTypes,
		req.SaveIterations,
		req.TotalIterations,
		req.SceneName,
	)
	if err != nil {
		s.logger.Debug("Video processing failed:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Debugf("Video received and processing scene %s. Check back later for updates.\n", sceneID)
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"id": sceneID, "message": "Video received and processing scene. Check back later for updates."})
}

// getSceneMetadata handles the request to get the metadata for a scene. It is a JWT protected route.
//
// It expects path parameter `scene_id`.
func (s *WebServer) getSceneMetadata(c *fiber.Ctx) error {
	s.logger.Debug("Get scene metadata request received")

	var req GetSceneMetadataRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Get job data request validation failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
	if err != nil {
		s.logger.Debug("Invalid job ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
	}

	sceneData, err := s.clientService.GetSceneMetadata(context.TODO(), userID, sceneID)
	if err != nil {
		s.logger.Debug("Failed to get job data: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	sceneJson, err := json.Marshal(sceneData)
	if err != nil {
		s.logger.Debug("Failed to marshal job data: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Debug(fmt.Sprintf("Job data retrieved successfully, data: %s", sceneJson))
	return c.Status(http.StatusOK).Send(sceneJson)
}

// getUserSceneHistory handles the request to get the history of scenes for a user. It is a JWT protected route.
func (s *WebServer) getUserSceneHistory(c *fiber.Ctx) error {
	s.logger.Debug("Get user history request received")

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	sceneIDList, err := s.clientService.GetUserSceneHistory(context.TODO(), userID)
	if err != nil {
		s.logger.Debug("Failed to get user history: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Debug("User history retrieved successfully")
	return c.Status(http.StatusOK).JSON(fiber.Map{"resources": sceneIDList})
}

// getSceneThumbnail handles the request to get the thumbnail for a scene. It is a JWT protected route.
//
// It expects path parameter `scene_id`
func (s *WebServer) getSceneThumbnail(c *fiber.Ctx) error {
	s.logger.Debug("Get scene thumbnail request received")

	var req GetSceneThumbnailRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Get scene thumbnail request validation failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
	if err != nil {
		s.logger.Debug("Invalid scene ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid scene ID"})
	}

	thumbnailPath, err := s.clientService.GetSceneThumbnailPath(context.TODO(), userID, sceneID)
	if err != nil {
		s.logger.Debug("Failed to get scene thumbnail: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	thumbnailData, err := os.ReadFile(thumbnailPath)
	if err != nil {
		s.logger.Debug("Failed to read thumbnail data: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Debug("Scene thumbnail retrieved successfully")
	return c.Status(http.StatusOK).Send(thumbnailData)
}

// getSceneName handles the request to get the name of a scene. It is a JWT protected route.
//
// It expects path parameter `scene_id`.
func (s *WebServer) getSceneName(c *fiber.Ctx) error {
	s.logger.Debug("Get scene name request received")

	var req GetSceneNameRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Get scene name request validation failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
	if err != nil {
		s.logger.Debug("Invalid scene ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid scene ID"})
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	sceneName, err := s.clientService.GetSceneName(context.TODO(), userID, sceneID)
	if err != nil {
		s.logger.Debug("Failed to get scene name: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"name": sceneName})
}

// getSceneOutput handles the request to get the output for a scene. It is a JWT protected route.
// 
// It expects a path parameters `scene_id` `output_type`.
// 
// The user can optionally specify a query parameter `iteration` to get the output at a specific iteration.
// If the iteration is not specified, the latest output is given.
func (s *WebServer) getSceneOutput(c *fiber.Ctx) error {
	s.logger.Debug("Get scene output request received")

	var req GetSceneOutputRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Get scene output request validation failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
	if err != nil {
		s.logger.Debug("Invalid scene ID: ", req.SceneID)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid scene ID"})
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", c.Locals("userID").(string))
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	outputPath, err := s.clientService.GetSceneOutputPath(context.TODO(), userID, sceneID, req.OutputType, req.Iteration)
	if err != nil {
		s.logger.Debugf("Failed to get scene output: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return s.sendFileWithRangeSupport(c, outputPath)
}

// getSceneProgress handles the request to get the progress of a scene. It is a JWT protected route.
//
// It expects a path parameter `scene_id`.
func (s *WebServer) getSceneProgress(c *fiber.Ctx) error {
	s.logger.Debug("Get scene progress request received")

	var req GetSceneProgressRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Debug("Get scene progress request validation failed: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
	if err != nil {
		s.logger.Debug("Invalid scene ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid scene ID"})
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Debug("Invalid user ID: ", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	progress, err := s.clientService.GetSceneProgress(context.TODO(), userID, sceneID)
	if err != nil {
		s.logger.Debug("Failed to get scene progress: ", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(progress)
}

// getWorkerData handles the request to send data between workers. It is an internal route.
// 
// The path given is trusted and thus a vulnerability.
func (s *WebServer) getWorkerData(c *fiber.Ctx) error {
	s.logger.Debug("Get worker data request received, path:", c.Params("*"))

	fullPath := c.Params("*")

	if fullPath == "" {
		s.logger.Debug("Invalid path parameter")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid path parameter"})
	}

	basePath := "/app"
	fullPath = filepath.Join(basePath, fullPath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		s.logger.Debug("File not found: ", fullPath)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "File Not Found"})
	}

	return c.SendFile(fullPath)
}

// getRoutes handles the request to get the list of routes available on the server.
func (s *WebServer) getRoutes(c *fiber.Ctx) error {
	s.logger.Debug("Get routes request received")
	routes := s.app.GetRoutes()
	return c.Status(http.StatusOK).JSON(routes)
}

// healthCheck handles the request to check the health of the server.
func (s *WebServer) healthCheck(c *fiber.Ctx) error {
	s.logger.Debug("Health check request received")
	return c.SendString("OK")
}


// sendFileWithRangeSupport sends a file with support for the Range header.
// Call this function from any handler which you suspect needs to handle large files.
//
// This function trusts the Range header and does not perform any validation on the range values.
func (s *WebServer) sendFileWithRangeSupport(c *fiber.Ctx, filePath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open file"})
    }
    defer file.Close()

    stat, err := file.Stat()
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get file info"})
    }

    fileSize := stat.Size()
    contentLength := fileSize
    start := int64(0)
    end := fileSize - 1

    rangeHeader := c.Get("Range")
    if rangeHeader != "" {
        if strings.HasPrefix(rangeHeader, "bytes=") {
            rangeHeader = rangeHeader[6:]
            rangeParts := strings.Split(rangeHeader, "-")
            if len(rangeParts) == 2 {
                start, _ = strconv.ParseInt(rangeParts[0], 10, 64)
                if rangeParts[1] != "" {
                    end, _ = strconv.ParseInt(rangeParts[1], 10, 64)
                }
            }
        }
    }

    if start >= fileSize || start > end || end >= fileSize {
        return c.Status(fiber.StatusRequestedRangeNotSatisfiable).SendString("Invalid range")
    }

    contentLength = end - start + 1

    c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
    c.Set("Accept-Ranges", "bytes")
    c.Set("Content-Length", fmt.Sprintf("%d", contentLength))

    // Set the appropriate status code
    if rangeHeader != "" {
        c.Status(fiber.StatusPartialContent)
    } else {
        c.Status(fiber.StatusOK)
    }

    // Set the Content-Type header based on the file extension
    c.Type(filepath.Ext(filePath))

    // Seek to the start position in the file
    _, err = file.Seek(start, io.SeekStart)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to seek file"})
    }

    // Use io.CopyN to send only the requested range of bytes
    _, err = io.CopyN(c, file, contentLength)
    if err != nil && err != io.EOF {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to send file"})
    }

    return nil
}