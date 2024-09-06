// This file contains the implementation of AMPQService. This service is responsible for handling the communication
// Between the web server and an AMPQ message broker, and thus workers. The service is the main handler for the training pipeline
// and is responsible for sending and receiving messages to and from the workers, as well as updating the database with the results.
//
// This service expects a rabbitMQ AMPQ 0.9.1 broker to be running on the specified domain. The service connects to the broker and
// creates the necessary queues for communication. The service then starts consumers for the 'sfm-out' and 'nerf-out' queues, which
// are responsible for processing the output of the workers.
//
// A go channel and waitgroup are used to manage the consumers, and the service can be gracefully shutdown by closing the stopChan.
// The consumers should *hopefully* be tolerant to connection failures, and will attempt to reconnect every 5 seconds if the connection
// is lost.

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/NeRF-or-Nothing/go-web-server/internal/log"
	"github.com/NeRF-or-Nothing/go-web-server/internal/models/queue"
	"github.com/NeRF-or-Nothing/go-web-server/internal/models/scene"
)

type AMPQService struct {
	baseURL             string
	messageBrokerDomain string
	sceneManager        *scene.SceneManager
	queueManager        *queue.QueueListManager
	connection          *amqp.Connection
	channel             *amqp.Channel
	logger              *log.Logger
	// used for reconnection and graceful shutdown
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// Starts a new AMPQService instance as goroutine
func NewAMPQService(messageBrokerDomain string, sceneManager *scene.SceneManager, queueManager *queue.QueueListManager, logger *log.Logger) (*AMPQService, error) {
	service := &AMPQService{
		messageBrokerDomain: messageBrokerDomain,
		queueManager:        queueManager,
		sceneManager:        sceneManager,
		baseURL:             "http://web-server:5000/",
		logger:              logger,
		stopChan:            make(chan struct{}),
	}

	err := service.connect()
	if err != nil {
		return nil, err
	}

	go service.startConsumers()

	return service, nil
}

// connect establishes a connection to the AMPQ message broker and creates the necessary queues
func (s *AMPQService) connect() error {
	fmt.Println("AMPQService.connect")
	timeout := time.Now().Add(time.Minute / 4)
	var err error

	for time.Now().Before(timeout) {
		s.connection, err = amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:5672/",
			os.Getenv("RABBITMQ_DEFAULT_USER"),
			os.Getenv("RABBITMQ_DEFAULT_PASS"),
			s.messageBrokerDomain))
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	s.channel, err = s.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %v", err)
	}

	// Declare queues with 1 hour consumer timeout
	queues := []string{"sfm-in", "nerf-in", "sfm-out", "nerf-out"}
	for _, queue := range queues {
		args := amqp.Table{
			"x-consumer-timeout": int64(time.Hour.Milliseconds()),
		}
		_, err = s.channel.QueueDeclare(queue, false, false, false, false, args)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %v", queue, err)
		}
	}

	return nil
}

// startConsumers starts the consumers for the AMPQ queues.
//
// consumers are started as goroutines, and the function waits for them to finish using a WaitGroup.
func (s *AMPQService) startConsumers() {
	go s.runConsumer("sfm-out", s.processSFMJob)
	go s.runConsumer("nerf-out", s.processNERFJob)
}

