// This file contains the ClientService implementation, which is responsible for handling client requests.
// The majority of work for an external api request SHOULD be handled by this service.
//
// Any work that needs database access should be delegated to the appropriate manager.
//
// In order to keep memory usage low, the service should not store any data that can be easily
// retrieved from the databases, and also not open/read files when it can be avoided.

package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/log"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/queue"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/scene"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/user"
)

type ClientService struct {
	mqService    *AMPQService
	sceneManager *scene.SceneManager
	userManager  *user.UserManager
	queueManager *queue.QueueListManager
	logger       *log.Logger
}

// NewClientService creates a new ClientService. Dependencies are injected via the constructor.
func NewClientService(mqs *AMPQService, sm *scene.SceneManager, um *user.UserManager, qlm *queue.QueueListManager, logger *log.Logger) *ClientService {
	return &ClientService{
		mqService:    mqs,
		sceneManager: sm,
		userManager:  um,
		queueManager: qlm,
		logger:       logger,
	}
}

// verifyUserAccess checks if the given user has access to the given scene.
//
// Returns nil if the user has access, error if the user does not have access or an error occurred.
func (s *ClientService) verifyUserAccess(ctx context.Context, userID, sceneID primitive.ObjectID) error {
	authorized, err := s.userManager.UserHasJobAccess(ctx, userID, sceneID)
	if err != nil {
		return err
	}
	if !authorized {
		return user.ErrUserNoAccess
	}
	return nil
}

// LoginUser checks if the given username and password are correct and returns the user's ID, nil if successful.
//
// Returns "", error if the username or password is incorrect.
func (s *ClientService) LoginUser(ctx context.Context, username, password string) (string, error) {
	user, err := s.userManager.GetUserByUsername(ctx, username)
	if err != nil {
		return "", err
	}

	err = user.CheckPassword(password)
	if err != nil {
		return "", err
	}

	return user.ID.Hex(), nil
}

// RegisterUser generates a new user document with the given username and password, and inserts it into the database.
//
// Returns nil if successful, error if the username is already taken or an error occurred while inserting the user.
func (s *ClientService) RegisterUser(ctx context.Context, username, password string) error {
	_, err := s.userManager.GenerateUser(ctx, username, password)
	if err != nil {
		return err
	}

	return nil
}

// UpdateUserUsername updates the username of the user with the given ID.
//
// Returns nil if successful, error if the user does not exist or an error occurred.
func (s *ClientService) UpdateUserUsername(ctx context.Context, userID primitive.ObjectID, password, newUsername string) error {
	return s.userManager.UpdateUsername(ctx, userID, password, newUsername)
}

// UpdateUserPassword updates the password of the user with the given ID.
//
// Returns nil if successful, error if the user does not exist or an error occurred.
func (s *ClientService) UpdateUserPassword(ctx context.Context, userID primitive.ObjectID, oldPassword, newPassword string) error {
	return s.userManager.UpdatePassword(ctx, userID, oldPassword, newPassword)
}

