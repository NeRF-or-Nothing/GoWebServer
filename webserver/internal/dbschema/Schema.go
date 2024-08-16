package dbschema

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

// Nerf represents the finished nerf training
type Nerf struct {
	ModelFilePathsMap      map[int]string `bson:"model_file_paths,omitempty"`
	SplatCloudFilePathsMap map[int]string `bson:"splat_cloud_file_paths,omitempty"`
	PointCloudFilePathsMap map[int]string `bson:"point_cloud_file_paths,omitempty"`
	VideoFilePathsMap      map[int]string `bson:"video_file_paths,omitempty"`
	Flag                   int            `bson:"flag"`
}

// Frame represents a single frame in the SfM process
type Frame struct {
	FilePath        string      `bson:"file_path"`
	ExtrinsicMatrix [][]float64 `bson:"extrinsic_matrix"`
}

// Sfm represents the Structure from Motion data from Colmap worker.
type Sfm struct {
	IntrinsicMatrix [][]float64 `bson:"intrinsic_matrix"`
	Frames          []Frame     `bson:"frames"`
	WhiteBackground bool        `bson:"white_background"`
}

// Video represents video metadata
type Video struct {
	FilePath   string `bson:"file_path"`
	Width      int    `bson:"width"`
	Height     int    `bson:"height"`
	FPS        int    `bson:"fps"`
	Duration   int    `bson:"duration"`
	FrameCount int    `bson:"frame_count"`
}

// TrainingConfig represents the configuration for training
type TrainingConfig struct {
	SfmConfig  map[string]interface{} `bson:"sfm_config"`
	NerfConfig map[string]interface{} `bson:"nerf_config"`
}

// Scene represents a complete scene with all its components
type Scene struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Status int                `bson:"status"`
	Video  *Video             `bson:"video,omitempty"`
	Sfm    *Sfm               `bson:"sfm,omitempty"`
	Nerf   *Nerf              `bson:"nerf,omitempty"`
	Config *TrainingConfig    `bson:"config,omitempty"`
}

// User represents a user in the system
type User struct {
	ID                primitive.ObjectID   `bson:"_id,omitempty"`
	Username          string               `bson:"username"`
	EncryptedPassword string               `bson:"encrypted_password"`
	SceneIDs          []primitive.ObjectID `bson:"scene_ids"`
}

// SetPassword sets a new password for the user
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.EncryptedPassword = string(hashedPassword)
	return nil
}

// CheckPassword verifies if the provided password is correct
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.EncryptedPassword), []byte(password))
	return err == nil
}

// AddScene adds a scene ID to the user's list of scenes
func (u *User) AddScene(sceneID primitive.ObjectID) {
	u.SceneIDs = append(u.SceneIDs, sceneID)
}

// Constants for valid training modes and output types
const (
	TrainingModeGaussian = "gaussian"
	TrainingModeTensorf  = "tensorf"
)

var (
	ValidTrainingModes = []string{TrainingModeGaussian, TrainingModeTensorf}
	ValidOutputTypes   = map[string][]string{
		TrainingModeGaussian: {"splat_cloud", "point_cloud", "video"},
		TrainingModeTensorf:  {"model", "video"},
	}
)

// IsValidTrainingMode checks if the given training mode is valid
func IsValidTrainingMode(mode string) bool {
	for _, validMode := range ValidTrainingModes {
		if mode == validMode {
			return true
		}
	}
	return false
}

// IsValidOutputType checks if the given output type is valid for the specified training mode
func IsValidOutputType(trainingMode, outputType string) bool {
	validTypes, ok := ValidOutputTypes[trainingMode]
	if !ok {
		return false
	}
	for _, validType := range validTypes {
		if outputType == validType {
			return true
		}
	}
	return false
}
