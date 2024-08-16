package services

import (
	"context"
	"mime/multipart"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/scene"
	"github.com/NeRF-or-Nothing/VidGoNerf/webserver/internal/models/user"
)

type ClientService struct {
    mqService    *AMPQService
    sceneManager  *scene.SceneManager
    userManager   *user.UserManager
}

func NewClientService(sceneManager *scene.SceneManager, mqService *AMPQService, userManager *user.UserManager) *ClientService {
    return &ClientService{
        mqService:   mqService,
        sceneManager: sceneManager,
        userManager:  userManager,
    }
}

func (s *ClientService) verifyUserAccess(userID primitive.ObjectID, jobID string) error {
    ctx := context.TODO()
    authorized, err := s.userManager.UserHasJobAccess(ctx, userID, jobID)
    if err != nil {
        return err
    }
    if !authorized {
        return user.ErrUserNoAccess
    }
    return nil
}

func (s *ClientService) GetNerfTypeMetadata(userID, uuid, outputType string) {
    return nil
}

func (s *ClientService) GetNerfMetadata(userID, uuid string) {
    return nil
}


func (s *ClientService) HandleIncomingVideo(userID string, videoFile *multipart.FileHeader, requestParams map[string]string, sceneName string) (string, error) {
    return "nil", nil
}

func (s *ClientService) GetNerfResource(userID, uuid, resourceType, iteration, rangeHeader string)  {
    return nil
}

func (s *ClientService) GetUserHistory(userID string) {
    return nil
}

func (s *ClientService) GetPreview(userID, uuid string)  {
    return nil
}

func (s *ClientService) LoginUser(username, password string) (string, error) {
    ctx := context.TODO()
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

func (s *ClientService) RegisterUser(username, password string) error {
    return nil
}
