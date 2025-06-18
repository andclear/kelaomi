package db

import (
	"errors"
	"github.com/jackc/pgconn" // For PostgreSQL error codes
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"gorm.io/driver/sqlite" // Added for local dev fallback
	"path/filepath"
	"sync"
	"time"

	"gorm.io/driver/postgres" // Changed from sqlite
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

// InitDB initializes the database connection
func InitDB() (*gorm.DB, error) {
	var err error
	dbOnce.Do(func() {
		// Get database connection string from environment variable
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Println("DATABASE_URL environment variable not set. Using default SQLite for local development.")
			// Fallback to SQLite for local development if DATABASE_URL is not set
			dbPath := "./credentials_dev.db" // Local dev database file
			config := &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			}
			db, err = gorm.Open(sqlite.Open(dbPath), config) // Keep sqlite for fallback
			if err != nil {
				log.Printf("Failed to connect to local SQLite database: %v", err)
				return
			}
		} else {
			// Configure GORM for PostgreSQL
			config := &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			}

			// Connect to PostgreSQL database
			db, err = gorm.Open(postgres.Open(dsn), config)
			if err != nil {
				log.Printf("Failed to connect to PostgreSQL database: %v", err)
				return
			}
		}

		// Auto migrate table structure
		err = db.AutoMigrate(&Credential{}, &APIToken{}, &AdminPassword{})
		if err != nil {
			log.Printf("AutoMigrate returned error: %v (Type: %T)", err, err)
			var pgErr *pgconn.PgError
			foundPgError42P07 := false

			// Attempt direct assertion first, as %T suggests it might be the direct type
			if errors.As(err, &pgErr) {
				log.Printf("Direct assertion to *pgconn.PgError successful. Code from pgErr.Code: [%s]", pgErr.Code)
				if pgErr.Code == "42P07" {
					foundPgError42P07 = true
				} else {
					 log.Printf("Directly asserted to PgError, but code is [%s], not \"42P07\".", pgErr.Code)
				}
			} else {
				log.Printf("Direct assertion to *pgconn.PgError FAILED, even though type was reported as %T. Proceeding to check error chain.", err)
				// If direct assertion fails, check the chain (this path should ideally not be taken given the %T log)
				currentErr := err
				depth := 0
				for currentErr != nil {
					log.Printf("Checking error in chain (depth %d): %v (Type: %T)", depth, currentErr, currentErr)
					if errors.As(currentErr, &pgErr) {
						log.Printf("Chain (depth %d): Successfully asserted to *pgconn.PgError. Code from pgErr.Code: [%s]", depth, pgErr.Code)
						if pgErr.Code == "42P07" {
							log.Printf("Chain (depth %d): PgError code IS \"42P07\".", depth)
							foundPgError42P07 = true
							break
						} else {
							log.Printf("Chain (depth %d): PgError code is [%s], not \"42P07\".", depth, pgErr.Code)
						}
					} else {
						 log.Printf("Chain (depth %d): Failed to assert current error in chain to *pgconn.PgError.", depth)
					}
					prevErr := currentErr
					currentErr = errors.Unwrap(currentErr)
					if currentErr == prevErr && currentErr != nil { // Avoid infinite loop if Unwrap returns itself and is not nil
						log.Printf("Chain (depth %d): errors.Unwrap returned the same error, breaking loop to prevent infinite recursion.", depth)
						break
					}
					depth++
				}
			}

			if foundPgError42P07 {
				log.Println("Tables already exist (PgError 42P07 detected), skipping migration.")
				err = nil // Clear the error for the outer scope
			} else {
				log.Printf("Failed to migrate table structure (PgError 42P07 not found after checks): %v", err)
				return
			}
		}
	})

	return db, err
}

// ensureDBDir is no longer needed for PostgreSQL, but kept for SQLite fallback
func ensureDBDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	return os.MkdirAll(dir, 0755)
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