// GetSceneMetadata returns metadata about the resources available for the given scene.
//
// Returns error if the user does not have access to the scene or an error occurred.
// For each available output file type, it returns a map of iteration numbers to file information.
// Specifically, it returns whether the file exists, its size, number of (1 MB) chunks, and size of the last chunk.
func (s *ClientService) GetSceneMetadata(ctx context.Context, userID, sceneID primitive.ObjectID) (interface{}, error) {
	// Information about a single resource available for a scene.
	type ResourceInfo struct {
		Exists        bool  `json:"exists"`
		Size          int64 `json:"size,omitempty"`
		Chunks        int   `json:"chunks,omitempty"`
		LastChunkSize int64 `json:"last_chunk_size,omitempty"`
	}
	// Metadata about all resources available for a scene.
	type SceneMetadata struct {
		Resources map[string]map[string]ResourceInfo `json:"resources"`
	}

	if err := s.verifyUserAccess(ctx, userID, sceneID); err != nil {
		return nil, err
	}

	nerf, err := s.sceneManager.GetNerf(ctx, sceneID)
	if err != nil {
		return nil, err
	}

	config, err := s.sceneManager.GetTrainingConfig(ctx, sceneID)
	if err != nil {
		return nil, err
	}

	metadata := &SceneMetadata{
		Resources: make(map[string]map[string]ResourceInfo),
	}

	for _, ot := range config.NerfTrainingConfig.OutputTypes {

		s.logger.Debug("Getting file paths for output type:", ot)

		metadata.Resources[ot] = make(map[string]ResourceInfo)

		iterFilePaths, err := nerf.GetFilePathsForType(ot)
		if err != nil {
			return nil, err
		}

		for iteration, path := range iterFilePaths {

			s.logger.Debug("Getting file info for iteration:", iteration)

			info := ResourceInfo{Exists: false}

			if fileInfo, err := os.Stat(path); err == nil {

				fileSize := fileInfo.Size()
				chunks := (fileSize + 1024*1024 - 1) / (1024 * 1024)
				lastChunkSize := fileSize % (1024 * 1024)
				if lastChunkSize == 0 {
					lastChunkSize = 1024 * 1024
				}

				info = ResourceInfo{
					Exists:        true,
					Size:          fileSize,
					Chunks:        int(chunks),
					LastChunkSize: lastChunkSize,
				}
			}

			metadata.Resources[ot][strconv.Itoa(iteration)] = info
		}
	}
	return metadata, nil
}

// HandleIncomingVideo processes the video file uploaded by the user and starts the processing pipeline.
//
// If a training config value is not provided, a default value is used.
//
// Returns the scene ID if successful, error otherwise.
func (s *ClientService) HandleIncomingVideo(
	ctx context.Context,
	userID primitive.ObjectID,
	file *multipart.FileHeader,
	trainingMode string,
	outputTypes []string,
	saveIterations []int,
	totalIterations int,
	sceneName string,
) (string, error) {
	// Validate video file
	if file == nil {
		return "", fmt.Errorf("file not received")
	}

	fileName := file.Filename
	if fileName == "" {
		return "", fmt.Errorf("file not received")
	}

	fileExt := filepath.Ext(fileName)
	if fileExt != ".mp4" {
		return "", fmt.Errorf("improper file extension")
	}

	sceneID := primitive.NewObjectID()

	// Save video to file storage
	videoName := sceneID.Hex() + ".mp4"
	videosFolder := "data/raw/videos"
	if err := os.MkdirAll(videosFolder, os.ModePerm); err != nil {
		return "", err
	}
	videoFilePath := filepath.Join(videosFolder, videoName)

	dst, err := os.Create(videoFilePath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return "", err
	}

	// Handle non-provided configuration values
	if sceneName == "" {
		sceneName = "Untitled Scene"
	}
	if trainingMode == "" {
		trainingMode = "gaussian"
	}
	if len(outputTypes) == 0 {
		outputTypes = []string{"video"}
	}
	if len(saveIterations) == 0 {
		saveIterations = []int{1000, 7000, 30000}
	}


	// Partially Initialize new scene
	newScene := &scene.Scene{
		ID: sceneID,
		Video: &scene.Video{
			FilePath: videoFilePath,
		},
		Config: &scene.TrainingConfig{
			NerfTrainingConfig: &scene.NerfTrainingConfig{
				TrainingMode:    trainingMode,
				OutputTypes:     outputTypes,
				SaveIterations:  saveIterations,
				TotalIterations: totalIterations,
			},
		},
		Name: sceneName,
	}

	// Insert scene into database
	if err := s.sceneManager.SetScene(ctx, sceneID, newScene); err != nil {
		s.logger.Errorf("Failed to insert new scene into database: %v", err)
		return "", err
	}

	// Start pipeline
	if err := s.mqService.PublishSFMJob(ctx, newScene); err != nil {
		s.logger.Errorf("Failed to publish SFM job: %v", err)
		return "", err
	}

	// Update user with new scene
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if err := user.AddScene(sceneID); err != nil {
		return "", err
	}
	if err := s.userManager.UpdateUser(ctx, user); err != nil {
		return "", err
	}

	return sceneID.Hex(), nil
}

