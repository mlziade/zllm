package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"zllm/internal/api/handlers"
	"zllm/internal/auth"
	"zllm/internal/config"
	"zllm/internal/ollama"
)

// Server holds the HTTP server configuration
type Server struct {
	app    *fiber.App
	config *Config
}

// Config holds server configuration
type Config struct {
	OllamaClient *ollama.Client
	AppConfig    *config.Config
}

// New creates a new server instance
func New(cfg *config.Config) *Server {
	app := fiber.New()

	// Add CORS middleware
	app.Use(cors.New())

	// Create Ollama client
	ollamaClient := ollama.NewClient(cfg.OllamaURL)

	serverConfig := &Config{
		OllamaClient: ollamaClient,
		AppConfig:    cfg,
	}

	s := &Server{
		app:    app,
		config: serverConfig,
	}

	s.setupRoutes()
	return s
}

// Start starts the server
func (s *Server) Start(addr string) error {
	return s.app.Listen(addr)
}

// setupRoutes configures all the routes
func (s *Server) setupRoutes() {
	// Auth endpoints
	s.app.Post("/auth", handlers.HandleAuth(s.config.AppConfig))

	// Protected routes
	protected := s.app.Group("", auth.JWTMiddleware())

	// LLM endpoints
	llmGroup := protected.Group("/llm")
	llmGroup.Post("/generate", handlers.HandleGeneration(s.config.OllamaClient))
	llmGroup.Post("/generate/stream", handlers.HandleGenerationStream(s.config.OllamaClient))
	llmGroup.Post("/chat", handlers.HandleChat(s.config.OllamaClient))
	llmGroup.Post("/chat/stream", handlers.HandleChatStream(s.config.OllamaClient))

	// Model endpoints
	modelGroup := protected.Group("/models")
	modelGroup.Get("/", handlers.HandleListModels(s.config.OllamaClient))

	// Admin routes
	admin := protected.Group("", auth.AdminMiddleware())
	admin.Post("/models/add", handlers.HandleAddModel(s.config.OllamaClient))
	admin.Delete("/models/:model", handlers.HandleDeleteModel(s.config.OllamaClient))

	// Job endpoints
	jobGroup := protected.Group("/jobs")
	jobGroup.Post("/generate", handlers.HandleCreateGenerationJob())
	jobGroup.Post("/multimodal_extraction", handlers.HandleCreateMultimodalJob())
	jobGroup.Get("/:id/status", handlers.HandleGetJobStatus())
	jobGroup.Get("/:id/result", handlers.HandleGetJobResult())

	// Admin job endpoints
	adminJobs := admin.Group("/jobs")
	adminJobs.Get("/", handlers.HandleListJobs())
	adminJobs.Delete("/", handlers.HandleDeleteAllJobs())
}