package dbschema

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID                primitive.ObjectID   `bson:"_id,omitempty"`
	Username          string               `bson:"username"`
	EncryptedPassword string               `bson:"encrypted_password"`
	SceneIDs          []primitive.ObjectID `bson:"scene_ids"`
}

// AddScene adds a scene ID to the user's list of scenes
func (u *User) AddScene(sceneID primitive.ObjectID) {
	u.SceneIDs = append(u.SceneIDs, sceneID)
}

// SetPassword sets a new password for the user. Encrypts the password using bcrypt.
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