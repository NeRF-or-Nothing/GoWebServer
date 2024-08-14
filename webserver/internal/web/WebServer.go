// // Package: webserver provides a (http only) web server that handles incoming requests and routes them to the
// // appropriate services. The web server is built using the Gin framework and provides a RESTful API
// // for interacting with the system. The web server is responsible for handling incoming video files,

package web

// import (
//     "net/http"
//     "os"
//     "path/filepath"
//     "strconv"
//
//
//
//
//

//     "github.com/gin-gonic/gin"
//     "github.com/golang-jwt/jwt"
//     "github.com/google/uuid"
//     "github.com/your-project/models"
//     "github.com/your-project/services"
// )

// type WebServer struct {
//     router        *gin.Engine
//     clientService *services.ClientService
//     queueManager  *services.QueueListManager
//     jwtSecret     string
// }

// func NewWebServer(clientService *services.ClientService, queueManager *services.QueueListManager, jwtSecret string) *WebServer {
//     router := gin.Default()
//     return &WebServer{
//         router:        router,
//         clientService: clientService,
//         queueManager:  queueManager,
//         jwtSecret:     jwtSecret,
//     }
// }

// func (s *WebServer) Run(port int) error {
//     return s.router.Run(":" + strconv.Itoa(port))
// }

// func (s *WebServer) SetupRoutes() {
//     s.router.POST("/video", s.tokenRequired(s.receiveVideo))
//     s.router.POST("/login", s.loginUser)
//     s.router.POST("/register", s.registerUser)
//     s.router.GET("/routes", s.sendRoutes)
//     s.router.GET("/data/metadata/:uuid", s.tokenRequired(s.sendNerfMetadata))
//     s.router.GET("/data/metadata/:output_type/:uuid", s.tokenRequired(s.sendNerfTypeMetadata))
//     s.router.GET("/data/nerf/:output_type/:uuid", s.tokenRequired(s.sendNerfResource))
//     s.router.GET("/worker-data/*path", s.sendToWorker)
//     s.router.GET("/queue", s.sendQueuePosition)
//     s.router.GET("/history", s.tokenRequired(s.sendUserHistory))
//     s.router.GET("/preview/:uuid", s.tokenRequired(s.sendPreview))
//     s.router.GET("/health", s.healthCheck)
// }

// func (s *WebServer) tokenRequired(handler gin.HandlerFunc) gin.HandlerFunc {
//     return func(c *gin.Context) {
//         tokenString := c.GetHeader("Authorization")
//         if tokenString == "" {
//             c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
//             return
//         }

//         token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
//             return []byte(s.jwtSecret), nil
//         })

//         if err != nil || !token.Valid {
//             c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
//             return
//         }

//         claims, ok := token.Claims.(jwt.MapClaims)
//         if !ok {
//             c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
//             return
//         }
//         userID, ok := claims["sub"].(string)
//         if !ok {
//             c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
//             return
//         }

//         c.Set("userID", userID)
//         handler(c)
//     }
// }

// func (s *WebServer) sendNerfMetadata(c *gin.Context) {
//     // userID := c.GetString("userID")
//     // uuid := c.Param("uuid")

//     // response := s.clientService.GetNerfMetadata(userID, uuid)
//     // c.JSON(response.StatusCode, response)
// }

// func (s *WebServer) sendNerfTypeMetadata(c *gin.Context) {
//     // userID := c.GetString("userID")
//     // outputType := c.Param("output_type")
//     // uuid := c.Param("uuid")

//     // response := s.clientService.GetNerfTypeMetadata(userID, uuid, outputType)
//     // c.JSON(response.StatusCode, response)
// }

// func (s *WebServer) sendNerfResource(c *gin.Context) {
//     // userID := c.GetString("userID")
//     // outputType := c.Param("output_type")
//     // uuid := c.Param("uuid")
//     // iteration := c.Query("iteration")
//     // rangeHeader := c.GetHeader("Range")

//     // response := s.clientService.SendNerfResource(userID, uuid, outputType, iteration, rangeHeader)
//     // c.DataFromReader(response.StatusCode, response.ContentLength, response.ContentType, response.Body, nil)
// }

// func (s *WebServer) sendUserHistory(c *gin.Context) {
//     // userID := c.GetString("userID")

//     // response := s.clientService.GetUserHistory(userID)
//     // c.JSON(response.StatusCode, response)
// }

// func (s *WebServer) sendToWorker(c *gin.Context) {
//     // path := c.Param("path")

//     // if _, err := os.Stat(path); os.IsNotExist(err) {
//     //     c.String(http.StatusNotFound, "File not found")
//     //     return
//     // }

//     // c.File(path)
// }

// func (s *WebServer) sendPreview(c *gin.Context) {
//     // userID := c.GetString("userID")
//     // uuid := c.Param("uuid")

//     // response := s.clientService.GetPreview(userID, uuid)
//     // c.JSON(response.StatusCode, response)
// }

// func (s *WebServer) receiveVideo(c *gin.Context) {
// //     userID := c.GetString("userID")

// //     file, err := c.FormFile("file")
// //     if err != nil {
// //         c.JSON(http.StatusBadRequest, gin.H{"error": "No file part in the request"})
// //         return
// //     }

// //     config, err := validateRequestParams(c)
// //     if err != nil {
// //         c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// //         return
// //     }

// //     sceneName := c.PostForm("scene_name")
// //     uuid, err := s.clientService.HandleIncomingVideo(userID, file, config, sceneName)
// //     if err != nil {
// //         c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
// //         return
// //     }

// //     response := models.Response{
// //         Status:  models.Processing,
// //         Error:   models.NoError,
// //         Message: "Video received and processing. Check back later for updates.",
// //         UUID:    uuid,
// //         Data:    config,
// //     }
// //     c.JSON(http.StatusOK, response)
// }

// func (s *WebServer) loginUser(c *gin.Context) {
//     // username := c.PostForm("username")
//     // password := c.PostForm("password")

//     // if username == "" || password == "" {
//     //     response := models.Response{
//     //         Status:  models.Error,
//     //         Error:   models.InvalidCredentials,
//     //         Message: "Username or password not provided",
//     //     }
//     //     c.JSON(http.StatusBadRequest, response)
//     //     return
//     // }

//     // response := s.clientService.LoginUser(username, password)
//     // c.JSON(response.StatusCode, response)
// }

// func (s *WebServer) registerUser(c *gin.Context) {
//     // username := c.PostForm("username")
//     // password := c.PostForm("password")

//     // response := s.clientService.RegisterUser(username, password)
//     // c.JSON(response.StatusCode, response)
// }

// func (s *WebServer) sendQueuePosition(c *gin.Context) {
//     // queueID := c.Query("queueid")
//     // taskID := c.Query("id")

//     // position := s.queueManager.GetQueuePosition(queueID, taskID)
//     // size := s.queueManager.GetQueueSize(queueID)

//     // c.String(http.StatusOK, "%d / %d", position, size)
// }

// func (s *WebServer) sendRoutes(c *gin.Context) {
//     // routes := make([]gin.RouteInfo, 0)
//     // for _, route := range s.router.Routes() {
//     //     routes = append(routes, route)
//     // }
//     // c.JSON(http.StatusOK, routes)
// }

// func (s *WebServer) healthCheck(c *gin.Context) {
//     // c.String(http.StatusOK, "OK")
// }

// func isValidUUID(value string) bool {
//     // _, err := uuid.Parse(value)
//     // return err == nil
// }
