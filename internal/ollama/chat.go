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

// ChatResponse sends a chat message to a model and returns the response
func (c *Client) ChatResponse(req ChatRequest) (map[string]interface{}, error) {
	log.Printf("Generating chat response | Model: %s", req.Model)

	if req.Model == "" {
		log.Println("ChatResponse failed: model is required")
		return nil, fmt.Errorf("model is required")
	}

	// Create the request payload for the Ollama API
	ollamaReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
	}

	// Marshal the payload into JSON
	reqBytes, err := json.Marshal(ollamaReq)
	if err != nil {
		log.Printf("ChatResponse failed to marshal request | Model: %s | Error: %v", req.Model, err)
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	log.Printf("Sending chat request to Ollama | Model: %s", req.Model)
	resp, err := http.Post(c.BaseURL+"/api/chat", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		log.Printf("ChatResponse failed to contact Ollama | Model: %s | Error: %v", req.Model, err)
		return nil, fmt.Errorf("error contacting Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code first
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err == nil {
			if errMsg, ok := apiResp["error"].(string); ok {
				if strings.Contains(errMsg, "not found") {
					log.Printf("ChatResponse model not found | Model: %s", req.Model)
					return nil, fmt.Errorf("model not found")
				}
				if isMemoryError(errMsg) {
					log.Printf("ChatResponse memory error | Model: %s | Error: %s", req.Model, errMsg)
					return nil, fmt.Errorf("model requires more system memory")
				}
				log.Printf("ChatResponse Ollama error | Model: %s | Error: %s", req.Model, errMsg)
				return nil, fmt.Errorf("ollama error: %s", errMsg)
			}
		}
		log.Printf("ChatResponse API error | Model: %s | Status: %d", req.Model, resp.StatusCode)
		return nil, fmt.Errorf("ollama API error: status %d", resp.StatusCode)
	}

	// Ollama may return NDJSON (one JSON object per line)
	scanner := bufio.NewScanner(resp.Body)
	var lastResp map[string]interface{}
	var allResponses []string
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(line, &obj); err != nil {
			log.Printf("ChatResponse failed to parse Ollama NDJSON line | Model: %s | Error: %v | Line: %s", req.Model, err, string(line))
			continue // skip invalid lines
		}

		// Check for error in the response object
		if errMsg, ok := obj["error"].(string); ok {
			if strings.Contains(errMsg, "not found") {
				log.Printf("ChatResponse model not found in stream | Model: %s", req.Model)
				return nil, fmt.Errorf("model not found")
			}
			if isMemoryError(errMsg) {
				log.Printf("ChatResponse memory error in stream | Model: %s | Error: %s", req.Model, errMsg)
				return nil, fmt.Errorf("model requires more system memory")
			}
			log.Printf("ChatResponse Ollama error in stream | Model: %s | Error: %s", req.Model, errMsg)
			return nil, fmt.Errorf("ollama error: %s", errMsg)
		}

		lastResp = obj
		if msg, ok := obj["message"]; ok {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				if content, ok := msgMap["content"].(string); ok {
					allResponses = append(allResponses, content)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("ChatResponse scanner error | Model: %s | Error: %v", req.Model, err)
		return nil, fmt.Errorf("error reading Ollama NDJSON: %w", err)
	}

	// If we got at least one message, return the concatenated content
	if len(allResponses) > 0 {
		return map[string]interface{}{
			"model":    req.Model,
			"response": strings.Join(allResponses, ""),
		}, nil
	}

	// If Ollama returned a single JSON object (not NDJSON), fallback to old logic
	if lastResp != nil {
		return map[string]interface{}{
			"model":    req.Model,
			"response": lastResp["response"],
		}, nil
	}

	return nil, fmt.Errorf("no valid response from Ollama")
}

// StreamChatResponse sets up streaming chat from Ollama to the client
func (c *Client) StreamChatResponse(req ChatRequest, writer *bufio.Writer) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	ollamaReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}
	// Marshal the payload into JSON
	reqBytes, err := json.Marshal(ollamaReq)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	// Send a POST request to the Ollama API chat endpoint
	resp, err := http.Post(c.BaseURL+"/api/chat", "application/json", bytes.NewBuffer(reqBytes))
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

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Format as server-sent event
		fmt.Fprintf(writer, "data: %s\n\n", line)
		writer.Flush()
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning Ollama chat response: %v", err)
		return fmt.Errorf("error scanning Ollama chat response: %w", err)
	}
	return nil
}