package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Credential represents the credential model in the database
type Credential struct {
	ID    uint   `gorm:"primarykey"`
	Email string `gorm:"uniqueIndex;not null"`
	Token string `gorm:"not null"`
}

// APIToken represents an API access token
type APIToken struct {
	ID        uint   `gorm:"primarykey"`
	Token     string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
}

// AdminPassword represents the admin password
type AdminPassword struct {
	ID           uint   `gorm:"primarykey"`
	PasswordHash string `gorm:"not null"`
	IsInitial    *bool  `gorm:"default:true"` // Whether it's the initial password
	CreatedAt    time.Time
}

var (
	db     *gorm.DB
	dbOnce sync.Once
)

// Ensure database directory exists
func ensureDBDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	return os.MkdirAll(dir, 0755)
}

// InitDB initializes the database connection
func InitDB() (*gorm.DB, error) {
	var err error
	dbOnce.Do(func() {
		// Database file path
		// 将数据库文件路径设置在 /data 卷中，以便持久化
		dbPath := "/data/credentials.db"

		// Ensure database directory exists
		if err = ensureDBDir(dbPath); err != nil {
			log.Printf("Failed to create database directory: %v", err)
			return
		}

		// Configure GORM
		config := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		}

		// Connect to database
		db, err = gorm.Open(sqlite.Open(dbPath), config)
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
			return
		}

		// Auto migrate table structure
		err = db.AutoMigrate(&Credential{}, &APIToken{}, &AdminPassword{})
		if err != nil {
			log.Printf("Failed to migrate table structure: %v", err)
			return
		}
	})

	return db, err
}

// GetDB returns the database connection
func GetDB() *gorm.DB {
	if db == nil {
		var err error
		db, err = InitDB()
		if err != nil {
			log.Fatalf("Failed to get database connection: %v", err)
		}
	}
	return db
}

// GetAllCredentials gets all credentials
func GetAllCredentials() ([]Credential, error) {
	var credentials []Credential
	result := GetDB().Find(&credentials)
	return credentials, result.Error
}

// AddCredential adds a new credential
func AddCredential(email, token string) error {
	credential := Credential{
		Email: email,
		Token: token,
	}
	result := GetDB().Create(&credential)
	return result.Error
}

// DeleteCredential deletes a credential
func DeleteCredential(id uint) error {
	result := GetDB().Delete(&Credential{}, id)
	return result.Error
}

// GetCredentialByID gets a credential by ID
func GetCredentialByID(id uint) (Credential, error) {
	var credential Credential
	result := GetDB().First(&credential, id)
	return credential, result.Error
}

// UpdateCredential updates a credential
func UpdateCredential(id uint, email, token string) error {
	result := GetDB().Model(&Credential{}).Where("id = ?", id).Updates(map[string]interface{}{
		"email": email,
		"token": token,
	})
	return result.Error
}

// GetAPIToken gets the API token
func GetAPIToken() (string, error) {
	var token APIToken
	result := GetDB().First(&token)
	if result.Error != nil {
		return "", result.Error
	}
	return token.Token, nil
}

// GenerateAPIToken generates a new API token
func GenerateAPIToken() (string, error) {
	// Generate random token
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	token := fmt.Sprintf("sk-%s", hex.EncodeToString(b))

	// Delete all existing tokens
	GetDB().Where("1=1").Delete(&APIToken{})

	// Create new token
	apiToken := APIToken{
		Token:     token,
		CreatedAt: time.Now(),
	}
	result := GetDB().Create(&apiToken)
	if result.Error != nil {
		return "", result.Error
	}

	return token, nil
}

// ValidateAPIToken validates an API token
func ValidateAPIToken(token string) bool {
	var count int64
	GetDB().Model(&APIToken{}).Where("token = ?", token).Count(&count)
	return count > 0
}

// SetAdminPassword sets the admin password
func SetAdminPassword(passwordHash string, isInitial bool) error {
	// Delete all existing passwords
	GetDB().Where("1=1").Delete(&AdminPassword{})

	// Create new password
	adminPassword := AdminPassword{
		PasswordHash: passwordHash,
		IsInitial:    &isInitial,
		CreatedAt:    time.Now(),
	}
	result := GetDB().Create(&adminPassword)
	return result.Error
}

// GetAdminPassword gets the admin password
func GetAdminPassword() (string, bool, error) {
	var adminPassword AdminPassword
	result := GetDB().First(&adminPassword)
	if result.Error != nil {
		return "", false, result.Error
	}
	return adminPassword.PasswordHash, *adminPassword.IsInitial, nil
}

// IsPasswordInitial checks if the current password is the initial password
func IsPasswordInitial() (bool, error) {
	var adminPassword AdminPassword
	result := GetDB().First(&adminPassword)
	fmt.Printf("adminPassword: %v\n", adminPassword)
	if result.Error != nil {
		return true, result.Error
	}
	return *adminPassword.IsInitial, nil
}

// GenerateRandomPassword generates a random password
func GenerateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+"
	b := make([]byte, length)
	for i := range b {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[randomIndex.Int64()]
	}
	return string(b)
}
