// This file contains the implementation of AMPQService. This service is responsible for handling the communication
// Between the web server and an AMPQ message broker, and thus workers. The service is the main handler for the training pipeline
// and is responsible for sending and receiving messages to and from the workers, as well as updating the database with the results.

// This service expects a AMPQ 0.9.1 broker to be running on the specified domain. The service connects to the broker and
// creates the necessary queues for communication. The service then starts goroutines for consumers for the queues, which listen for messages
// from the workers

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/log"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/queue"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/scene"
)

type AMPQService struct {
	baseURL             string
	messageBrokerDomain string
	queueManager        *queue.QueueListManager
	sceneManager        *scene.SceneManager
	connection          *amqp.Connection
	channel             *amqp.Channel
	logger              *log.Logger
}

// Starts a new AMPQService instance as goroutine
func NewAMPQService(messageBrokerDomain string, queueManager *queue.QueueListManager, sceneManager *scene.SceneManager, logger *log.Logger) (*AMPQService, error) {
	service := &AMPQService{
		messageBrokerDomain: messageBrokerDomain,
		queueManager:        queueManager,
		sceneManager:        sceneManager,
		baseURL:             "http://host.docker.internal:5000/",
		logger:              logger,
	}

	err := service.connect()
	if err != nil {
		return nil, err
	}

	go service.startConsumers()

	return service, nil
}

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

	queues := []string{"sfm-in", "nerf-in", "sfm-out", "nerf-out"}
	for _, queue := range queues {
		_, err = s.channel.QueueDeclare(queue, false, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %v", queue, err)
		}
	}

	return nil
}


func (s *AMPQService) startConsumers() {
	go s.consumeSFMOut()
	// go s.consumeNERFOut()
}


func (s *AMPQService) toAPIUrl(filePath string) string {
	return s.baseURL + "worker-data/" + filePath
}

func (s *AMPQService) PublishSFMJob(ctx context.Context, id primitive.ObjectID, vid *scene.Video, config *scene.TrainingConfig) error {
	job := map[string]interface{}{
		"id":        id.Hex(),
		"file_path": s.toAPIUrl    (vid.FilePath),
	}

	jsonJob, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal SFM job: %v", err)
	}

	err = s.channel.Publish("", "sfm-in", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonJob,
	})
	if err != nil {
		return fmt.Errorf("failed to publish SFM job: %v", err)
	}

	err = s.queueManager.AppendToQueue(ctx, "sfm_list", id.Hex())
	if err != nil {
		return fmt.Errorf("failed to append to sfm_list: %v", err)
	}

	err = s.queueManager.AppendToQueue(ctx, "queue_list", id.Hex())
	if err != nil {
		return fmt.Errorf("failed to append to queue_list: %v", err)
	}

	s.logger.Infof("SFM Job Published with ID %s", id.Hex())
	return nil
}

func (s *AMPQService) PublishNERFJob(ctx context.Context, id primitive.ObjectID, vid *scene.Video, sfm *scene.Sfm, config *scene.TrainingConfig) error {
	job := map[string]interface{}{
		"id":         id.Hex(),
		"vid_width":  vid.Width,
		"vid_height": vid.Height,
	}

	jobJson, err := json.Marshal(job)
	if err != nil {
		s.logger.info("Failed to marshal video part of nerf job: %v", err)
		return fmt.Errorf("failed to marshal video part of nerf job: %v", err)
	}
	s.logger.info("Job JSON: %v", jobJson)

	sfmJson, err := json.Marshal(sfm)
	if err != nil {
		s.logger.info("Failed to marshal sfm part of nerf job: %v", err)
		return fmt.Errorf("failed to marshal sfm part of nerf job: %v", err)
	}
	s.logger.info("SFM JSON: %v", sfmJson)

	nerfConfigJson, err := json.Marshal(config.NerfTrainingConfig)
	if err != nil {
		s.logger.info("Failed to marshal nerf config part of nerf job: %v", err)
		return fmt.Errorf("failed to marshal nerf config part of nerf job: %v", err)
	}
	s.logger.info("NERF Config JSON: %v", nerfConfigJson)


	// Combine the JSONs
	append(jobJson, sfmJson...)
	append(jobJson, nerfConfigJson...)
	s.logger.info("Combined JSON: %v", jobJson)

	err = s.channel.Publish("", "nerf-in", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        jobJson,
	})
	if err != nil {
		return fmt.Errorf("failed to publish NERF job: %v", err)
	}

	err = s.queueManager.AppendQueue(ctx, "nerf_list", id.Hex())
	if err != nil {
		return fmt.Errorf("failed to append to nerf_list: %v", err)
	}

	s.logger.Infof("NERF Job Published with ID %s", id.Hex())
	return nil
}

