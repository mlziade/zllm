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
	"os/exec"
	"path/filepath"
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

// DeleteModelRequest structure
type DeleteModelRequest struct {
	Model string `json:"model"`
}

// JWT claims structure
type JWTClaims struct {
	Role string `json:"role"`
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

// Generate a JWT token with specified expiration time and role
func generateToken(expirationTime time.Time, role string) (string, error) {
	claims := &JWTClaims{
		Role: role,
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

		if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
			// Store user role in context for later use
			c.Locals("role", claims.Role)
			return c.Next()
		}

		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}
}

// Admin-only middleware
func adminMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if the user has admin role
		role := c.Locals("role")
		if role != "admin" {
			return c.Status(403).JSON(fiber.Map{"error": "Admin access required"})
		}

		return c.Next()
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

		// Get API keys from .env
		normalAPIKey := os.Getenv("API_KEY")
		adminAPIKey := os.Getenv("ADMIN_API_KEY")

		if normalAPIKey == "" || adminAPIKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "API keys are not properly set in the .env file"})
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

		// Determine role based on API key
		var role string
		if req.APIKey == adminAPIKey {
			role = "admin"
		} else if req.APIKey == normalAPIKey {
			role = "user"
		} else {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid API key"})
		}

		// Generate JWT token valid for 24 hours
		expirationTime := time.Now().Add(24 * time.Hour)
		tokenString, err := generateToken(expirationTime, role)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Error generating token"})
		}

		// Return the token, role, and expiration time
		return c.JSON(fiber.Map{
			"token":      tokenString,
			"role":       role,
			"expires_at": expirationTime,
		})
	})

	// GET llms/models/list
	// List all models available locally on Ollama
	app.Get("/llms/models/list", jwtMiddleware(), func(c *fiber.Ctx) error {
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

	// GET llms/generate
	// Generate a awnser from a model and prompt
	app.Post("/llms/generate", jwtMiddleware(), func(c *fiber.Ctx) error {
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

	// POST llms/generate-streaming
	// Generate an answer from a model and prompt with a streaming response
	app.Post("/llms/generate-streaming", jwtMiddleware(), func(c *fiber.Ctx) error {
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

		// Set response headers for streaming
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		// Stream the response from Ollama to the client
		c.Response().SetBodyStreamWriter(func(w *bufio.Writer) {
			defer resp.Body.Close()

			// Create a scanner to read the response line by line
			scanner := bufio.NewScanner(resp.Body)

			// Increase scanner buffer size for potentially large lines
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 1024*1024)

			// Process each line as it arrives
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}

				// Format as server-sent event
				fmt.Fprintf(w, "data: %s\n\n", line)
				w.Flush() // Important: flush after each line to send immediately
			}

			// Check for errors during scanning
			if err := scanner.Err(); err != nil {
				log.Printf("Error scanning Ollama response: %v", err)
			}
		})
		return nil
	})

	// POST llms/models/add
	// Pull a model from the Ollama library
	app.Post("/llms/models/add", jwtMiddleware(), adminMiddleware(), func(c *fiber.Ctx) error {
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

	// DELETE llms/models/delete
	// Delete a model from Ollama
	app.Delete("/llms/models/delete", jwtMiddleware(), adminMiddleware(), func(c *fiber.Ctx) error {
		// Load the .env file
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: Error loading .env file")
		}

		// Get the Ollama URL from the .env file
		url := os.Getenv("OLLAMA_URL")
		if url == "" {
			return c.Status(500).SendString("OLLAMA_URL is not set in the .env file")
		}

		// Parse the request body into a DeleteModelRequest structure
		var req DeleteModelRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		// Ensure the model field is provided
		if req.Model == "" {
			return c.Status(400).SendString("Model name is required")
		}

		// Create the request payload for the Ollama API
		ollamaReq := map[string]interface{}{
			"model": req.Model,
		}

		// Marshal the payload into JSON
		reqBytes, err := json.Marshal(ollamaReq)
		if err != nil {
			return c.Status(500).SendString("Error creating request")
		}

		// Create a DELETE request to the Ollama API
		client := &http.Client{}
		request, err := http.NewRequest("DELETE", url+"/api/delete", bytes.NewBuffer(reqBytes))
		if err != nil {
			return c.Status(500).SendString("Error creating DELETE request")
		}
		request.Header.Set("Content-Type", "application/json")

		// Send the request
		resp, err := client.Do(request)
		if err != nil {
			return c.Status(500).SendString("Error contacting Ollama: " + err.Error())
		}
		defer resp.Body.Close()

		// Pass through the status code from Ollama
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return c.Status(resp.StatusCode).SendString("Error from Ollama API: " + string(body))
		}

		// Successful deletion
		return c.JSON(fiber.Map{"status": "success", "message": "Model deleted successfully"})
	})

	// POST ocr/extract/image
	// Extract text from an image using Tesseract OCR
	// Supports .png, .jpg, and .jpeg images
	// Supports English (en) and Portuguese (pt) languages (defaults to: English)
	app.Post("ocr/extract/image", jwtMiddleware(), func(c *fiber.Ctx) error {
		// Check if Tesseract is installed
		tesseractPath, err := exec.LookPath("tesseract")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Tesseract OCR is not installed or not in PATH",
				"details": err.Error(),
			})
		}

		log.Printf("Found tesseract at: %s", tesseractPath)

		// Determine tessdata directory location
		// Try to get it from environment variable first
		tessdataDir := os.Getenv("TESSDATA_PREFIX")
		if tessdataDir == "" {
			// If not set, try to infer it from tesseract path
			tessdataDir = strings.Replace(filepath.Dir(tesseractPath), "\\", "/", -1) + "/tessdata"
		}

		log.Printf("Using tessdata directory: %s", tessdataDir)

		// Get the image file from the request
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error":   "Error parsing file",
				"details": err.Error(),
			})
		}

		// Validate file extension (only allow .png, .jpg, .jpeg)
		filename := strings.ToLower(file.Filename)
		isValidExtension := false
		allowedExtensions := []string{".png", ".jpg", ".jpeg"}
		for _, ext := range allowedExtensions {
			if strings.HasSuffix(filename, ext) {
				isValidExtension = true
				break
			}
		}

		if !isValidExtension {
			return c.Status(400).JSON(fiber.Map{
				"error": "Unsupported file type. Only .png, .jpg, and .jpeg images are supported",
			})
		}

		// Get language parameter from query, default to English if not provided
		lang := c.Query("lang", "en")

		// Map to Tesseract language codes
		var tesseractLang string
		switch lang {
		case "en":
			tesseractLang = "eng"
		case "pt":
			tesseractLang = "por"
		default:
			tesseractLang = "eng" // Default to English for invalid languages
		}

		// Check if the language data file exists
		langDataFile := fmt.Sprintf("%s/%s.traineddata", tessdataDir, tesseractLang)
		if _, err := os.Stat(langDataFile); os.IsNotExist(err) {
			log.Printf("Language data file not found: %s", langDataFile)

			// If requested language is not English and English data exists, fallback to English
			if tesseractLang != "eng" {
				engDataFile := fmt.Sprintf("%s/eng.traineddata", tessdataDir)
				if _, err := os.Stat(engDataFile); err == nil {
					log.Printf("Falling back to English language")
					tesseractLang = "eng"
					lang = "en"
				} else {
					return c.Status(500).JSON(fiber.Map{
						"error":    "Requested language data not available",
						"details":  fmt.Sprintf("Language data file not found: %s", langDataFile),
						"solution": "Please install Tesseract language data for " + lang + " or try with English (en)",
					})
				}
			} else {
				return c.Status(500).JSON(fiber.Map{
					"error":    "English language data not available",
					"details":  "English language training data not found",
					"solution": "Please install Tesseract English language data",
				})
			}
		}

		// Create uploads directory if it doesn't exist
		uploadsDir := "./uploads"
		if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
			if err := os.MkdirAll(uploadsDir, 0755); err != nil {
				return c.Status(500).JSON(fiber.Map{
					"error":   "Error creating uploads directory",
					"details": err.Error(),
				})
			}
		}

		// Save the file to the uploads directory with a unique name to avoid conflicts
		tempFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), file.Filename)
		filePath := fmt.Sprintf("%s/%s", uploadsDir, tempFilename)
		if err := c.SaveFile(file, filePath); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error saving file",
				"details": err.Error(),
			})
		}

		// Ensure the file is removed after processing
		defer os.Remove(filePath)

		log.Printf("Running Tesseract OCR on file: %s with language: %s", filePath, tesseractLang)

		// Create command for Tesseract OCR
		cmd := exec.Command(
			tesseractPath,
			filePath,
			"stdout",
			"-l", tesseractLang,
			"--oem", "1", // Use LSTM OCR Engine
			"--dpi", "300", // Assume 300 DPI for better accuracy
			"--psm", "3", // Auto-page segmentation with OSD
		)

		// Set TESSDATA_PREFIX environment variable
		cmd.Env = append(os.Environ(), fmt.Sprintf("TESSDATA_PREFIX=%s", filepath.Dir(tessdataDir)))

		// Capture both stdout and stderr for better error diagnostics
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Tesseract Error: %v, Output: %s", err, string(out))
			return c.Status(500).JSON(fiber.Map{
				"error":   "Error running Tesseract OCR",
				"details": err.Error(),
				"output":  string(out),
				"command": cmd.String(),
				"env":     fmt.Sprintf("TESSDATA_PREFIX=%s", filepath.Dir(tessdataDir)),
			})
		}

		// Return extracted text
		return c.JSON(fiber.Map{
			"text":           strings.TrimSpace(string(out)),
			"file_processed": file.Filename,
			"language":       lang,
		})
	})

	log.Fatal(app.Listen(":3000"))
}
