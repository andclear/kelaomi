package main

import (
	"fmt"
	"log"
	"os"

	"atlassian/auth"
	"atlassian/db"
)

func main() {
	_, err := db.InitDB()
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	var adminPasswordErr error
	_, _, adminPasswordErr = db.GetAdminPassword()
	if adminPasswordErr != nil {
		initialPassword := db.GenerateRandomPassword(12)
		hashedPassword := auth.HashPassword(initialPassword)
		err = db.SetAdminPassword(hashedPassword, true)
		if err != nil {
			log.Fatalf("设置初始密码失败: %v", err)
		}
		IsFirstRun = true
		fmt.Printf("\n🔐 初始管理员密码: %s\n", initialPassword)
		fmt.Printf("请在首次登录后立即修改此密码\n\n")
	}

	// 从数据库加载凭据
	LoadCredentials()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	router := SetupRoutes()

	fmt.Printf("🚀 OpenAI‑Compatible Proxy via Atlassian AI Gateway\n")
	fmt.Printf("📡 Server starting on port %s\n", port)
	fmt.Printf("🔗 Base URL: http://localhost:%s/v1\n", port)
	fmt.Printf("📋 Endpoints:\n")
	fmt.Printf("   • GET  /v1/models\n")
	fmt.Printf("   • POST /v1/chat/completions\n")
	fmt.Printf("   • GET  /health\n")
	fmt.Printf("🔐 Configured with %d credential(s)\n", len(Credentials))

	if DebugMode {
		fmt.Printf("🐛 Debug mode: ENABLED\n")
	}

	fmt.Printf("\n")

	address := fmt.Sprintf(":%s", port)
	log.Printf("Server listening on %s", address)

	if err := router.Run(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
