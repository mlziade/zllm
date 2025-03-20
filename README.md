# zllm

## Description

zllm is my implementation of a server with a LLM API that runs models locally on a VPS.

It uses the Golang framework [Fiber](https://github.com/gofiber/fiber) to integrate with [Ollama](https://github.com/ollama/ollama) running as a [server](https://github.com/ollama/ollama?tab=readme-ov-file#building).

## Technologies

### Backend

- Golang
- Fiber
- Ollama
- Tesseract

### Deployment Infrastructure

- Ubuntu: Linux server for hosting the application
- Nginx: Web server acting as a reverse proxy
- Systemd: Linux initialization system used to automate application startup and manage service processes
- Unix Socket: For communication between Gunicorn and Nginx

## API Documentation

Refer to the [API documentation](docs/endpoints.md) for detailed information about endpoints and authentication.

## Roadmap

### Completed
- ✅ Basic API server with Fiber framework
- ✅ JWT authentication with role-based access
- ✅ Integration with Ollama for local model execution
- ✅ Text generation endpoints (streaming and non-streaming)
- ✅ Model management (list, add)
- ✅ OCR capabilities for text extraction from images (Tesseract)

### Future
- 🚧 Multimodal models support (text + image inputs)
- 🚧 Translation endpoint for multiple languages
- 🚧 Image generation