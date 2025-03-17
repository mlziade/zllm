# zllm

## Description

zllm is my implementation of a server with a LLM API that runs models locally on a VPS.

It uses the Golang framework [Fiber](https://github.com/gofiber/fiber) to integrate with [Ollama](https://github.com/ollama/ollama) running as a [server](https://github.com/ollama/ollama?tab=readme-ov-file#building).

## Technologies

### Full-Stack

- Golang
- Fiber
- Ollama

### Deployment Infrastructure

- Ubuntu: Linux server for hosting the application
- Nginx: Web server acting as a reverse proxy
- Systemd: Linux initialization system used to automate application startup and manage service processes
- Unix Socket: For communication between Gunicorn and Nginx