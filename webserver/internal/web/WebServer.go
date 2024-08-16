package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson/primitive"

	// Internal imports
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/queue"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/scene"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/services"
    "github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/common"
)

type WebServer struct {
    router        *gin.Engine
    clientService *services.ClientService
    queueManager  *queue.QueueListManager
    jwtSecret     string
}

func NewWebServer(clientService *services.ClientService, queueManager *queue.QueueListManager, jwtSecret string) *WebServer {
    router := gin.Default()
    return &WebServer{
        router:        router,
        clientService: clientService,
        queueManager:  queueManager,
        jwtSecret:     jwtSecret,
    }
}

func (s *WebServer) Run(ip string, port int) error {
    return s.router.Run(ip + ":" + strconv.Itoa(port))
}

func (s *WebServer) SetupRoutes() {
    s.router.POST("/login", s.loginUser)
    s.router.POST("/register", s.registerUser)
    s.router.POST("/video", s.tokenRequired(s.receiveVideo))
    s.router.GET("/routes", s.getRoutes)
    s.router.GET("/queue", s.getQueuePosition)
    s.router.GET("/health", s.healthCheck)
    s.router.GET("/worker-data/*path", s.getWorkerData)
    s.router.GET("/data/metadata/:scene_id", s.tokenRequired(s.getNerfMetadata))
    s.router.GET("/data/nerf/:output_type/:scene_id", s.tokenRequired(s.getNerfResource))
    s.router.GET("/preview/:scene_id", s.tokenRequired(s.getPreview))
    s.router.GET("/history", s.tokenRequired(s.getUserHistory))
}

// tokenRequired is a middleware that checks if a valid JWT token is present in the Authorization header.
// If the token is valid, the user ID is extracted from the token and set in the context. 
// If the token is invalid, a 401 Unauthorized response is returned.
func (s *WebServer) tokenRequired(handler gin.HandlerFunc) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString := c.GetHeader("Authorization")
        if tokenString == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
            return
        }

        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(s.jwtSecret), nil
        })

        if err != nil || !token.Valid {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
            return
        }

        claims, ok := token.Claims.(jwt.MapClaims)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
            return
        }
        userID, ok := claims["sub"].(string)
        if !ok {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
            return
        }

        c.Set("userID", userID)
        handler(c)
    }
}

// loginUser handles the login request. It delegates the login operation to the client service,
// and if successful, generates a JWT token containing the user ID and returns it in the response.
func (s *WebServer) loginUser(c *gin.Context) {
    var req common.LoginRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID, err := s.clientService.LoginUser(context.TODO(), req.Username, req.Password)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
        return
    }

    // Generate JWT token contianing user ID
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": userID,
    })
    tokenString, err := token.SignedString([]byte(s.jwtSecret))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"jwtToken": tokenString})
    return
}

// registerUser handles the register request. It delegates the register operation to the client service,
// and returns a 201 Created response if successful.
func (s *WebServer) registerUser(c *gin.Context) {
    var req common.RegisterRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    err := s.clientService.RegisterUser(context.TODO(), req.Username, req.Password)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusCreated, gin.H{"message": "User created"})
}

// getNerfMetadata handles the request to get metadata about a scene. It delegates the operation to the client service,
// and returns the metadata in the response if successful.
func (s *WebServer) getNerfMetadata(c *gin.Context) {
    var req common.GetNerfMetadataRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID, err := primitive.ObjectIDFromHex(c.GetString("userID"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }
    sceneID, err := primitive.ObjectIDFromHex(req.SceneID)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scene ID"})
        return
    }

    metadata, err := s.clientService.GetNerfMetadata( context.TODO(), userID, sceneID, req.OutputType)

    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, metadata)
}

// getNerfResource handles the request to get a resource for a scene. It delegates the operation to the client service,
// and returns the resource in the response if successful. The resource is streamed back to the client, using the provided range header.
func (s *WebServer) getNerfResource(c *gin.Context) {
    var req common.GetNerfResourceRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID := c.GetString("userID")
    rangeHeader := c.GetHeader("Range")

    response := s.clientService.GetNerfResource(context.TODO(), userID, req.SceneID, req.OutputType, req.Iteration, rangeHeader)
    c.DataFromReader(response.StatusCode, response.ContentLength, response.ContentType, response.Body, nil)
}

func (s *WebServer) getUserHistory(c *gin.Context) {
    userID := c.GetString("userID")

    response := s.clientService.GetUserHistory(context.TODO(), userID)
    c.JSON(response.StatusCode, response)
}

func (s *WebServer) getWorkerData(c *gin.Context) {
    path := c.Param("path")

    if _, err := os.Stat(path); os.IsNotExist(err) {
        c.String(http.StatusNotFound, "File not found")
        return
    }

    c.File(path)
}

func (s *WebServer) getPreview(c *gin.Context) {
    var req GetPreviewRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID := c.GetString("userID")
    response := s.clientService.GetPreview(context.TODO(), userID, req.SceneID)
    c.JSON(response.StatusCode, response)
}

func (s *WebServer) receiveVideo(c *gin.Context) {
    userID, err := primitive.ObjectIDFromHex(c.GetString("userID"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return  
    }

    req, err := ParseVideoUploadRequest(c)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }



    scene_id, err := s.clientService.HandleIncomingVideo(context.TODO(), userID, req)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // TODO: Fix
    c.JSON(http.StatusOK, fmt.Sprintf("Video received and processing scene %s. Check back later for updates.", &scene_id))
}

func (s *WebServer) getQueuePosition(c *gin.Context) {
    var req GetQueuePositionRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    position := s.queueManager.getQueuePosition(req.QueueID, req.TaskID)
    size := s.queueManager.getQueueSize(req.QueueID)

    c.String(http.StatusOK, "%d / %d", position, size)
}

func (s *WebServer) getRoutes(c *gin.Context) {
    routes := make([]gin.RouteInfo, 0)
    for _, route := range s.router.Routes() {
        routes = append(routes, route)
    }
    c.JSON(http.StatusOK, routes)
}

func (s *WebServer) healthCheck(c *gin.Context) {
    c.String(http.StatusOK, "OK")
}