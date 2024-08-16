package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"github.com/NeRF-Or-Nothing/VidGoNerf/webserver/internal/dbschema"
	"your-project-path/dbschema/managers"
)

type RabbitMQServiceV2 struct {
	logger        *log.Logger
	rabbitMQDomain string
	queueManager   *managers.QueueListManager
	sceneManager   *managers.SceneManager
	baseURL        string
	connection     *amqp.Connection
	channel        *amqp.Channel
}

func NewRabbitMQServiceV2(rabbitMQDomain string, queueManager *managers.QueueListManager, sceneManager *managers.SceneManager) (*RabbitMQServiceV2, error) {
	service := &RabbitMQServiceV2{
		logger:         log.New(os.Stdout, "RabbitMQServiceV2: ", log.LstdFlags),
		rabbitMQDomain: rabbitMQDomain,
		queueManager:   queueManager,
		sceneManager:   sceneManager,
		baseURL:        "https://host.docker.internal:5000/",
	}

	err := service.connect()
	if err != nil {
		return nil, err
	}

	go service.startConsumers()

	return service, nil
}

func (s *RabbitMQServiceV2) connect() error {
	timeout := time.Now().Add(2 * time.Minute)
	var err error

	for time.Now().Before(timeout) {
		s.connection, err = amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:5672/", 
			os.Getenv("RABBITMQ_DEFAULT_USER"), 
			os.Getenv("RABBITMQ_DEFAULT_PASS"), 
			s.rabbitMQDomain))
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
		_, err = s.channel.QueueDeclare(queue, true, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %v", queue, err)
		}
	}

	return nil
}

func (s *RabbitMQServiceV2) toURL(filePath string) string {
	return s.baseURL + "worker-data/" + filePath
}

func (s *RabbitMQServiceV2) PublishSFMJob(ctx context.Context, id primitive.ObjectID, vid *dbschema.Video, config *dbschema.TrainingConfig) error {
	job := map[string]interface{}{
		"id":        id.Hex(),
		"file_path": s.toURL(vid.FilePath),
	}

	for k, v := range config.SfmConfig {
		job[k] = v
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

	err = s.queueManager.AppendQueue(ctx, "sfm_list", id.Hex())
	if err != nil {
		return fmt.Errorf("failed to append to sfm_list: %v", err)
	}

	err = s.queueManager.AppendQueue(ctx, "queue_list", id.Hex())
	if err != nil {
		return fmt.Errorf("failed to append to queue_list: %v", err)
	}

	s.logger.Printf("SFM Job Published with ID %s", id.Hex())
	return nil
}

func (s *RabbitMQServiceV2) PublishNERFJob(ctx context.Context, id primitive.ObjectID, vid *dbschema.Video, sfm *dbschema.Sfm, config *dbschema.TrainingConfig) error {
	job := map[string]interface{}{
		"id":         id.Hex(),
		"vid_width":  vid.Width,
		"vid_height": vid.Height,
	}

	sfmData := sfm.ToMap()
	for i, frame := range sfmData["frames"].([]map[string]interface{}) {
		frame["file_path"] = s.toURL(frame["file_path"].(string))
		sfmData["frames"].([]map[string]interface{})[i] = frame
	}

	for k, v := range sfmData {
		job[k] = v
	}
	for k, v := range config.NerfConfig {
		job[k] = v
	}

	jsonJob, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal NERF job: %v", err)
	}

	err = s.channel.Publish("", "nerf-in", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonJob,
	})
	if err != nil {
		return fmt.Errorf("failed to publish NERF job: %v", err)
	}

	err = s.queueManager.AppendQueue(ctx, "nerf_list", id.Hex())
	if err != nil {
		return fmt.Errorf("failed to append to nerf_list: %v", err)
	}

	s.logger.Printf("NERF Job Published with ID %s", id.Hex())
	return nil
}

func (s *RabbitMQServiceV2) startConsumers() {
	go s.consumeSFMOut()
	go s.consumeNERFOut()
}

func (s *RabbitMQServiceV2) consumeSFMOut() {
	messages, err := s.channel.Consume("sfm-out", "", false, false, false, false, nil)
	if err != nil {
		s.logger.Printf("Failed to register a consumer: %v", err)
		return
	}

	for msg := range messages {
		err := s.processSFMJob(msg)
		if err != nil {
			s.logger.Printf("Error processing SFM job: %v", err)
			msg.Nack(false, true)
		} else {
			msg.Ack(false)
		}
	}
}

