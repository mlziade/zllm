# zllm

## Description

zllm is my implementation of a server with a LLM API that runs models locally on a VPS.

It uses the Golang framework [Fiber](https://github.com/gofiber/fiber) to integrate with [Ollama](https://github.com/ollama/ollama) running as a [server](https://github.com/ollama/ollama?tab=readme-ov-file#building).

## Technologies

### Backend

- Golang
- Fiber
- Ollama
- SQLite with GORM ORM
- JWT Authentication

### Deployment Infrastructure

- **Ubuntu**: Linux server for hosting the application
- **Nginx**: Web server acting as a reverse proxy
- **Systemd**: Service manager for the Fiber application daemon
- **Unix Socket**: Communication between Nginx and the Go Fiber server

### CI/CD

- **GitHub Actions**: Automated continuous integration and deployment pipeline

## API Documentation

Refer to the [API documentation](docs/endpoints.md) for detailed information about endpoints and authentication.

## Roadmap

### Completed
- ✅ Basic API server with Fiber framework
- ✅ JWT authentication with role-based access
- ✅ Integration with Ollama for local model execution
- ✅ Text generation endpoints (streaming and non-streaming)
- ✅ Model management (list, add, remove)
- ❌ OCR capabilities for text extraction from images (Tesseract) *[Deprecated in favor of multimodal models]*
- ✅ OCR capabilities for text extraction from images (Multimodal Models)
- ✅ Asynchronous Jobs implementation
- ✅ Asynchronous Generation Jobs
- ✅ Asynchronous Multimodal Extraction


### Projected
- 🚧 Non multimodal LLM OCR Implementation for images and pdfs
- 🚧 Translation endpoint for multiple languages (Multimodal models)
- 🚧 Image generation
- ✅ Automated CI/CD workflow using GitHub Actions and SSH deploy
