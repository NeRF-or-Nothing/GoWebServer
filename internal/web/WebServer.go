// This file contains the WebServer implementation. The WebServer is responsible for handling/validating HTTP requests and
// dispatching them to the appropriate handler.

package web

import (
	"os"
	"fmt"
	"context"
	"strconv"
	"strings"
	"net/http"
	"path/filepath"

	"github.com/golang-jwt/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/log"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/common"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/services"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/queue"
)

type WebServer struct {
	jwtSecret     string
	app           *fiber.App
	clientService *services.ClientService
	queueManager  *queue.QueueListManager
	logger        *log.Logger
}

// NewWebServer creates a new WebServer instance.
func NewWebServer(jwtSecret string, clientService *services.ClientService, queueManager *queue.QueueListManager, logger *log.Logger) *WebServer {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Authorization, Content-Type",
	}))

	return &WebServer{
		jwtSecret:     jwtSecret,
		app:           app,
		clientService: clientService,
		queueManager:  queueManager,
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
	s.app.Post("/user/login", s.loginUser)
	s.app.Post("/user/register", s.registerUser)
	s.app.Patch("/user/account/update/password", s.tokenRequired(s.updateUserPassword))
	s.app.Patch("/user/account/update/username", s.tokenRequired(s.updateUserUsername))
	s.app.Delete("/user/account/delete/scene", s.tokenRequired(s.deleteUserScene))
	s.app.Delete("/user/account/delete/account", s.tokenRequired(s.deleteUser))
	s.app.Post("/user/scene/new", s.tokenRequired(s.receiveVideo))
	s.app.Get("/user/scene/metadata/:scene_id", s.tokenRequired(s.getSceneMetadata))
	s.app.Get("/user/scene/thumbnail/:scene_id", s.tokenRequired(s.getSceneThumbnail))
	s.app.Get("/user/scene/name/:scene_id", s.tokenRequired(s.getSceneName))
	s.app.Get("/user/history", s.tokenRequired(s.getUserSceneHistory))
	s.app.Get("/routes", s.getRoutes)
	s.app.Get("/health", s.healthCheck)
	s.app.Get("/worker-data/*", s.getWorkerData)
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
// The token is expected to be in the format: `Bearer <token>`. A valid token will decode to a user ID (of type String(primitive.ObjectID)).
// It is expected that the user ID is stored in the token's `sub` claim. Validation of the user ID is not performed,
// and instead the user ID is stored in the request context for use in the handler.
func (s *WebServer) tokenRequired(handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			s.logger.Info("Missing Authorization header")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Missing Authorization header"})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			s.logger.Info("Invalid Authorization header format. Expected: `Bearer <token>`")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid Authorization header format. Expected: `Bearer <token>`"})
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(s.jwtSecret), nil
		})

		if err != nil || !token.Valid {
			s.logger.Info("Invalid token")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			s.logger.Info("Invalid token claims")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
		}
		userID, ok := claims["sub"].(string)
		if !ok {
			s.logger.Info("Invalid user ID in token")
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user ID in token"})
		}

		c.Locals("userID", userID)
		return handler(c)
	}
}

// loginUser handles the login request. It expects a JSON payload with the following format:
// {
//     "username": "username",
//     "password": "password"
// }
func (s *WebServer) loginUser(c *fiber.Ctx) error {
	s.logger.Info("Login request received")

	var req common.LoginRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Info("Login request validation failed:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	s.logger.Info("Login request validated")

	userID, err := s.clientService.LoginUser(context.TODO(), req.Username, req.Password)
	if err != nil {
		s.logger.Info("User login failed:", err.Error())
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	s.logger.Info("User logged in")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
	})
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		s.logger.Info("Failed to generate token")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}
	s.logger.Infof("JWT token generated, userID %s\n", userID)

	return c.Status(http.StatusOK).JSON(fiber.Map{"jwtToken": tokenString})
}

