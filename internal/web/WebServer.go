package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/common"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/log"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/queue"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/services"
)

type WebServer struct {
	jwtSecret     string
	app           *fiber.App
	clientService *services.ClientService
	queueManager  *queue.QueueListManager
	logger        *log.Logger
}

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

func (s *WebServer) Run(ip string, port int) error {
	s.SetupRoutes()
	return s.app.Listen(ip + ":" + strconv.Itoa(port))
}

func (s *WebServer) SetupRoutes() {
	s.app.Post("/login", s.loginUser)
	s.app.Post("/register", s.registerUser)
	s.app.Post("/video", s.tokenRequired(s.receiveVideo))
	s.app.Get("/routes", s.getRoutes)
	s.app.Get("/health", s.healthCheck)
}

func (s *WebServer) tokenRequired(handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenString := c.Get("Authorization")
		if tokenString == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Missing Authorization header"})
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(s.jwtSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
		}
		userID, ok := claims["sub"].(string)
		if !ok {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid user ID in token"})
		}

		c.Locals("userID", userID)
		return handler(c)
	}
}

func (s *WebServer) loginUser(c *fiber.Ctx) error {
	fmt.Println("Login request received")

	var req common.LoginRequest
    if err := ValidateRequest(c, &req); err != nil {
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }
	fmt.Println("Login request validated")

	userID, err := s.clientService.LoginUser(context.TODO(), req.Username, req.Password)
	if err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}
	fmt.Println("User logged in")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
	})
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		fmt.Println("Failed to generate token")
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}
	fmt.Printf("JWT token generated, userID %s\n", userID)

	return c.Status(http.StatusOK).JSON(fiber.Map{"jwtToken": tokenString})
}

func (s *WebServer) registerUser(c *fiber.Ctx) error {
	var req common.RegisterRequest
    if err := ValidateRequest(c, &req); err != nil {
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }

	err := s.clientService.RegisterUser(context.TODO(), req.Username, req.Password)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{"message": "User created"})
}

func (s *WebServer) receiveVideo(c *fiber.Ctx) error {
    userID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
    if err != nil {
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
    }

    req, err := ParseVideoUploadRequest(c)
    if err != nil {
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }

    scene_id, err := s.clientService.HandleIncomingVideo(context.TODO(), userID, req)
    if err != nil {
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }

    return c.Status(fiber.StatusOK).SendString(fmt.Sprintf("Video received and processing scene %s. Check back later for updates.", scene_id))
}

func (s *WebServer) getRoutes(c *fiber.Ctx) error {
	routes := s.app.GetRoutes()
	return c.Status(http.StatusOK).JSON(routes)
}

func (s *WebServer) healthCheck(c *fiber.Ctx) error {
	return c.SendString("OK")
}