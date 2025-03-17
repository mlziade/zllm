package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
)

/* Ollama Server API endpoints
 *
 * https://github.com/ollama/ollama/blob/main/docs/api.md
 *
 */

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type AddModelRequest struct {
	Model string `json:"model"`
}

// JWT claims structure
type JWTClaims struct {
	jwt.StandardClaims
}

// Authentication request structure
type AuthRequest struct {
	APIKey string `json:"api_key"`
}

// Authentication response structure
type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Generate a JWT token with specified expiration time
func generateToken(expirationTime time.Time) (string, error) {
	claims := &JWTClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))

	return tokenString, err
}

// JWT authentication middleware
func jwtMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get token from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Authorization header is required"})
		}

		// Check if the header has the "Bearer " prefix
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid authorization header format"})
		}

		tokenString := headerParts[1]

		// Parse and validate the token
		token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		if _, ok := token.Claims.(*JWTClaims); ok && token.Valid {
			return c.Next()
		}

		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}
}

func main() {
	// Create a new Fiber server
	app := fiber.New()

	// Authentication endpoint
	app.Post("/auth", func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get API key from .env
		expectedAPIKey := os.Getenv("API_KEY")
		if expectedAPIKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "API_KEY is not set in the .env file"})
		}

		// Get JWT secret key from .env
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			return c.Status(500).JSON(fiber.Map{"error": "JWT_SECRET is not set in the .env file"})
		}

		// Parse the request body
		var req AuthRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Error parsing request body"})
		}

		// Validate API key
		if req.APIKey != expectedAPIKey {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid API key"})
		}

		// Generate JWT token valid for 24 hours
		expirationTime := time.Now().Add(24 * time.Hour)
		tokenString, err := generateToken(expirationTime)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Error generating token"})
		}

		// Return the token and expiration time
		return c.JSON(AuthResponse{
			Token:     tokenString,
			ExpiresAt: expirationTime,
		})
	})

	// GET /list-models
	// List all models available locally on Ollama
	app.Get("/list-models", jwtMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Make a GET request to the Ollama API
		resp, err := http.Get(url + "/api/tags")
		if err != nil {
			return c.Status(500).SendString("Error contacting Ollama: " + err.Error())
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(500).SendString("Error reading response body")
		}

		// Parse the JSON response
		var responseJson map[string]interface{}
		if err := json.Unmarshal(body, &responseJson); err != nil {
			return c.Status(500).SendString("Error parsing JSON response: " + err.Error())
		}

		// Extract model names from the response
		models, ok := responseJson["models"].([]interface{})
		if !ok {
			return c.Status(500).SendString("Invalid JSON format: 'models' field missing or incorrect")
		}

		availableModels := []string{}
		for _, model := range models {
			if modelMap, ok := model.(map[string]interface{}); ok {
				if name, exists := modelMap["name"].(string); exists {
					availableModels = append(availableModels, name)
				}
			}
		}

		// Return the formatted response
		return c.JSON(fiber.Map{"models": availableModels})
	})

	// GET /generate
	// Generate a awnser from a model and prompt
	app.Post("/generate", jwtMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into an GenerateRequest structure
		var req GenerateRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the prompt and model fields are provided
		if req.Prompt == "" || req.Model == "" {
			return c.Status(400).SendString("Prompt and model are required")
		}

		// Create the request payload for the Ollama API
		ollamaReq := map[string]interface{}{
			"model":  req.Model,
			"prompt": req.Prompt,
			"stream": false,
		}

		// Marshal the payload into JSON
		reqBytes, err := json.Marshal(ollamaReq)
		if err != nil {
			return c.Status(500).SendString("Error creating request")
		}

		// Send a POST request to the Ollama API generate endpoint
		resp, err := http.Post(url+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
		if err != nil {
			return c.Status(500).SendString("Error contacting Ollama")
		}
		defer resp.Body.Close()

		// Read the response body from Ollama
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(500).SendString("Error reading response body")
		}

		// Parse the JSON response from Ollama
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return c.Status(500).SendString("Error parsing Ollama response")
		}

		return c.JSON(fiber.Map{
			"model":    req.Model,
			"response": apiResp["response"],
		})
	})

	// POST /generate-streaming
	// Generate an answer from a model and prompt with a streaming response
	app.Post("/generate-streaming", jwtMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into a GenerateRequest structure
		var req GenerateRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the prompt and model fields are provided
		if req.Prompt == "" || req.Model == "" {
			return c.Status(400).SendString("Prompt and model are required")
		}

		// Create the request payload for the Ollama API with streaming enabled
		ollamaReq := map[string]interface{}{
			"model":  req.Model,
			"prompt": req.Prompt,
			"stream": true,
		}

		// Marshal the payload into JSON
		reqBytes, err := json.Marshal(ollamaReq)
		if err != nil {
			return c.Status(500).SendString("Error creating request")
		}

		// Send a POST request to the Ollama API generate endpoint
		resp, err := http.Post(url+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
		if err != nil {
			return c.Status(500).SendString("Error contacting Ollama")
		}

		// Set the response header content type
		c.Set("Content-Type", "application/json")

		// Stream the response from Ollama to the client
		c.Set("Content-Type", "application/json")
		c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
			defer resp.Body.Close()
			io.Copy(w, resp.Body)
			w.Flush()
		})
		return nil
	})

	// POST /add-model
	// Pull a model from the Ollama library
	app.Post("/add-model", jwtMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into an AddModelRequest structure
		var req AddModelRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the model field is provided
		if req.Model == "" {
			return c.Status(400).SendString("Model is required")
		}

		// Create the request payload for the Ollama API including stream=false
		ollamaReq := map[string]interface{}{
			"model":  req.Model,
			"stream": false,
		}

		// Marshal the payload into JSON
		reqBytes, err := json.Marshal(ollamaReq)
		if err != nil {
			return c.Status(500).SendString("Error creating request")
		}

		// Send a POST request to the Ollama API pull endpoint
		resp, err := http.Post(url+"/api/pull", "application/json", bytes.NewBuffer(reqBytes))
		if err != nil {
			return c.Status(500).SendString("Error contacting Ollama")
		}
		defer resp.Body.Close()

		// Read the response body from Ollama
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(500).SendString("Error reading response body")
		}

		// Parse the JSON response from Ollama
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return c.Status(500).SendString("Error parsing Ollama response")
		}

		// Only return a 200 if the status is "success"
		if status, ok := apiResp["status"].(string); ok && status == "success" {
			return c.Status(200).JSON(fiber.Map{"status": "success"})
		}

		// Return appropriate status code with the error from Ollama
		return c.Status(resp.StatusCode).JSON(apiResp)
	})

	log.Fatal(app.Listen(":3000"))
}