func (s *RabbitMQServiceV2) processSFMJob(msg amqp.Delivery) error {
	var sfmData map[string]interface{}
	err := json.Unmarshal(msg.Body, &sfmData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal SFM data: %v", err)
	}

	id := sfmData["id"].(string)
	flag := int(sfmData["flag"].(float64))

	if flag == 0 {
		for i, frame := range sfmData["frames"].([]interface{}) {
			frameMap := frame.(map[string]interface{})
			url := frameMap["file_path"].(string)
			filename := filepath.Base(url)
			filePath := filepath.Join("data", "sfm", id, filename)

			err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to create directory: %v", err)
			}

			// Download and save the file
			// Note: Implement the actual file download logic here

			sfmData["frames"].([]interface{})[i].(map[string]interface{})["file_path"] = filePath
		}
	}

	delete(sfmData, "flag")

	vid := dbschema.VideoFromMap(sfmData)
	sfm := dbschema.SfmFromMap(sfmData)

	ctx := context.Background()

	err = s.sceneManager.SetSfm(ctx, id, sfm)
	if err != nil {
		return fmt.Errorf("failed to set SFM: %v", err)
	}

	err = s.sceneManager.SetVideo(ctx, id, vid)
	if err != nil {
		return fmt.Errorf("failed to set Video: %v", err)
	}

	config, err := s.sceneManager.GetTrainingConfig(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get training config: %v", err)
	}

	err = s.queueManager.PopQueue(ctx, "sfm_list", id)
	if err != nil {
		return fmt.Errorf("failed to pop from sfm_list: %v", err)
	}

	if flag == 0 {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return fmt.Errorf("failed to convert ID to ObjectID: %v", err)
		}
		
		err = s.PublishNERFJob(ctx, oid, vid, sfm, config)
		if err != nil {
			return fmt.Errorf("failed to publish NERF job: %v", err)
		}
	} else {
		err = s.queueManager.PopQueue(ctx, "queue_list", id)
		if err != nil {
			return fmt.Errorf("failed to pop from queue_list: %v", err)
		}

		nerf := dbschema.NerfV2{Flag: flag}
		err = s.sceneManager.SetNerfV2(ctx, id, &nerf)
		if err != nil {
			return fmt.Errorf("failed to set NerfV2: %v", err)
		}
	}

	return nil
}

func (s *RabbitMQServiceV2) consumeNERFOut() {
	messages, err := s.channel.Consume("nerf-out", "", false, false, false, false, nil)
	if err != nil {
		s.logger.Printf("Failed to register a consumer: %v", err)
		return
	}

	for msg := range messages {
		err := s.processNERFJob(msg)
		if err != nil {
			s.logger.Printf("Error processing NERF job: %v", err)
			msg.Nack(false, true)
		} else {
			msg.Ack(false)
		}
	}
}

func (s *RabbitMQServiceV2) processNERFJob(msg amqp.Delivery) error {
	var nerfData map[string]interface{}
	err := json.Unmarshal(msg.Body, &nerfData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal NERF data: %v", err)
	}

	id := nerfData["id"].(string)
	ctx := context.Background()

	nerf, err := s.sceneManager.GetNerfV2(ctx, id)
	if err != nil {
		s.logger.Printf("Could not find nerf object for id %s, creating a new one", id)
		nerf = &dbschema.NerfV2{}
	}

	outputEndpoints := nerfData["output_endpoints"].(map[string]interface{})
	config, err := s.sceneManager.GetTrainingConfig(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get training config: %v", err)
	}

	outputTypes := config.NerfConfig["output_types"].([]string)
	saveIterations := config.NerfConfig["save_iterations"].([]int)
	outputPath := filepath.Join("data", "nerf", id)

	for endpointType, endpointData := range outputEndpoints {
		if _, exists := nerf.ModelFilePathsMap[endpointType]; !exists {
			nerf.ModelFilePathsMap[endpointType] = make(map[int]string)
		}

		extension := s.getExtensionForType(endpointType)
		if extension == "" {
			s.logger.Printf("Unexpected endpoint type received. Skipping Saving. Job %s", id)
			continue
		}

		endpointInfo := endpointData.(map[string]interface{})
		for _, iteration := range endpointInfo["save_iterations"].([]interface{}) {
			iter := int(iteration.(float64))
			
			// Download and save the file
			// Note: Implement the actual file download logic here

			filePath := filepath.Join(outputPath, endpointType, fmt.Sprintf("iteration_%d", iter), fmt.Sprintf("%s.%s", id, extension))
			nerf.ModelFilePathsMap[endpointType][iter] = filePath
		}
	}

	nerf.Flag = 0

	err = s.sceneManager.SetNerfV2(ctx, id, nerf)
	if err != nil {
		return fmt.Errorf("failed to set NerfV2: %v", err)
	}

	err = s.queueManager.PopQueue(ctx, "nerf_list", id)
	if err != nil {
		return fmt.Errorf("failed to pop from nerf_list: %v", err)
	}

	err = s.queueManager.PopQueue(ctx, "queue_list", id)
	if err != nil {
		return fmt.Errorf("failed to pop from queue_list: %v", err)
	}

	return nil
}

func (s *RabbitMQServiceV2) getExtensionForType(endpointType string) string {
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