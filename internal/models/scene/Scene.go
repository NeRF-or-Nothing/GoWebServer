// This file contains the Scene struct and its members, as well as some methods to check the validity of training modes and output types,
// converting integer keys to string keys for file paths, and getting file paths for output types.

// When interacting with MongoDB, bson tags are used to specify the field names in the database.
// For each struct field, you should add a bson tag with the field name in the database, to allow ease of serialization and deserialization.
// Optional fields should be marked with omitempty, saving memory; i.e if a scene fails during sfm, theres no need to store nerf data.

package scene

import (
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TrainingConfig represents the configuration for training
type TrainingConfig struct {
	SfmTrainingConfig  *SfmTrainingConfig  `bson:"sfm_training_config,omitempty"`
	NerfTrainingConfig *NerfTrainingConfig `bson:"nerf_training_config,omitempty"`
}

// NerfTrainingConfig represents the configuration for NeRF training
type NerfTrainingConfig struct {
	TrainingMode    string   `bson:"training_mode"`
	OutputTypes     []string `bson:"output_types"`
	SaveIterations  []int    `bson:"save_iterations"`
	TotalIterations int      `bson:"total_iterations"`
}

// SfmTrainingConfig represents the configuration for SfM training
type SfmTrainingConfig struct {
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

// Scene represents a complete scene with all its components
type Scene struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	Status int                `bson:"status"`
	Video  *Video             `bson:"video,omitempty"`
	Sfm    *Sfm               `bson:"sfm,omitempty"`
	Nerf   *Nerf              `bson:"nerf,omitempty"`
	Config *TrainingConfig    `bson:"config,omitempty"`
}

// Nerf represents the finished nerf training
type Nerf struct {
	ModelFilePathsMap      map[int]string `bson:"model_file_paths,omitempty"`
	SplatCloudFilePathsMap map[int]string `bson:"splat_cloud_file_paths,omitempty"`
	PointCloudFilePathsMap map[int]string `bson:"point_cloud_file_paths,omitempty"`
	VideoFilePathsMap      map[int]string `bson:"video_file_paths,omitempty"`
	Flag                   int            `bson:"flag"`
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
func (Nerf) IsValidTrainingMode(mode string) bool {
	for _, validMode := range ValidTrainingModes {
		if mode == validMode {
			return true
		}
	}
	return false
}

// IsValidOutputType checks if the given output type is valid for the specified training mode
func (Nerf) IsValidOutputType(trainingMode, outputType string) bool {
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

// ConvertIntKeysToString converts a map with integer keys to a map with string keys.
// Used to convert the file paths map to a map that can be returned as JSON.
func ConvertIntKeysToString(m map[int]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[strconv.Itoa(k)] = v
	}
	return result
}

// GetFilePathsForOutputType returns the file paths for a single given output type.\
func (n *Nerf) GetFilePathsForOutputType(outputType string) map[string]string {
	switch outputType {
	case "model":
		return ConvertIntKeysToString(n.ModelFilePathsMap)
	case "splat_cloud":
		return ConvertIntKeysToString(n.SplatCloudFilePathsMap)
	case "point_cloud":
		return ConvertIntKeysToString(n.PointCloudFilePathsMap)
	case "video":
		return ConvertIntKeysToString(n.VideoFilePathsMap)
	default:
		return make(map[string]string)
	}
}