// runConsumer runs a consumer for the specified queue and consumption handler
func (s *AMPQService) runConsumer(queueName string, processFunc func(amqp.Delivery) error) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			s.logger.Infof("Stopping %s consumer", queueName)
			return
		default:
			if err := s.consume(queueName, processFunc); err != nil {
				s.logger.Errorf("Error in %s consumer: %v. Reconnecting in 5 seconds...", queueName, err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// consume consumes messages from the specified queue and processes them using the provided function
func (s *AMPQService) consume(queueName string, processFunc func(amqp.Delivery) error) error {
	if err := s.ensureConnection(); err != nil {
		return fmt.Errorf("failed to ensure connection: %v", err)
	}

	ch, err := s.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %v", err)
	}
	defer ch.Close()

	messages, err := ch.Consume(
		queueName, "", false, false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %v", err)
	}

	s.logger.Infof("Started consuming from %s", queueName)

	for msg := range messages {
		if err := processFunc(msg); err != nil {
			s.logger.Errorf("Error processing message from %s: %v", queueName, err)
			msg.Nack(false, true) // Negative acknowledge and requeue
		} else {
			msg.Ack(false)
		}
	}

	return fmt.Errorf("consumer channel closed")
}

// ensureConnection ensures that the AMPQ connection is established
func (s *AMPQService) ensureConnection() error {
	if s.connection != nil && !s.connection.IsClosed() {
		return nil
	}

	s.logger.Info("Reconnecting to RabbitMQ...")
	return s.connect()
}

// Shutdown shuts down the AMPQ service
func (s *AMPQService) Shutdown() {
	s.logger.Info("Shutting down AMQP service...")
	close(s.stopChan)
	s.wg.Wait()
	if s.connection != nil {
		s.connection.Close()
	}
	s.logger.Info("AMQP service shut down")
}

// toAPIUrl converts a file path to an API URL
func (s *AMPQService) toAPIUrl(filePath string) string {
	return s.baseURL + "worker-data/" + filePath
}

// PublishSFMJob publishes a new SFM job to the AMPQ message broker.
//
// The job is published to the 'sfm-in' queue, and the scene ID is appended to the 'sfm_list' and 'queue_list' queues.
//
// Returns an error if the job could not be published.
func (s *AMPQService) PublishSFMJob(ctx context.Context, scene *scene.Scene) error {
	job := map[string]interface{}{
		"id":        scene.ID.Hex(),
		"file_path": s.toAPIUrl(scene.Video.FilePath),
	}

	jsonJob, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal SFM job: %v", err)
	}

	err = s.channel.PublishWithContext(ctx, "", "sfm-in", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonJob,
	})
	if err != nil {
		return fmt.Errorf("failed to publish SFM job: %v", err)
	}

	err = s.queueManager.AppendToQueue(ctx, "sfm_list", scene.ID)
	if err != nil {
		return fmt.Errorf("failed to append to sfm_list: %v", err)
	}

	err = s.queueManager.AppendToQueue(ctx, "queue_list", scene.ID)
	if err != nil {
		return fmt.Errorf("failed to append to queue_list: %v", err)
	}

	s.logger.Infof("SFM Job Published with ID %s", scene.ID.Hex())
	return nil
}

// processSFMJob processes a message from the 'sfm-out' queue.
//
// The message is expected to contain the output of the SFM worker, which is then processed and saved to the database.
// Upon successful processing, the scene is removed from the 'sfm_list' queue and a new NERF job is published.
//
// This function TRUSTS the output of the SFM worker, and does not perform any validation on the message.
// The expected message format is:
//
//	{
//  	"id": string (primitive.ObjectID.Hex()),
//  	"vid_width": int,
//  	"vid_height": int,
//  	"sfm": {
//  	    "intrinsic_matrix": [[float64]] 3x3,
//  	    "frames": [
//  	        {
//  	            "file_path": string (url),
//  	            "extrinsic_matrix": [[float64]] 4x4
//  	        },
//  	        ...
//  	    ],
//  	    "white_background": bool
//  	},
//  	"flag": someInt
//	}
func (s *AMPQService) processSFMJob(d amqp.Delivery) error {
	type SfmWorkerData struct {
		SceneID   string    `json:"id"`
		VidWidth  int       `json:"vid_width"`
		VidHeight int       `json:"vid_height"`
		Sfm       scene.Sfm `json:"sfm"`
		Flag      int       `json:"flag"`
	}

	var data SfmWorkerData

	// Decode sfm-worker output
	err := json.Unmarshal(d.Body, &data)
	if err != nil {
		s.logger.Errorf("Error unmarshalling SFM data: %v", err)
		d.Nack(false, true)
		return err
	}

	s.logger.Debug("Processing SFM job: ", data)

	sceneID, err := primitive.ObjectIDFromHex(data.SceneID)
	if err != nil {
		s.logger.Errorf("Invalid ID format: %v", err)
		d.Nack(false, true)
		return err
	}

	ctx := context.Background()

	// Create sfm output directory
	saveDir := filepath.Join("data", "sfm", sceneID.Hex())
	err = os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		s.logger.Errorf("Error creating directory: %v", err)
		return fmt.Errorf("error creating directory: %v", err)
	}

	// Process the frames: download and save the files
	for i, frame := range data.Sfm.Frames {
		url := frame.FilePath
		s.logger.Debugf("Downloading image from %s", url)

		resp, err := http.Get(url)
		if err != nil {
			s.logger.Errorf("Error downloading image: %v", err)
			return fmt.Errorf("error downloading image: %v", err)
		}
		defer resp.Body.Close()

		// Download and save the file
		fileName := filepath.Base(url)
		filePath := filepath.Join(saveDir, fileName)

		file, err := os.Create(filePath)
		if err != nil {
			s.logger.Errorf("Error creating file: %v", err)
			return fmt.Errorf("error creating file: %v", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			s.logger.Errorf("Error saving file: %v", err)
			return fmt.Errorf("error saving file: %v", err)
		}

		s.logger.Infof("File saved at %s", filePath)

		data.Sfm.Frames[i].FilePath = s.toAPIUrl(filePath)
	}

	// Update the scene with the new SFM Worker data
	currentScene, err := s.sceneManager.GetScene(ctx, sceneID)
	if err != nil {
		s.logger.Errorf("Error getting scene: %v", err)
		d.Nack(false, true)
		return err
	}

	// Assumes that scene, scene.Video, and scene.Config are already populated
	currentScene.Sfm = &data.Sfm
	currentScene.Video.Width = data.VidWidth
	currentScene.Video.Height = data.VidHeight

	err = s.sceneManager.SetScene(ctx, sceneID, currentScene)
	if err != nil {
		s.logger.Errorf("Error setting scene data: %v", err)
		d.Nack(false, true)
		return err
	}

	// Remove from sfm_list queue
	err = s.queueManager.DeleteFromQueue(ctx, "sfm_list", sceneID)
	if err != nil {
		s.logger.Errorf("Error popping from sfm_list queue: %v", err)
	}

	s.logger.Debug("Saved finished SFM job")

	// Publish new job to nerf-in
	err = s.PublishNERFJob(ctx, currentScene)
	if err != nil {
		s.logger.Errorf("Error publishing NERF job: %v", err)
		d.Nack(false, true)
		return err
	}

	d.Ack(false)
	return nil
}

