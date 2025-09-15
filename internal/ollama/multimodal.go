package ollama

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MultiModalTextExtractionFromImage processes an image and extracts text using multimodal LLM
func (c *Client) MultiModalTextExtractionFromImage(modelName string, fileBytes []byte, filename string) (map[string]interface{}, error) {
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