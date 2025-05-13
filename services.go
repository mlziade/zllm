package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type MultimodalModel string

const (
	Gemma3   MultimodalModel = "gemma3:4b"
	Llava7   MultimodalModel = "llava:7b"
	MiniCPMv MultimodalModel = "minicpm-v:8b"
)

type GenerationRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type MultiModalExtractionRequest struct {
	Model         string `json:"model"`
	FileBytes     []byte `json:"file_bytes"`
	Filename      string `json:"filename"`
	FileExtension string `json:"file_extension"`
}

type AddModelRequest struct {
	Model string `json:"model"`
}

type DeleteModelRequest struct {
	Model string `json:"model"`
}

// ListModels retrieves all models available locally on Ollama
func ListModels(ollamaURL string) ([]string, error) {
	// Make a GET request to the Ollama API
	resp, err := http.Get(ollamaURL + "/api/tags")
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
func GenerateResponse(ollamaURL string, req GenerationRequest) (map[string]interface{}, error) {
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
	resp, err := http.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
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

	// Check for model not found error
	if errMsg, ok := apiResp["error"].(string); ok && strings.Contains(errMsg, "not found") {
		return nil, fmt.Errorf("model not found")
	}

	return map[string]interface{}{
		"model":    req.Model,
		"response": apiResp["response"],
	}, nil
}

// StreamResponse sets up streaming from Ollama to the client
func StreamGenerationResponse(ollamaURL string, req GenerationRequest, writer *bufio.Writer) error {
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
	resp, err := http.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return fmt.Errorf("error contacting Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Check for model not found error before streaming
	bodyPeek, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}
	var apiResp map[string]interface{}
	if err := json.Unmarshal(bodyPeek, &apiResp); err == nil {
		if errMsg, ok := apiResp["error"].(string); ok && strings.Contains(errMsg, "not found") {
			return fmt.Errorf("model not found")
		}
	}
	// If not an error, re-create the response body for streaming
	resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyPeek), resp.Body))

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

// AddModel pulls a model from Ollama library
func AddModel(ollamaURL string, req AddModelRequest) (map[string]interface{}, error) {
	// Create the request payload for the Ollama API including stream=false
	ollamaReq := map[string]interface{}{
		"model":  req.Model,
		"stream": false,
	}

	// Marshal the payload into JSON
	reqBytes, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Send a POST request to the Ollama API pull endpoint
	resp, err := http.Post(ollamaURL+"/api/pull", "application/json", bytes.NewBuffer(reqBytes))
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

	return apiResp, nil
}

// DeleteModel removes a model from Ollama
func DeleteModel(ollamaURL string, req DeleteModelRequest) error {
	// Create the request payload for the Ollama API
	ollamaReq := map[string]interface{}{
		"model": req.Model,
	}

	// Marshal the payload into JSON
	reqBytes, err := json.Marshal(ollamaReq)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Create a DELETE request to the Ollama API
	client := &http.Client{}
	request, err := http.NewRequest("DELETE", ollamaURL+"/api/delete", bytes.NewBuffer(reqBytes))
	if err != nil {
		return fmt.Errorf("error creating DELETE request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("error contacting Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error from Ollama API: %s", string(body))
	}

	return nil
}

// MultiModalTextExtractionFromImage processes an image and extracts text using multimodal LLM
func MultiModalTextExtractionFromImage(ollamaURL, modelName string, fileBytes []byte, filename string) (map[string]interface{}, error) {
	// Convert the image to base64
	base64Image := base64.StdEncoding.EncodeToString(fileBytes)

	// Create the request payload for the Ollama API
	ollamaReq := map[string]interface{}{
		"model":  modelName,
		"prompt": "Please carefully extract and transcribe all text visible in this image. Return your response as a JSON object with the following structure: {\"original_text\": \"[extracted text]\"}",
		"images": []string{base64Image},
		"stream": false,
	}

	// Marshal the payload into JSON
	reqBytes, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Send a POST request to the Ollama API generate endpoint
	resp, err := http.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(reqBytes))
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

	// Check for model not found error
	if errMsg, ok := apiResp["error"].(string); ok && strings.Contains(errMsg, "not found") {
		return nil, fmt.Errorf("model not found")
	}

	// Extract the LLM's text response
	responseText, ok := apiResp["response"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid response format from Ollama")
	}

	// Try to parse the JSON content from the LLM's text response
	jsonStart := strings.Index(responseText, "{")
	jsonEnd := strings.LastIndex(responseText, "}") + 1

	// Check if valid JSON boundaries were found
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonContent := responseText[jsonStart:jsonEnd]

		// Try to parse the extracted JSON content
		var extractedData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonContent), &extractedData); err == nil {
			// Successfully parsed JSON from LLM response
			result := map[string]interface{}{
				"file_processed": filename,
				"model":          modelName,
			}

			// Add just the original text to the result
			if originalText, exists := extractedData["original_text"]; exists {
				result["original_text"] = originalText
			}

			return result, nil
		}
	}

	// If we couldn't parse JSON from the response, return the raw response with a warning
	return map[string]interface{}{
		"warning":        "Could not parse structured data from LLM response",
		"raw_response":   responseText,
		"file_processed": filename,
		"model":          modelName,
	}, nil
}
