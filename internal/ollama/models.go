package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AddModel pulls a model from Ollama library
func (c *Client) AddModel(req AddModelRequest) (map[string]interface{}, error) {
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
	resp, err := http.Post(c.BaseURL+"/api/pull", "application/json", bytes.NewBuffer(reqBytes))
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
func (c *Client) DeleteModel(req DeleteModelRequest) error {
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
	request, err := http.NewRequest("DELETE", c.BaseURL+"/api/delete", bytes.NewBuffer(reqBytes))
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