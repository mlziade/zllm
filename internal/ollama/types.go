package ollama

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

type ChatRole string

const (
	System    ChatRole = "system"
	User      ChatRole = "user"
	Assistant ChatRole = "assistant"
	Tool      ChatRole = "tool"
)

type Message struct {
	Role    ChatRole `json:"role"`
	Content string   `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type AddModelRequest struct {
	Model string `json:"model"`
}

type DeleteModelRequest struct {
	Model string `json:"model"`
}