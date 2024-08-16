package services

import (
    "mime/multipart"
    "github.com/NeRF-Or-Nothing/VidGoNerf/webserver/internal/models"
    "github.com/NeRF-Or-Nothing/VidGoNerf/webserver/internal/services"
)

type ClientService struct {
    sceneManager  *models.SceneManager
    rmqService    *services.AMPQService
    userManager   *models.UserManager
}

func NewClientService(sceneManager *models.SceneManager, rmqService *AMPQService, userManager *models.UserManager) *ClientService {
    return &ClientService{
        sceneManager: sceneManager,
        rmqService:   rmqService,
        userManager:  userManager,
    }
}

func (s *ClientService) VerifyUserAccess(userID, jobID string) error {
    if !s.userManager.UserHasJobAccess(userID, jobID) {
        return models.NewErrorResponse(models.Unauthorized, "User does not have access to this resource")
    }
    return nil
}

func (s *ClientService) GetNerfMetadata(userID, uuid string) *models.Response {
}

func (s *ClientService) GetNerfTypeMetadata(userID, uuid, outputType string) *models.Response {

}

func (s *ClientService) HandleIncomingVideo(userID string, videoFile *multipart.FileHeader, requestParams map[string]string, sceneName string) (string, error) {

}

func (s *ClientService) GetNerfResource(userID, uuid, resourceType, iteration, rangeHeader string) *models.FileResponse {

}

func (s *ClientService) GetUserHistory(userID string) *models.Response {

}

func (s *ClientService) GetPreview(userID, uuid string) *models.FileResponse {

}
