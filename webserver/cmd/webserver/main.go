package main

import (
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load() // Just to ensure it's used
}

// import (
// 	"encoding/json"
// 	"log"
// 	"os"
//
//

// 	"github.com/joho/godotenv"

// 	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models"
// 	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/services"
// 	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/utils"
// 	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/web"
// )

// func main() {
// 	// Load environment variables from .env file
// 	err := godotenv.Load("secrets/.env")
// 	if err != nil {
// 		log.Fatal("Error loading .env file")
// 	}

// 	// Parse command line arguments
// 	args := utils.ParseArguments()

// 	// Load IP configuration
// 	ipFile, err := os.Open(args.ConfigIP)
// 	if err != nil {
// 		log.Fatal("Error opening IP configuration file:", err)
// 	}
// 	defer ipFile.Close()

// 	var ipData map[string]string
// 	if err := json.NewDecoder(ipFile).Decode(&ipData); err != nil {
// 		log.Fatal("Error decoding IP configuration:", err)
// 	}

// 	rabbitMQIP := os.Getenv("RABBITMQ_IP")
// 	flaskIP := ipData["flaskdomain"]

// 	// Initialize managers and services
// 	sceneManager := models.NewSceneManager()
// 	queueManager := models.NewQueueListManager()
// 	rabbitMQService := services.NewRabbitMQService(rabbitMQIP, queueManager, sceneManager)
// 	userManager := models.NewUserManager()
// 	clientService := services.NewClientService(sceneManager, rabbitMQService, userManager)

// 	// Initialize web server
// 	jwtSecret := os.Getenv("JWT_SECRET_KEY")
// 	server := web.NewWebServer(flaskIP, jwtSecret, args, clientService, queueManager)

// 	// Start the web server
// 	if err := server.Run(); err != nil {
// 		log.Fatal("Error starting web server:", err)
// 	}
// }