// registerUser handles the registration request. It expects a JSON payload with the following format:
// {
//     "username": "username",
//     "password": "password"
// }
func (s *WebServer) registerUser(c *fiber.Ctx) error {
	s.logger.Info("Register request received")

	var req common.RegisterRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Info("Register request validation failed:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := s.clientService.RegisterUser(context.TODO(), req.Username, req.Password)
	if err != nil {
		s.logger.Info("User registration failed:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Info("User registered successfully")
	return c.Status(http.StatusCreated).JSON(fiber.Map{"message": "User created"})
}

// updateUserUsername handles the request to update the username of a user. It is a JWT protected route.
// It expects a JSON payload with the following format:
// {
//     "password": "password",
//     "new_username": "new_username"
// }
func (s *WebServer) updateUserUsername(c* fiber.Ctx) error {
	var req common.UpdateUsernameRequest
	if err := ValidateRequest(c, &req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	
	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, "Invalid user ID")
	}

	err = s.clientService.UpdateUserUsername(context.TODO(), userID, req.Password, req.NewUsername)
	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "Username updated"})
}

// updateUserPassword handles the request to update the password of a user. It is a JWT protected route.
// It expects a JSON payload with the following format:
// {
//     "old_password": "old_password",
//     "new_password": "new_password"
// }
func (s *WebServer) updateUserPassword(c* fiber.Ctx) error {
	var req common.UpdatePasswordRequest
	if err := ValidateRequest(c, &req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, "Invalid user ID")
	}

	err = s.clientService.UpdateUserPassword(context.TODO(), userID, req.OldPassword, req.NewPassword)
	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "Password updated"})
}

// Must be careful in implementing these two functions.
// Figure our how to gracefully handle deletion of scenes since they might be processing.
func (s *WebServer) deleteUserScene(c* fiber.Ctx) error {
	return fiber.NewError(http.StatusNotImplemented, "Not implemented")
}

// Should deleting a user also delete all their scenes? How to handle this?
func (s *WebServer) deleteUser(c* fiber.Ctx) error {
	return fiber.NewError(http.StatusNotImplemented, "Not implemented")
}


