package main

import (
	"bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
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
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}

		url := os.Getenv("OLLAMA_URL")

		resp, err := http.Get(url+"/api/tags")
		if err != nil {
			return c.Status(500).SendString("Error contacting Ollama")
		}

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		return c.JSON(fiber.Map{"response": string(body)})
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
		body, _ := ioutil.ReadAll(resp.Body)

		return c.JSON(fiber.Map{"response": string(body)})
	})

	// GET /pull
	// Pull a model from the Ollama library
	app.Post("/add-model", func(c *fiber.Ctx) error {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
		
		url := os.Getenv("OLLAMA_URL")

		var req AddModelRequest
        if err := c.BodyParser(&req); err != nil {
            return c.Status(400).SendString("Error parsing request body")
        }

        if req.Model == "" {
            return c.Status(400).SendString("Model is required")
        }

		fmt.Println("Model: ", req.Model)

		ollamaReq := map[string]string{
            "model":  req.Model,
        }

		reqBytes, err := json.Marshal(ollamaReq)
        if err != nil {
            return c.Status(500).SendString("Error creating request")
        }
        
        resp, err := http.Post(url+"/api/pull", "application/json", bytes.NewBuffer(reqBytes))
        if err != nil {
            return c.Status(500).SendString("Error contacting Ollama")
        }

		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		return c.JSON(fiber.Map{"response": string(body)})
	})

	log.Fatal(app.Listen(":3000"))
}
