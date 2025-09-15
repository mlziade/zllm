package jobs

// GenerationRequest represents a text generation request
type GenerationRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// MultiModalExtractionRequest represents a multimodal text extraction request
type MultiModalExtractionRequest struct {
	Model         string `json:"model"`
	FileBytes     []byte `json:"file_bytes"`
	FileExtension string `json:"file_extension"`
}

