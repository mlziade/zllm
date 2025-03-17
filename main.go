package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
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

func main() {
	// Create a new Fiber server
	app := fiber.New()

	/* Route Handling */

	// GET /list-models
	// List all models available locally on Ollama
	app.Get("/list-models", func(c *fiber.Ctx) error {
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
	// Generate a awnser from a model
	// TODO: Add streaming support
	app.Post("/generate", func(c *fiber.Ctx) error {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}

		url := os.Getenv("OLLAMA_URL")

		var req GenerateRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Error parsing request body")
		}

		if req.Prompt == "" || req.Model == "" {
			return c.Status(400).SendString("Prompt and model are required")
		}

		fmt.Println("Prompt: ", req.Prompt)
		fmt.Println("Model: ", req.Model)

		ollamaReq := map[string]string{
			"model":  req.Model,
			"prompt": req.Prompt,
		}

		reqBytes, err := json.Marshal(ollamaReq)
		if err != nil {
			return c.Status(500).SendString("Error creating request")
		}

		resp, err := http.Post(url+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
		if err != nil {
			return c.Status(500).SendString("Error contacting Ollama")
		}

		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		return c.JSON(fiber.Map{"response": string(body)})
	})

	// POST /add-model
	// Pull a model from the Ollama library
	app.Post("/add-model", func(c *fiber.Ctx) error {
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
