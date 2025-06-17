package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"atlassian/auth"
	"atlassian/db"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
)

// SetupRoutes configures the HTTP routes
func SetupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// OpenAI compatible endpoints
	v1 := r.Group("/v1")
	{
		v1.GET("/models", ListModels)
		v1.POST("/chat/completions", ChatCompletions)
	}

	// Admin page routes
	admin := r.Group("/admin")
	{
		// Login page
		admin.GET("/login", ShowLoginPage)
		admin.POST("/login", HandleLogin)

		// Routes requiring authentication
		authorized := admin.Group("/")
		authorized.Use(AuthMiddleware())
		{
			// Credential management page
			authorized.GET("/credentials", ShowCredentialsPage)
			authorized.POST("/credentials", AddCredential)
			authorized.POST("/credentials/delete/:id", DeleteCredential)
			authorized.GET("/credentials/reload", ReloadCredentialsHandler)

			// API token management
			authorized.POST("/apitoken/generate", GenerateAPITokenHandler)

			// Password management
			authorized.GET("/change-password", ShowChangePasswordPage)
			authorized.POST("/change-password", ChangePassword)
			authorized.GET("/reset-password", ShowResetPasswordPage)
			authorized.POST("/reset-password", ResetPassword)
		}
	}

	// Load embedded HTML templates
	templ := template.Must(template.New("").ParseFS(GetTemplatesFS(), "templates/*.html"))
	r.SetHTMLTemplate(templ)

	// Load embedded static files
	r.StaticFS("/static", GetStaticFS())

	return r
}

// AuthMiddleware authentication middleware
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get JWT token from cookie
		tokenString, err := c.Cookie("admin_jwt")
		if err != nil {
			// Not authenticated, redirect to login page
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}

		// Validate JWT token
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			// Invalid token, clear cookie and redirect to login page
			c.SetCookie("admin_jwt", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}

		// Check if initial password needs to be changed
		isInitial, err := db.IsPasswordInitial()
		if err == nil && isInitial {
			// If current path is not change password page, redirect to change password page
			if c.Request.URL.Path != "/admin/change-password" {
				c.Redirect(http.StatusFound, "/admin/change-password")
				c.Abort()
				return
			}
		}

		// Authentication passed, continue processing request
		c.Set("userID", claims.UserID)
		c.Next()
	}
}

// ShowLoginPage displays the login page
func ShowLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Admin Login",
	})
}

// HandleLogin processes login requests
func HandleLogin(c *gin.Context) {
	password := c.PostForm("password")

	// Get stored password hash
	storedHash, isInitial, err := db.GetAdminPassword()
	fmt.Println(isInitial)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to get password: " + err.Error(),
		})
		return
	}

	// Verify password
	if auth.VerifyPassword(storedHash, password) {
		// Generate JWT token
		token, err := auth.GenerateToken(1) // Use fixed user ID
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error": "Failed to generate token: " + err.Error(),
			})
			return
		}

		// Set JWT cookie
		c.SetCookie("admin_jwt", token, 3600, "/", "", false, true)

		// If initial password, redirect to change password page
		if isInitial {
			c.Redirect(http.StatusFound, "/admin/change-password")
		} else {
			c.Redirect(http.StatusFound, "/admin/credentials")
		}
	} else {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title": "Admin Login",
			"error": "Incorrect password",
		})
	}
}

// ShowCredentialsPage displays the credentials management page
func ShowCredentialsPage(c *gin.Context) {
	// Get all credentials from database
	credentials, err := db.GetAllCredentials()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to get credentials: " + err.Error(),
		})
		return
	}

	// Get API token
	apiToken, _ := db.GetAPIToken()

	c.HTML(http.StatusOK, "credentials.html", gin.H{
		"title":       "Credential Management",
		"credentials": credentials,
		"apiToken":    apiToken,
	})
}

// AddCredential adds a new credential
func AddCredential(c *gin.Context) {
	email := c.PostForm("email")
	token := c.PostForm("token")

	// Validate input
	if email == "" || token == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Email and token cannot be empty",
		})
		return
	}

	// Add to database
	err := db.AddCredential(email, token)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to add credential: " + err.Error(),
		})
		return
	}

	// Reload credentials
	ReloadCredentials()

	// Redirect back to credentials page
	c.Redirect(http.StatusFound, "/admin/credentials")
}

// DeleteCredential deletes a credential
func DeleteCredential(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Invalid ID",
		})
		return
	}

	// Delete from database
	err = db.DeleteCredential(uint(id))
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to delete credential: " + err.Error(),
		})
		return
	}

	// Reload credentials
	ReloadCredentials()

	// Redirect back to credentials page
	c.Redirect(http.StatusFound, "/admin/credentials")
}

// ReloadCredentialsHandler reloads credentials
func ReloadCredentialsHandler(c *gin.Context) {
	ReloadCredentials()
	c.Redirect(http.StatusFound, "/admin/credentials")
}

// GenerateAPITokenHandler generates a new API token
func GenerateAPITokenHandler(c *gin.Context) {
	_, err := db.GenerateAPIToken()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to generate API token: " + err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/credentials")
}