// receiveVideo handles the new scene request. It is a JWT protected route.
//It expects a multipart form with the following fields:
//- file: 
//   the video file to upload
// - training_mode: 
//   the training mode to use (gaussian or tensorf)
// - output_types: 
//   a comma-separated list of output types to save (e.g. splat_cloud, point_cloud, etc.)
// - save_iterations: 
//   a comma-separated list of iterations to save the output at (0 <= x <= 30000)
// - total_iterations: 
//   the total number of iterations to run (0 <= x <= 30000)
// - scene_name: 
//   the name of the scene
func (s *WebServer) receiveVideo(c *fiber.Ctx) error {
	s.logger.Info("Video upload request received")

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Info("Invalid user ID:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	req, err := ParseVideoUploadRequest(c)
	if err != nil {
		s.logger.Info("Video upload request parsing failed:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	sceneID, err := s.clientService.HandleIncomingVideo(context.TODO(), userID, req)
	if err != nil {
		s.logger.Info("Video processing failed:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Infof("Video received and processing scene %s. Check back later for updates.\n", sceneID)
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"id": sceneID, "message": "Video received and processing scene. Check back later for updates."})
}


// getSceneMetadata handles the request to get the metadata for a scene. It is a JWT protected route.
// It expects a path parameter `scene_id` with the scene ID.
func (s *WebServer) getSceneMetadata(c *fiber.Ctx) error {
    s.logger.Info("Get job data request received")

    var req common.GetSceneMetadataRequest
    if err := ValidateRequest(c, &req); err != nil {
        s.logger.Info("Get job data request validation failed:", err.Error())
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }

    s.logger.Info("Request data:", req)

    userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
    if err != nil {
        s.logger.Info("Invalid user ID:", err.Error())
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
    }

    sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
    if err != nil {
        s.logger.Info("Invalid job ID:", err.Error())
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid job ID"})
    }

    sceneData, err := s.clientService.GetSceneMetadata(context.TODO(), userID, sceneID)
    if err != nil {
        s.logger.Info("Failed to get job data:", err.Error())
        return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
    }

    s.logger.Info(fmt.Sprintf("Job data retrieved successfully, data: %s", sceneData))
    return c.Status(http.StatusOK).JSON(sceneData)
}

// getUserSceneHistory handles the request to get the history of scenes for a user. It is a JWT protected route.
func (s *WebServer) getUserSceneHistory(c *fiber.Ctx) error {
	s.logger.Info("Get user history request received")

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Info("Invalid user ID:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	sceneIDList, err := s.clientService.GetUserSceneHistory(context.TODO(), userID)
	if err != nil {
		s.logger.Info("Failed to get user history:", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Info("User history retrieved successfully")
	return c.Status(http.StatusOK).JSON(fiber.Map{"resources": sceneIDList})
}

// getSceneThumbnail handles the request to get the thumbnail for a scene. It is a JWT protected route.
// It expects a path parameter `scene_id` with the scene ID.
func (s *WebServer) getSceneThumbnail(c *fiber.Ctx) error {
	s.logger.Info("Get scene thumbnail request received")

	var req common.GetSceneThumbnailRequest
	if err := ValidateRequest(c, &req); err != nil {
		s.logger.Info("Get scene thumbnail request validation failed:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		s.logger.Info("Invalid user ID:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
	if err != nil {
		s.logger.Info("Invalid scene ID:", err.Error())
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid scene ID"})
	}

	thumbnailPath, err := s.clientService.GetSceneThumbnailPath(context.TODO(), userID, sceneID)
	if err != nil {
		s.logger.Info("Failed to get scene thumbnail:", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	thumbnailData, err := os.ReadFile(thumbnailPath)
	if err != nil {
		s.logger.Info("Failed to read thumbnail data:", err.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	s.logger.Info("Scene thumbnail retrieved successfully")
	return c.Status(http.StatusOK).Send(thumbnailData)
}

// getSceneName handles the request to get the name of a scene. It is a JWT protected route.
// It expects a path parameter `scene_id` with the scene ID.
func (s *WebServer) getSceneName(c *fiber.Ctx) error {
	var req common.GetSceneNameRequest
	if err := ValidateRequest(c, &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid scene ID"})
	}

	userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	sceneName, err := s.clientService.GetSceneName(context.TODO(), userID, sceneID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"scene_name": sceneName})
}

// getWorkerData handles the request to send data between workers.
func (s *WebServer) getWorkerData(c *fiber.Ctx) error {
    fmt.Println("Get worker data request received")

    fullPath := c.Params("*")
    fmt.Println("Received path:", fullPath)

    if fullPath == "" {
        fmt.Println("Invalid path parameter")
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid path parameter"})
    }

    basePath := "/app"
    fmt.Println("Base path:", basePath)
    
    fullPath = filepath.Join(basePath, fullPath)
    fmt.Println("Full path:", fullPath)

    fmt.Println("Attempting to send worker data from path:", fullPath)
    fmt.Println("to address:", c.IP())

    if _, err := os.Stat(fullPath); os.IsNotExist(err) {
        fmt.Println("File not found:", fullPath)
        return c.Status(fiber.StatusNotFound).SendString("File not found")
    }

    return c.SendFile(fullPath)
}

// getRoutes handles the request to get the list of routes available on the server.
func (s *WebServer) getRoutes(c *fiber.Ctx) error {
	s.logger.Info("Get routes request received")
	routes := s.app.GetRoutes()
	return c.Status(http.StatusOK).JSON(routes)
}

// healthCheck handles the request to check the health of the server.
func (s *WebServer) healthCheck(c *fiber.Ctx) error {
	s.logger.Info("Health check request received")
	return c.SendString("OK")
}