func (s *AMPQService) consumeSFMOut() {
	messages, err := s.channel.Consume("sfm-out", "", false, false, false, false, nil)
	if err != nil {
		s.logger.Infof("Failed to register a consumer: %v", err)
		return
	}

	for msg := range messages {
		err := s.processSFMJob(msg)
		if err != nil {
			s.logger.Infof("Error processing SFM job: %v", err)
			msg.Nack(false, true)
		} else {
			msg.Ack(false)
		}
	}
}

func (s *AMPQService) processSFMJob(d amqp.Delivery) (error) {
	type SfmWorkerData struct {
		SceneID   string      `json:"id"`
		VidWidth  int         `json:"vid_width"`
		VidHeight int         `json:"vid_height"`
		Sfm 	  scene.Sfm   `json:"sfm"`
		Flag      int         `json:"flag"`
	}
	
    var sfmWorkerData SfmWorkerData

    err := json.Unmarshal(d.Body, &sfmWorkerData)
    if err != nil {
        s.logger.Errorf("Error unmarshalling SFM data: %v", err)
        d.Nack(false, true)
        return err
    }

    // Extract scene ID from the message or use a predefined field
    sceneID, err := primitive.ObjectIDFromHex(sfmWorkerData.SceneID)
    if err != nil {
        s.logger.Errorf("Invalid ID format: %v", err)
        d.Nack(false, true)
        return err
    }

    s.logger.Infof("SFM TASK RETURNED FOR ID %s", sceneID.Hex())

    ctx := context.Background()

	// Process the frames: download and save the files
	for i, frame := range sfmWorkerData.Sfm.Frames {
        url := frame.FilePath
        s.logger.Infof("Downloading image from %s", url)

        resp, err := http.Get(url)
        if err != nil {
            return fmt.Errorf("error downloading image: %v", err)
        }
        defer resp.Body.Close()

		// Download and save the file
		fileDir := filepath.Join("/data/sfm/", sceneID.Hex())
		fileName := filepath.Base(url)
		filePath := filepath.Join(fileDir, fileName)

		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("error creating file: %v", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return fmt.Errorf("error saving file: %v", err)
		}

		s.logger.Infof("File saved at %s", filePath)
		sfmWorkerData.Sfm.Frames[i].FilePath = s.toAPIUrl(url)
    }


    // Update the Scene in the database
    currentScene, err := s.sceneManager.GetScene(ctx, id)
    if err != nil {
        if err == scene.ErrSceneNotFound {
            scene_ = &scene.Scene{ID: id}
        } else {
            s.logger.Errorf("Error getting scene: %v", err)
            d.Nack(false, true)
            return err
        }
    }


    // Update Sfm and Video
    currentScene.Sfm = &sfmWorkerData.Sfm
    
    if currentScene.Video == nil {
        currentScene.Video = &scene.Video{}
    }

    currentScene.Video.Width = sfmWorkerData.VidWidth
    currentScene.Video.Height = sfmWorkerData.VidHeight

    err = s.sceneManager.SetScene(ctx, id, currentScene)
    if err != nil {
        s.logger.Errorf("Error setting scene data: %v", err)
        d.Nack(false, true)
        return
    }

    // Get the training config
    config, err := s.sceneManager.GetTrainingConfig(ctx, id)
    if err != nil {
        s.logger.Errorf("Error getting training config: %v", err)
        d.Nack(false, true)
        return
    }

    // Remove from sfm_list queue
    err = s.queueManager.PopFromQueue("sfm_list", id.Hex())
    if err != nil {
        s.logger.Errorf("Error popping from sfm_list queue: %v", err)
    }

    s.logger.Info("Saved finished SFM job")

    // Publish new job to nerf-in
    err = s.PublishNerfJob(id.Hex(), scene.Video, scene.Sfm, config)
    if err != nil {
        s.logger.Errorf("Error publishing NERF job: %v", err)
        d.Nack(false, true)
        return
    }

    d.Ack(false)
}

