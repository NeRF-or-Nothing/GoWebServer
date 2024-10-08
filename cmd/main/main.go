package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/NeRF-or-Nothing/go-web-server/internal/log"
	"github.com/NeRF-or-Nothing/go-web-server/internal/models/queue"
	"github.com/NeRF-or-Nothing/go-web-server/internal/models/scene"
	"github.com/NeRF-or-Nothing/go-web-server/internal/models/user"
	"github.com/NeRF-or-Nothing/go-web-server/internal/services"
	"github.com/NeRF-or-Nothing/go-web-server/internal/web"
)

func main() {
	// Load environment variables from .env file (should be redundant, as docker-compose should load these)
	err := godotenv.Load("secrets/.env")
	if err != nil {
		panic(fmt.Sprintf("Error loading .env file: %s", err))
	}

	// Create webserver logger
	logger, err := log.NewLogger(true, true)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	rabbitMQIP := os.Getenv("RABBITMQ_IP")
	webserverIP := os.Getenv("WEBSERVER_IP")

	// Create a MongoDB client
	mongoURI := fmt.Sprintf("mongodb://%s:%s@%s:27017",
        os.Getenv("MONGO_INITDB_ROOT_USERNAME"),
        os.Getenv("MONGO_INITDB_ROOT_PASSWORD"),
        os.Getenv("MONGO_IP"))
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		logger.Fatal("Error creating MongoDB client:", err)
	}

	// Create separate managers with the MongoDB client
	sceneManager := scene.NewSceneManager(client, logger, false)
	queueManager := queue.NewQueueListManager(client, logger, false)
	userManager := user.NewUserManager(client, logger, false)

	// Initialize services
	mqService, err := services.NewAMPQService(rabbitMQIP, sceneManager, queueManager, logger)
	if err != nil {
		logger.Panic("Error initializing AMPQ service:", err)
	}
	clientService := services.NewClientService(mqService, sceneManager, userManager, queueManager, logger)

	// Initialize web server
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	server := web.NewWebServer(jwtSecret, clientService, logger)

	fmt.Println("Starting server...")

	// Start the web server
	if err := server.Run(webserverIP, 5000); err != nil {
		logger.Fatal("Error starting web server:", err)
	}
}