// PublishNERFJob publishes a new NERF job to the AMPQ message broker.
// The job is published to the 'nerf-in' queue, and the scene ID is appended to the 'nerf_list' queue.
//
// Returns an error if the job could not be published.
func (s *AMPQService) PublishNERFJob(ctx context.Context, scene *scene.Scene) error {
	// Extract data from scene
	sceneID := scene.ID
	vid := scene.Video
	sfm := scene.Sfm
	config := scene.Config

	// Construct job
	jobMap := map[string]interface{}{
		"id":               sceneID.Hex(),
		"vid_width":        vid.Width,
		"vid_height":       vid.Height,
		"frames":           sfm.Frames,
		"intrinsic_matrix": sfm.IntrinsicMatrix,
		"white_background": sfm.WhiteBackground,
		"output_types":     config.NerfTrainingConfig.OutputTypes,
		"training_mode":    config.NerfTrainingConfig.TrainingMode,
		"save_iterations":  config.NerfTrainingConfig.SaveIterations,
		"total_iterations": config.NerfTrainingConfig.TotalIterations,
	}

	jobJson, err := json.Marshal(jobMap)
	if err != nil {
		s.logger.Errorf("Failed to marshal NERF job: %v", err)
		return fmt.Errorf("failed to marshal NERF job: %v", err)
	}

	s.logger.Debugf("Job JSON: %s", jobJson)

	// Publish job
	err = s.channel.PublishWithContext(ctx, "", "nerf-in", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        jobJson,
	})
	if err != nil {
		s.logger.Errorf("Failed to publish NERF job: %v", err)
		return fmt.Errorf("failed to publish NERF job: %v", err)
	}

	// Append to nerf_list queue
	err = s.queueManager.AppendToQueue(ctx, "nerf_list", sceneID)
	if err != nil {
		return fmt.Errorf("failed to append to nerf_list: %v", err)
	}

	s.logger.Debug("NERF Job Published with ID ", sceneID.Hex())
	return nil
}

