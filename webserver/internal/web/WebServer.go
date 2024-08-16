package web

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/dbschema"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

type WebServer struct {
    router        *gin.Engine
    clientService *services.ClientService
    queueManager  *dbschema.QueueListManager
    jwtSecret     string
}

func NewWebServer(clientService *services.ClientService, queueManager *dbschema.QueueListManager, jwtSecret string) *WebServer {
    router := gin.Default()
    return &WebServer{
        router:        router,
        clientService: clientService,
        queueManager:  queueManager,
        jwtSecret:     jwtSecret,
    }
}

func (s *WebServer) Run(port int) error {
    return s.router.Run(":" + strconv.Itoa(port))
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
    s.router.GET("/data/metadata/:output_type/:scene_id", s.tokenRequired(s.getNerfTypeMetadata))
    s.router.GET("/data/nerf/:output_type/:scene_id", s.tokenRequired(s.getNerfResource))
    s.router.GET("/preview/:scene_id", s.tokenRequired(s.getPreview))
    s.router.GET("/history", s.tokenRequired(s.getUserHistory))
}

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

func (s *WebServer) loginUser(c *gin.Context) {
    var req LoginRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID, err := s.clientService.LoginUser(req.Username, req.Password)
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

func (s *WebServer) registerUser(c *gin.Context) {
    var req RegisterRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    response := s.clientService.RegisterUser(req.Username, req.Password)
    c.JSON(response.StatusCode, response)
}

func (s *WebServer) getNerfMetadata(c *gin.Context) {
    var req GetNerfMetadataRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID := c.GetString("userID")
    response := s.clientService.GetNerfMetadata(userID, req.SceneID)
    c.JSON(response.StatusCode, response)
}

func (s *WebServer) getNerfTypeMetadata(c *gin.Context) {
    var req GetNerfTypeMetadataRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID := c.GetString("userID")
    response := s.clientService.GetNerfTypeMetadata(userID, req.SceneID, req.OutputType)
    c.JSON(response.StatusCode, response)
}

func (s *WebServer) getNerfResource(c *gin.Context) {
    var req GetNerfResourceRequest
    if err := ValidateRequest(c, &req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID := c.GetString("userID")
    rangeHeader := c.GetHeader("Range")

    response := s.clientService.GetNerfResource(userID, req.SceneID, req.OutputType, req.Iteration, rangeHeader)
    c.DataFromReader(response.StatusCode, response.ContentLength, response.ContentType, response.Body, nil)
}

func (s *WebServer) getUserHistory(c *gin.Context) {
    userID := c.GetString("userID")

    response := s.clientService.GetUserHistory(userID)
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
    response := s.clientService.GetPreview(userID, req.SceneID)
    c.JSON(response.StatusCode, response)
}

func (s *WebServer) receiveVideo(c *gin.Context) {
    userID := c.GetString("userID")

    req, err := ParseVideoUploadRequest(c)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    scene_id, err := s.clientService.HandleIncomingVideo(userID, req.File, req, req.SceneName)
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