func (s *AMPQService) processFrames(sfmData *SfmWorkerData) error {
    for i, frame := range sfmData.Frames {
        url := frame.FilePath
        s.logger.Infof("Downloading image from %s", url)

        resp, err := http.Get(url)
        if err != nil {
            return fmt.Errorf("error downloading image: %v", err)
        }
        defer resp.Body.Close()

        urlPath, err := url.Parse(frame.FilePath)
        if err != nil {
            return fmt.Errorf("error parsing URL: %v", err)
        }

        filename := path.Base(urlPath.Path)
        filePath := filepath.Join("data/sfm", sfmData.ID, filename)

        err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
        if err != nil {
            return fmt.Errorf("error creating directory: %v", err)
        }

        file, err := os.Create(filePath)
        if err != nil {
            return fmt.Errorf("error creating file: %v", err)
        }
        defer file.Close()

        _, err = io.Copy(file, resp.Body)
        if err != nil {
            return fmt.Errorf("error writing file: %v", err)
        }

        sfmData.Frames[i].FilePath = filePath
    }

    return nil
}

// func (s *AMPQService) consumeNERFOut() {
// 	messages, err := s.channel.Consume("nerf-out", "", false, false, false, false, nil)
// 	if err != nil {
// 		s.logger.Infof("Failed to register a consumer: %v", err)
// 		return
// 	}

// 	for msg := range messages {
// 		err := s.processNERFJob(msg)
// 		if err != nil {
// 			s.logger.Infof("Error processing NERF job: %v", err)
// 			msg.Nack(false, true)
// 		} else {
// 			msg.Ack(false)
// 		}
// 	}
// }

// func (s *AMPQService) processNERFJob(msg amqp.Delivery) error {
// 	var nerfData map[string]interface{}
// 	err := json.Unmarshal(msg.Body, &nerfData)
// 	if err != nil {
// 		return fmt.Errorf("failed to unmarshal NERF data: %v", err)
// 	}

// 	id := nerfData["id"].(string)
// 	ctx := context.Background()

// 	nerf, err := s.sceneManager.GetNerf(ctx, id)
// 	if err != nil {
// 		s.logger.Infof("Could not find nerf object for id %s, creating a new one", id)
// 		nerf = &dbschema.Nerf{}
// 	}

// 	outputEndpoints := nerfData["output_endpoints"].(map[string]interface{})
// 	config, err := s.sceneManager.GetTrainingConfig(ctx, id)
// 	if err != nil {
// 		return fmt.Errorf("failed to get training config: %v", err)
// 	}

// 	outputTypes := config.NerfConfig["output_types"].([]string)
// 	saveIterations := config.NerfConfig["save_iterations"].([]int)
// 	outputPath := filepath.Join("data", "nerf", id)

// 	for endpointType, endpointData := range outputEndpoints {
// 		if _, exists := nerf.ModelFilePathsMap[endpointType]; !exists {
// 			nerf.ModelFilePathsMap[endpointType] = make(map[int]string)
// 		}

// 		extension := s.getExtensionForType(endpointType)
// 		if extension == "" {
// 			s.logger.Infof("Unexpected endpoint type received. Skipping Saving. Job %s", id)
// 			continue
// 		}

// 		endpointInfo := endpointData.(map[string]interface{})
// 		for _, iteration := range endpointInfo["save_iterations"].([]interface{}) {
// 			iter := int(iteration.(float64))

// 			// Download and save the file
// 			// Note: Implement the actual file download logic here

// 			filePath := filepath.Join(outputPath, endpointType, fmt.Sprintf("iteration_%d", iter), fmt.Sprintf("%s.%s", id, extension))
// 			nerf.ModelFilePathsMap[endpointType][iter] = filePath
// 		}
// 	}

// 	nerf.Flag = 0

// 	err = s.sceneManager.SetNerf(ctx, id, nerf)
// 	if err != nil {
// 		return fmt.Errorf("failed to set Nerf: %v", err)
// 	}

// 	err = s.queueManager.PopQueue(ctx, "nerf_list", id)
// 	if err != nil {
// 		return fmt.Errorf("failed to pop from nerf_list: %v", err)
// 	}

// 	err = s.queueManager.PopQueue(ctx, "queue_list", id)
// 	if err != nil {
// 		return fmt.Errorf("failed to pop from queue_list: %v", err)
// 	}

// 	return nil
// }

func (s *AMPQService) getExtensionForType(endpointType string) string {
	switch endpointType {
	case "splat_cloud":
		return "splat"
	case "point_cloud":
		return "ply"
	case "video":
		return "mp4"
	case "model":
		return "th"
	default:
		return ""
	}
}