// ShowChangePasswordPage displays the change password page
func ShowChangePasswordPage(c *gin.Context) {
	// Check if it's the initial password
	isInitial, _ := db.IsPasswordInitial()

	c.HTML(http.StatusOK, "change_password.html", gin.H{
		"title":     "Change Password",
		"isInitial": isInitial,
	})
}

// ChangePassword handles password change requests
func ChangePassword(c *gin.Context) {
	// Get form data
	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate new password
	if newPassword == "" {
		c.HTML(http.StatusBadRequest, "change_password.html", gin.H{
			"title": "Change Password",
			"error": "New password cannot be empty",
		})
		return
	}

	if newPassword != confirmPassword {
		c.HTML(http.StatusBadRequest, "change_password.html", gin.H{
			"title": "Change Password",
			"error": "Passwords do not match",
		})
		return
	}

	// Get stored password hash
	storedHash, _, err := db.GetAdminPassword()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to get password: " + err.Error(),
		})
		return
	}

	// Verify current password
	if !auth.VerifyPassword(storedHash, currentPassword) {
		c.HTML(http.StatusBadRequest, "change_password.html", gin.H{
			"title": "Change Password",
			"error": "Current password is incorrect",
		})
		return
	}

	// Update password
	newHash := auth.HashPassword(newPassword)
	err = db.SetAdminPassword(newHash, false)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to update password: " + err.Error(),
		})
		return
	}

	// Clear JWT cookie, force re-login
	c.SetCookie("admin_jwt", "", -1, "/", "", false, true)

	// Redirect to login page
	c.Redirect(http.StatusFound, "/admin/login?message=Password updated, please login again")
}

// ShowResetPasswordPage displays the reset password page
func ShowResetPasswordPage(c *gin.Context) {
	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"title": "Reset Password",
	})
}

// ResetPassword handles password reset requests
func ResetPassword(c *gin.Context) {
	// Generate new random password
	newPassword := db.GenerateRandomPassword(12)
	newHash := auth.HashPassword(newPassword)

	// Update password
	err := db.SetAdminPassword(newHash, true)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to reset password: " + err.Error(),
		})
		return
	}

	// Clear JWT cookie, force re-login
	c.SetCookie("admin_jwt", "", -1, "/", "", false, true)

	// Show new password
	c.HTML(http.StatusOK, "password_reset_success.html", gin.H{
		"title":    "Password Reset",
		"password": newPassword,
	})
}

// ListModels handles GET /v1/models
func ListModels(c *gin.Context) {
	now := time.Now().Unix()

	models := make([]Model, len(SupportedModels))
	for i, modelID := range SupportedModels {
		models[i] = Model{
			ID:      modelID,
			Object:  "model",
			Created: now,
			OwnedBy: "system",
		}
	}

	response := ModelsResponse{
		Object: "list",
		Data:   models,
	}

	c.JSON(http.StatusOK, response)
}

// ChatCompletions handles POST /v1/chat/completions
func ChatCompletions(c *gin.Context) {
	// Validate API token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
		return
	}

	// Extract token
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key format"})
		return
	}

	apiToken := tokenParts[1]
	if !db.ValidateAPIToken(apiToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return
	}

	var req ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate required fields
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model is required"})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Messages are required"})
		return
	}

	request := req.ToOpenAIRequest()

	// Create Atlassian request
	atlassianReq := AtlassianRequest{
		RequestPayload: AtlassianRequestPayload{
			Messages:    request.Messages,
			Temperature: req.Temperature,
			Stream:      req.Stream,
		},
		PlatformAttributes: AtlassianPlatformAttrs{
			Model: TransformModelID(req.Model),
		},
	}

	// Create HTTP client
	client := NewHTTPClient()
	ctx := c.Request.Context()

	// Make request with retry
	resp, err := client.FetchWithRetry(ctx, atlassianReq, req.Stream)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "All credentials exhausted"})
		return
	}

	// Handle streaming response
	if req.Stream {
		handleStreamingResponse(c, resp, req.Model)
		return
	}

	// Handle non-streaming response
	handleNonStreamingResponse(c, resp, req.Model)
}

// handleStreamingResponse processes streaming chat completion
func handleStreamingResponse(c *gin.Context, resp *resty.Response, requestedModel string) {
	// Set streaming headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Create stream response
	streamResp := &StreamResponse{
		Response: resp,
		Model:    requestedModel,
	}

	ctx := c.Request.Context()
	dataChan, errChan := streamResp.ConvertToOpenAIStream(ctx)

	// Stream data to client
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				return
			}
			c.Writer.Write(data)
			flusher.Flush()
		case err := <-errChan:
			if err != nil && err != context.Canceled {
				c.Writer.Write([]byte("data: {\"error\":\"" + err.Error() + "\"}\n\n"))
				flusher.Flush()
			}
			return
		case <-ctx.Done():
			return
		}
	}
}

// handleNonStreamingResponse processes non-streaming chat completion
func handleNonStreamingResponse(c *gin.Context, resp *resty.Response, requestedModel string) {
	var atlassianResp AtlassianResponse
	if err := json.Unmarshal(resp.Body(), &atlassianResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse upstream response"})
		return
	}

	// Convert to OpenAI format
	openaiResp := ToOpenAI(atlassianResp, requestedModel)
	c.JSON(http.StatusOK, openaiResp)
}
