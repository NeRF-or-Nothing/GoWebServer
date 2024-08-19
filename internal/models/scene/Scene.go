// This file contains the Scene struct and its members, as well as some methods to check the validity of training modes and output types,
// converting integer keys to string keys for file paths, and getting file paths for output types.

// When interacting with MongoDB, bson tags are used to specify the field names in the database.
// For each struct field, you should add a bson tag with the field name in the database, to allow ease of serialization and deserialization.
// Optional fields should be marked with omitempty, saving memory; i.e if a scene fails during sfm, theres no need to store nerf data.

// When interfacting with workers (and consequently json-based ampq), json tags are used to specify the field names in the JSON payload.
// For each struct field, you should add a json tag with the field name in the JSON payload, to allow ease of marshalling and unmarshalling.

package scene

import (
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidOutputType = errors.New("invalid output type")
	ErrNoOutputPaths = errors.New("no output path found")
)


// Scene represents a scene and its components
type Scene struct {
    ID     primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
    Status int                `bson:"status" json:"status"`
    Video  *Video             `bson:"video,omitempty" json:"video,omitempty"`
    Sfm    *Sfm               `bson:"sfm,omitempty" json:"sfm,omitempty"`
    Nerf   *Nerf              `bson:"nerf,omitempty" json:"nerf,omitempty"`
    Config *TrainingConfig    `bson:"config,omitempty" json:"config,omitempty"`
}


// TrainingConfig represents the configuration for training
type TrainingConfig struct {
	SfmTrainingConfig  *SfmTrainingConfig  `bson:"sfm_training_config,omitempty" json:"sfm_training_config,omitempty"`
	NerfTrainingConfig *NerfTrainingConfig `bson:"nerf_training_config,omitempty" json:"nerf_training_config,omitempty"`
}

// NerfTrainingConfig represents the configuration for NeRF training
type NerfTrainingConfig struct {
	TrainingMode    string   `bson:"training_mode" json:"training_mode"`
	OutputTypes     []string `bson:"output_types" json:"output_types"`
	SaveIterations  []int    `bson:"save_iterations" json:"save_iterations"`
	TotalIterations int      `bson:"total_iterations" json:"total_iterations"`
}

// SfmTrainingConfig represents the configuration for SfM training
type SfmTrainingConfig struct {
	// Add fields as needed
}

// Frame represents a single frame in the SfM process
type Frame struct {
    FilePath        string      `bson:"file_path" json:"file_path"`
    ExtrinsicMatrix [][]float64 `bson:"extrinsic_matrix" json:"extrinsic_matrix"`
}

// Sfm represents the Structure from Motion data from Colmap worker.
type Sfm struct {
    IntrinsicMatrix [][]float64 `bson:"intrinsic_matrix" json:"intrinsic_matrix"`
    Frames          []Frame     `bson:"frames" json:"frames"`
    WhiteBackground bool        `bson:"white_background" json:"white_background"`
}
// Video represents video metadata
type Video struct {
    FilePath   string `bson:"file_path" json:"file_path"`
    Width      int    `bson:"width" json:"width"`
    Height     int    `bson:"height" json:"height"`
    FPS        int    `bson:"fps" json:"fps"`
    Duration   int    `bson:"duration" json:"duration"`
    FrameCount int    `bson:"frame_count" json:"frame_count"`
}

// Nerf represents the finished nerf training. Int Keys should be strictly greater than 0.
type Nerf struct {
    ModelFilePathsMap      map[int]string `bson:"model_file_paths,omitempty" json:"model_file_paths,omitempty"`
    SplatCloudFilePathsMap map[int]string `bson:"splat_cloud_file_paths,omitempty" json:"splat_cloud_file_paths,omitempty"`
    PointCloudFilePathsMap map[int]string `bson:"point_cloud_file_paths,omitempty" json:"point_cloud_file_paths,omitempty"`
    VideoFilePathsMap      map[int]string `bson:"video_file_paths,omitempty" json:"video_file_paths,omitempty"`
    Flag                   int            `bson:"flag" json:"flag"`
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

// GetFilePathsForTypeAndIter returns the file path for a single given output type.
// Iteration is the key in the file paths map, and should be > 0. If iteration is -1, the last iteration is returned.
func (n *Nerf) GetFilePathForTypeAndIter(outputType string, iteration int) (string, error) {
	var filePathsMap map[int]string
	
	switch outputType {
	case "model":
		filePathsMap = n.ModelFilePathsMap
	case "splat_cloud":
		filePathsMap = n.SplatCloudFilePathsMap
	case "point_cloud":
		filePathsMap = n.PointCloudFilePathsMap
	case "video":
		filePathsMap = n.VideoFilePathsMap
	default:
		return "", ErrInvalidOutputType
	}

	if iteration == -1 {
		iteration = getMaxKey(filePathsMap)
	}

	filePath, ok := filePathsMap[iteration]
	if !ok {
		return "", ErrNoOutputPaths
	}
	return filePath, nil
}

// GetMaxKey returns the maximum key in a map with positive integer keys.
// Internally used to get the last iteration for a given output type.
func getMaxKey(m map[int]string) int {
	max := 0
	for k := range m {
		if k > max {
			max = k
		}
	}
	return max
}
