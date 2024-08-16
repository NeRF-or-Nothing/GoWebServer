package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/queue"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/scene"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/user"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/services"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/utils"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/web"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load("secrets/.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Parse command line arguments
	args := utils.ParseArguments()

	// Load IP configuration
	ipFile, err := os.Open(args.ConfigIP)
	if err != nil {
		log.Fatal("Error opening IP configuration file:", err)
	}
	defer ipFile.Close()

	var ipData map[string]string
	if err := json.NewDecoder(ipFile).Decode(&ipData); err != nil {
		log.Fatal("Error decoding IP configuration:", err)
	}

	rabbitMQIP := os.Getenv("RABBITMQ_IP")
	webserverIP := ipData["domain"]
	mongoIP := ipData["mongo"]

	// Create a MongoDB client
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal("Error creating MongoDB client:", err)
	}

	// Connect to the MongoDB server
	err = client.Connect(context.Background())
	if err != nil {
		log.Fatal("Error connecting to MongoDB server:", err)
	}

	// Create separate managers with the MongoDB client
	sceneManager := scene.NewSceneManager(client, false)
	queueManager := queue.NewQueueListManager(client, false)
	userManager := user.NewUserManager(client, false)

	// Initialize services
	mqService, err := services.NewAMPQService(rabbitMQIP, queueManager, sceneManager)
	if err != nil {
		log.Panic("Error initializing AMPQ service:", err)
	}
	clientService := services.NewClientService(sceneManager, mqService, userManager)

	// Initialize web server
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	server := web.NewWebServer(clientService, queueManager, jwtSecret)

	// Start the web server
	if err := server.Run(webserverIP, 5000); err != nil {
		log.Fatal("Error starting web server:", err)
	}
}