// processNERFJob processes a message from the 'nerf-out' queue
//
// The message is expected to contain the output of the NERF worker, which is then processed and saved to the file system & database.
// Upon successful processing, the scene is removed from the 'nerf_list' and 'queue_list' queues.
//
// This function TRUSTS the output of the nerf worker, and only validates the output types
// and iterations against the scene config.
//
// The expected message format is:
//
//	{
//	    "id": string (primitive.ObjectID.Hex()),
//	    "file_paths": {
//	        "typeA": {
//	            int (iteration): string (url),
//				 ...
//	        },
//	        "typeB": {
//				...
//	        },
//	        ...
//		}
//	}
func (s *AMPQService) processNERFJob(msg amqp.Delivery) error {
	type IterationPaths map[int]string
	type FilePaths map[string]IterationPaths
	type NerfWorkerData struct {
		SceneID   string    `json:"id"`
		FilePaths FilePaths `json:"file_paths"`
	}

	var data NerfWorkerData
	err := json.Unmarshal(msg.Body, &data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal NERF worker data: %w", err)
	}

	s.logger.Debug("Processing NERF job: ", data)

	sceneID, err := primitive.ObjectIDFromHex(data.SceneID)
	if err != nil {
		return fmt.Errorf("invalid ID format: %v", err)
	}

	ctx := context.Background()

	currentScene, err := s.sceneManager.GetScene(ctx, sceneID)
	if err != nil {
		return fmt.Errorf("failed to get scene: %v", err)
	}

	nerf := &scene.Nerf{}
	s.logger.Debug("Current Nerf: ", nerf)
	config := currentScene.Config
	s.logger.Debug("Current Config: ", config)
	outputTypes := config.NerfTrainingConfig.OutputTypes
	s.logger.Debug("Output Types: ", outputTypes)
	saveIterations := config.NerfTrainingConfig.SaveIterations
	s.logger.Debug("Save Iterations: ", saveIterations)

	saveDir := filepath.Join("data", "nerf", sceneID.Hex())
	// Create the save directory if it doesn't exist
	err = os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create save directory: %v", err)
	}

	for outputType, outputTypeURLs := range data.FilePaths {

		// Create the type save directory if it doesn't exist
		typeSaveDir := filepath.Join(saveDir, outputType)
		err = os.MkdirAll(typeSaveDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create save directory for type %s: %v", outputType, err)
		}

		if !slices.Contains(outputTypes, outputType) {
			return fmt.Errorf("output type unwanted by config: %s", outputType)
		}

		for iteration, URL := range outputTypeURLs {

			// Create the iteration save directory if it doesn't exist
			iterSaveDir := filepath.Join(typeSaveDir, fmt.Sprintf("iteration_%d", iteration))
			err = os.MkdirAll(iterSaveDir, os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to create save directory for output/iteration %d: %v", iteration, err)
			}

			if !slices.Contains(saveIterations, iteration) {
				return fmt.Errorf("iteration unwanted by config: %d", iteration)
			}

			// Download and save the file
			resp, err := http.Get(URL)
			if err != nil {
				return fmt.Errorf("error downloading file: %v", err)
			}
			defer resp.Body.Close()

			fileName := filepath.Base(URL)
			filePath := filepath.Join(iterSaveDir, fileName)
			file, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("error creating file: %v", err)
			}
			defer file.Close()

			_, err = io.Copy(file, resp.Body)
			if err != nil {
				return fmt.Errorf("error saving file: %v", err)
			}

			switch outputType {
			case "splat_cloud":
				if nerf.SplatCloudFilePathsMap == nil {
					nerf.SplatCloudFilePathsMap = make(map[int]string)
				}
				nerf.SplatCloudFilePathsMap[iteration] = filePath
			case "point_cloud":
				if nerf.PointCloudFilePathsMap == nil {
					nerf.PointCloudFilePathsMap = make(map[int]string)
				}
				nerf.PointCloudFilePathsMap[iteration] = filePath
			case "video":
				if nerf.VideoFilePathsMap == nil {
					nerf.VideoFilePathsMap = make(map[int]string)
				}
				nerf.VideoFilePathsMap[iteration] = filePath
			case "model":
				if nerf.ModelFilePathsMap == nil {
					nerf.ModelFilePathsMap = make(map[int]string)
				}
				nerf.ModelFilePathsMap[iteration] = filePath
			default:
				s.logger.Errorf("Unexpected output type: %v. Orphaned file now in system", outputType)
			}

			s.logger.Debug("File saved at ", filePath)
		}
	}

	err = s.sceneManager.SetNerf(ctx, sceneID, nerf)
	if err != nil {
		return fmt.Errorf("failed to set Nerf: %v", err)
	}

	err = s.queueManager.DeleteFromQueue(ctx, "nerf_list", sceneID)
	if err != nil {
		return fmt.Errorf("failed to pop from nerf_list: %v", err)
	}

	err = s.queueManager.DeleteFromQueue(ctx, "queue_list", sceneID)
	if err != nil {
		return fmt.Errorf("failed to pop from queue_list: %v", err)
	}

	return nil
}
