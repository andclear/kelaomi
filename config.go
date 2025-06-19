package main

import (
	"log"
	"time"

	"atlassian/db"
)

// Configuration & constants
const (
	// Debug mode for verbose logging
	DebugMode = true

	// Upstream Atlassian AI Gateway
	RovoDevProxyURL      = "https://api.atlassian.com/rovodev/v2/proxy/ai"
	UnifiedChatPath      = "/v2/beta/chat"
	AtlassianAPIEndpoint = RovoDevProxyURL + UnifiedChatPath

	// Retry configuration
	InitialDelay    = 500 * time.Millisecond
	MaxDelay        = 16 * time.Second
	DelayMultiplier = 2
)

// Supported model list returned to clients (with prefixes visible)
var SupportedModels = []string{
	"anthropic:claude-3-5-sonnet-v2@20241022",
	"anthropic:claude-3-7-sonnet@20250219",
	"anthropic:claude-sonnet-4@20250514",
}

// Credential represents an email/token pair
type Credential struct {
	Email string
	Token string
}

var Credentials []Credential

var IsFirstRun = true

// LoadCredentials loads credentials from database
func LoadCredentials() {
	dbCredentials, err := db.GetAllCredentials()
	if err != nil {
		log.Printf("Failed to load credentials from database: %v", err)

		Credentials = []Credential{}
		return
	}

	Credentials = make([]Credential, 0, len(dbCredentials))
	for _, cred := range dbCredentials {
		Credentials = append(Credentials, Credential{
			Email: cred.Email,
			Token: cred.Token,
		})
	}

	log.Printf("Loaded %d credentials from database", len(Credentials))
}

func ReloadCredentials() {
	LoadCredentials()
}
