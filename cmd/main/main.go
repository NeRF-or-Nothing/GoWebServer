package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/log"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/queue"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/scene"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/user"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/services"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/web"
)

func exploreDirectory(root string) error {
    return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            fmt.Printf("Error accessing path %q: %v\n", path, err)
            return err
        }

        // Get the relative path
        relPath, err := filepath.Rel(root, path)
        if err != nil {
            fmt.Printf("Error getting relative path for %q: %v\n", path, err)
            return err
        }

        if info.IsDir() {
            fmt.Printf("Directory: %s\n", relPath)
        } else {
            fmt.Printf("File: %s (Size: %d bytes)\n", relPath, info.Size())
        }

        return nil
    })
}


func main() {
	// Load environment variables from .env file
	err := godotenv.Load("secrets/.env")
	if err != nil {
		panic(fmt.Sprintf("Error loading .env file: %s", err))
	}

	// Create webserver logger
	logger, err := log.NewLogger(true)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Get the current working directory
    currentDir, err := os.Getwd()
    if err != nil {
        fmt.Printf("Error getting current directory: %v\n", err)
        return
    }

    fmt.Printf("Exploring directory: %s\n", currentDir)
    err = exploreDirectory(currentDir)
    if err != nil {
        fmt.Printf("Error exploring directory: %v\n", err)
    }


	// // Load IP configuration
	// ipFile, err := os.Open(os.Getenv("DOCKER_IN_PATH"))
	// if err != nil {
	// 	logger.Fatal("Error opening IP configuration file:", err)
	// }
	// defer ipFile.Close()

	// var ipData map[string]string
	// if err := json.NewDecoder(ipFile).Decode(&ipData); err != nil {
	// 	logger.Fatal("Error decoding IP configuration:", err)
	// }

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
	mqService, err := services.NewAMPQService(rabbitMQIP, queueManager, sceneManager, logger)
	if err != nil {
		logger.Panic("Error initializing AMPQ service:", err)
	}
	clientService := services.NewClientService(sceneManager, mqService, userManager, logger)

	// Initialize web server
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	server := web.NewWebServer(jwtSecret, clientService, queueManager, logger)

	fmt.Println("Starting server...")

	// Start the web server
	if err := server.Run(webserverIP, 5000); err != nil {
		logger.Fatal("Error starting web server:", err)
	}
}