// GetUserSceneHistory returns a list of scene IDS that the user has access to.
// It is tolerant of scenes that have been deleted / not finished processing by ignoring them.
//
// Returns a list of primitive.ObjectID's. Returns error if the user does not exist or non scene-existence errors occur.
func (s *ClientService) GetUserSceneHistory(ctx context.Context, userID primitive.ObjectID) ([]string, error) {
	s.logger.Debug("Get user history request received")

	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Info("Failed to get user history:", err.Error())
		return nil, err
	}

	resources := make([]string, 0)
	for _, sceneID := range user.SceneIDs {
		_, err := s.sceneManager.GetNerf(ctx, sceneID)

		// Ignore scenes that have been deleted / not finished processing
		if err == scene.ErrSceneNotFound || err == scene.ErrNerfNotFound {
			continue
		}
		if err != nil {
			s.logger.Info("Failed to get user history:", err.Error())
			return nil, err
		}

		resources = append(resources, sceneID.Hex())
	}

	s.logger.Info("User history retrieved successfully")
	return resources, nil
}

// GetSceneThumbnailPath returns the path to the thumbnail image for the given scene.
// Paths are relative to the main *.go executable.
//
// Internally, sfm frame data is used to determine the thumbnail path. THese are stored as http endpoints.
// So, a little bit of string manipulation is required.
//
// Returns ("", error) if the user does not have access to the scene or an error occurred.
func (s *ClientService) GetSceneThumbnailPath(ctx context.Context, userID, sceneID primitive.ObjectID) (string, error) {
	s.logger.Debug("Get scene thumbnail request received")

	// Verify user access to scene
	if err := s.verifyUserAccess(ctx, userID, sceneID); err != nil {
		s.logger.Info("Invalid user ID access:", err.Error())
		return "", err
	}

	sfm, err := s.sceneManager.GetSfm(ctx, sceneID)
	if err != nil {
		s.logger.Info("Invalid scene ID:", err.Error())
		return "", err
	}

	if len(sfm.Frames) == 0 {
		s.logger.Info("No frames found in SFM data")
		return "", fmt.Errorf("no frames found in SFM data")
	}

	// Use the first frame as the thumbnail
	thumbnailPath := sfm.Frames[0].FilePath

	if filepath.Ext(thumbnailPath) != ".png" {
		s.logger.Info("First frame is not a PNG file")
		return "", fmt.Errorf("first frame is not a PNG file")
	}

	// Convert API endpoint path to local file system path
	u, err := url.Parse(thumbnailPath)
	if err != nil {
		s.logger.Info("Invalid URL:", err.Error())
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	// Extract the path and remove "/worker-data" prefix if it exists
	localPath := strings.TrimPrefix(u.Path, "/worker-data/")

	// Ensure the path starts with "/data"
	if !strings.HasPrefix(localPath, "data") {
		s.logger.Info("Invalid path: does not start with data")
		return "", fmt.Errorf("invalid path: does not start with data")
	}

	s.logger.Info("Thumbnail retrieved successfully")
	return localPath, nil
}
// GetSceneName returns the name of the scene with the given ID.
//
// Returns (string) if scene valid. Returns ("", error) if the user does not have access to the scene or an error occurred.
func (s *ClientService) GetSceneName(ctx context.Context, userID, sceneID primitive.ObjectID) (string, error) {
	s.logger.Debug("Get scene name request received")

	// Verify user access to scene
	if err := s.verifyUserAccess(ctx, userID, sceneID); err != nil {
		s.logger.Info("Invalid user ID access:", err.Error())
		return "", err
	}

	sceneName, err := s.sceneManager.GetSceneName(ctx, sceneID)
	if err != nil {
		s.logger.Info("Error getting scene name:", err.Error())
		return "", err
	}

	s.logger.Info("Scene name retrieved successfully")
	return sceneName, nil
}

// GetSceneOutputPath returns the relative path to the output file for the given scene.
// Paths are relative to the main *.go executable.
//
// Returns (string) if successful. Returns ("", error) if the user does not have access to the scene or an error occurred.
func (s *ClientService) GetSceneOutputPath(ctx context.Context, userID, sceneID primitive.ObjectID, outputType, iteration string) (string, error) {
	s.logger.Debug("Get scene output request received")

	// Verify user access to scene
	if err := s.verifyUserAccess(ctx, userID, sceneID); err != nil {
		s.logger.Info("Invalid user ID access:", err.Error())
		return "", err
	}

	nerf, err := s.sceneManager.GetNerf(ctx, sceneID)
	if err != nil {
		s.logger.Info("Invalid scene ID:", err.Error())
		return "", err
	}

	intIteration := -1
	if iteration == "" {
		intIteration = -1
	} else {
		intIteration, err = strconv.Atoi(iteration)
		if err != nil {
			s.logger.Info("Invalid iteration:", err.Error())
			return "", err
		}
	}

	outputPath, err := nerf.GetFilePathForTypeAndIter(outputType, intIteration)
	if err != nil {
		s.logger.Info("Error getting output file:", err.Error())
		return "", err
	}

	return outputPath, nil
}

// GetSceneProgress returns the progress of the scene processing pipeline for the given scene.
// Returns (nil, error) if the user does not have access to the scene or an error occurred.
//
// This function trusts the ordering of queue names provivded by QueueListManager.
// The first queue is the overall progress, and the rest are the ordered training stages.
//
// Returns json with the following fields: {
//	    "processing": bool,
//	    "overall_position": int,
//	    "overall_size": int,
//	    "stage": string,
//	    "stage_position": int,
//	    "stage_size": int,
//	}
func (s *ClientService) GetSceneProgress(ctx context.Context, userID, sceneID primitive.ObjectID) (map[string]interface{}, error) {
	s.logger.Debug("Get scene progress handler")

	// Verify user access to scene
	if err := s.verifyUserAccess(ctx, userID, sceneID); err != nil {
		s.logger.Info("Invalid user ID access:", err.Error())
		return nil, err
	}

	var processing = false
	var overallPosition = -1
	var overallSize = -1
	var stagePosition = -1
	var stageSize = -1
	stageIdx := -1
	var err error

	queueNames := s.queueManager.GetQueueNames()
	s.logger.Debugf("Queue names: %v", queueNames)

	for idx, queueName := range queueNames {

		// Overall progress
		if idx == 0 {
			overallPosition, overallSize, err = s.queueManager.GetQueuePosition(ctx, queueName, sceneID)
			if err != nil && err != queue.ErrIDNotFoundInQueue {
				s.logger.Info("Error getting overall queue position:", err.Error())
				return nil, err
			}
			if err == queue.ErrIDNotFoundInQueue {
				processing = false
			} else {
				processing = true
			}
			continue
		}
		if !processing {
			break
		}

		// Training stages (sfm_list, nerf_list)
		size, position, err := s.queueManager.GetQueuePosition(ctx, queueName, sceneID)
		if err != nil && err != queue.ErrIDNotFoundInQueue {
			s.logger.Info("Error getting stage queue position:", err.Error())
			return nil, err
		}
		// ID found in stage
		if err == nil {
			stagePosition = position
			stageSize = size
			stageIdx = idx
			break
		}
	}

	s.logger.Debugf("Processing: %v, Overall position: %d, Overall size: %d, Stage Idx: %d, Stage position: %d, Stage size: %d", processing, overallPosition, overallSize, stageIdx, stagePosition, stageSize)

	if !processing{
		return map[string]interface{}{
			"processing": false,
		}, nil
	}

	return map[string]interface{}{
		"processing":       processing,
		"overall_position": overallPosition,
		"overall_size":     overallSize,
		"stage":            queueNames[stageIdx],
		"stage_position":   stagePosition,
		"stage_size":       stageSize,
	}, nil
}
