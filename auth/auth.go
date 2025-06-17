package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// JWT secret key, should be read from environment variables or config file
	jwtSecret = []byte("atlassian_proxy_jwt_secret")

	// JWT expiration time
	tokenExpiration = 24 * time.Hour
)

// Claims custom JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID uint `json:"user_id"`
}

// GenerateToken generates a JWT token
func GenerateToken(userID uint) (string, error) {
	// Create claims
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	return token.SignedString(jwtSecret)
}

// ParseToken parses a JWT token
func ParseToken(tokenString string) (*Claims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	// Validate token
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// HashPassword hashes a password
func HashPassword(password string) string {
	// Use SHA-256 to hash the password
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// VerifyPassword verifies a password
func VerifyPassword(hashedPassword, password string) bool {
	return hashedPassword == HashPassword(password)
}
