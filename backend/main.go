package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

type Config struct {
	Port        string
	BotToken    string
	BotUsername string
	JWTSecret   string
	AdminIDs    []string
	FrontendURL string
}

type App struct {
	config *Config
}

type TelegramUser struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	PhotoURL  string `json:"photo_url,omitempty"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

type AuthResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Role    string `json:"role,omitempty"`
	Message string `json:"message,omitempty"`
}

type Claims struct {
	UserID   int    `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	adminIDs := strings.Split(os.Getenv("ADMIN_IDS"), ",")
	if len(adminIDs) == 1 && adminIDs[0] == "" {
		adminIDs = []string{}
	}

	config := &Config{
		Port:        getEnv("PORT", "8081"),
		BotToken:    os.Getenv("BOT_TOKEN"),
		BotUsername: os.Getenv("BOT_USERNAME"),
		JWTSecret:   getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		AdminIDs:    adminIDs,
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
	}

	log.Println("Configuration loaded:")
	log.Printf("  Port: %s", config.Port)
	log.Printf("  BotUsername: %s", config.BotUsername)
	log.Printf("  FrontendURL: %s", config.FrontendURL)
	log.Printf("  AdminIDs: %v", config.AdminIDs)

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (app *App) configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	response := map[string]string{
		"botUsername": app.config.BotUsername,
	}

	json.NewEncoder(w).Encode(response)
}

func (app *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))

	response := map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
		"bot":    app.config.BotUsername != "",
	}

	json.NewEncoder(w).Encode(response)
}

func (app *App) authHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var userData TelegramUser
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("=== AUTH REQUEST ===")
	log.Printf("User ID: %d", userData.ID)
	log.Printf("Username: %s", userData.Username)
	log.Printf("First Name: %s", userData.FirstName)

	if !app.verifyTelegramAuth(userData) {
		log.Printf("Invalid hash for user %d", userData.ID)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Invalid authentication data",
		})
		return
	}

	role := "user"
	userIDStr := fmt.Sprintf("%d", userData.ID)
	for _, adminID := range app.config.AdminIDs {
		if adminID == userIDStr {
			role = "admin"
			break
		}
	}

	log.Printf("User %d authenticated as %s", userData.ID, role)

	claims := &Claims{
		UserID:   userData.ID,
		Username: userData.Username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(app.config.JWTSecret))
	if err != nil {
		log.Printf("Error creating token: %v", err)
		http.Error(w, "Error creating token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Token:   tokenString,
		Role:    role,
	})
}

func (app *App) verifyTelegramAuth(userData TelegramUser) bool {
	dataCheckArr := []string{}

	if userData.ID != 0 {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("id=%d", userData.ID))
	}
	if userData.FirstName != "" {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("first_name=%s", userData.FirstName))
	}
	if userData.LastName != "" {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("last_name=%s", userData.LastName))
	}
	if userData.Username != "" {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("username=%s", userData.Username))
	}
	if userData.PhotoURL != "" {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("photo_url=%s", userData.PhotoURL))
	}
	if userData.AuthDate != 0 {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("auth_date=%d", userData.AuthDate))
	}

	sort.Strings(dataCheckArr)
	dataCheckString := strings.Join(dataCheckArr, "\n")

	secretKey := sha256.Sum256([]byte(app.config.BotToken))
	h := hmac.New(sha256.New, secretKey[:])
	h.Write([]byte(dataCheckString))
	calculatedHash := hex.EncodeToString(h.Sum(nil))

	isValid := calculatedHash == userData.Hash
	isRecent := time.Now().Unix()-userData.AuthDate < 86400

	if !isValid {
		log.Printf("Hash mismatch: got %s, expected %s", userData.Hash, calculatedHash)
	}
	if !isRecent {
		log.Printf("Auth date too old: %d", userData.AuthDate)
	}

	return isValid && isRecent
}

func (app *App) meHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenStr := r.Header.Get("Authorization")
	if tokenStr == "" || !strings.HasPrefix(tokenStr, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(app.config.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"role":          claims.Role,
		"userId":        claims.UserID,
		"username":      claims.Username,
	})
}

func main() {
	config := LoadConfig()

	if config.BotToken == "" {
		log.Fatal("BOT_TOKEN is required in .env file")
	}
	if config.BotUsername == "" {
		log.Fatal("BOT_USERNAME is required in .env file")
	}

	app := &App{config: config}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", app.healthHandler)
	mux.HandleFunc("/api/config", app.configHandler)
	mux.HandleFunc("/api/auth", app.authHandler)
	mux.HandleFunc("/api/me", app.meHandler)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"https://*.ngrok-free.app",
			"https://*.ngrok-free.dev",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Origin", "Accept"},
		AllowCredentials: true,
		Debug:            true,
	})

	handler := corsHandler.Handler(mux)

	log.Printf("Server starting on port %s", config.Port)
	log.Printf("Bot username: @%s", config.BotUsername)

	if err := http.ListenAndServe(":"+config.Port, handler); err != nil {
		log.Fatal(err)
	}
}
