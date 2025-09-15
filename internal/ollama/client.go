package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Client wraps Ollama API interactions
type Client struct {
	BaseURL string
}

// NewClient creates a new Ollama client
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL}
}

// isMemoryError checks if an error message indicates insufficient system memory
func isMemoryError(errorMsg string) bool {
	memoryIndicators := []string{
		"model requires more system memory",
		"not enough memory",
		"insufficient memory",
		"out of memory",
		"memory allocation failed",
	}

	for _, indicator := range memoryIndicators {
		if strings.Contains(strings.ToLower(errorMsg), strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

// ListModels retrieves all models available locally on Ollama
func (c *Client) ListModels() ([]string, error) {
	// Make a GET request to the Ollama API
	resp, err := http.Get(c.BaseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("error contacting Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Parse the JSON response
	var responseJson map[string]interface{}
	if err := json.Unmarshal(body, &responseJson); err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %w", err)
	}

	// Extract model names from the response
	models, ok := responseJson["models"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid JSON format: 'models' field missing or incorrect")
	}

	availableModels := []string{}
	for _, model := range models {
		if modelMap, ok := model.(map[string]interface{}); ok {
			if name, exists := modelMap["name"].(string); exists {
				availableModels = append(availableModels, name)
			}
		}
	}

	return availableModels, nil
}

// GenerateResponse sends a prompt to a model and returns the response
func (c *Client) GenerateResponse(req GenerationRequest) (map[string]interface{}, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
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
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Send a POST request to the Ollama API generate endpoint
	resp, err := http.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("error contacting Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body from Ollama
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Parse the JSON response from Ollama
	var apiResp map[string]interface{}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("error parsing Ollama response: %w", err)
	}

	// Check for errors in the response
	if errMsg, ok := apiResp["error"].(string); ok {
		if strings.Contains(errMsg, "not found") {
			return nil, fmt.Errorf("model not found")
		}
		if isMemoryError(errMsg) {
			return nil, fmt.Errorf("model requires more system memory")
		}
		return nil, fmt.Errorf("ollama error: %s", errMsg)
	}

	return map[string]interface{}{
		"model":    req.Model,
		"response": apiResp["response"],
	}, nil
}

// StreamGenerationResponse sets up streaming from Ollama to the client
func (c *Client) StreamGenerationResponse(req GenerationRequest, writer *bufio.Writer) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
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
		return fmt.Errorf("error creating request: %w", err)
	}

	// Send a POST request to the Ollama API generate endpoint
	resp, err := http.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return fmt.Errorf("error contacting Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code first
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err == nil {
			if errMsg, ok := apiResp["error"].(string); ok {
				if strings.Contains(errMsg, "not found") {
					return fmt.Errorf("model not found")
				}
				if isMemoryError(errMsg) {
					return fmt.Errorf("model requires more system memory")
				}
				return fmt.Errorf("ollama error: %s", errMsg)
			}
		}
		return fmt.Errorf("ollama API error: status %d", resp.StatusCode)
	}

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
		fmt.Fprintf(writer, "data: %s\n\n", line)
		writer.Flush() // Important: flush after each line to send immediately
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning Ollama response: %v", err)
		return fmt.Errorf("error scanning Ollama response: %w", err)
	}

	return nil
}