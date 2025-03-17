# zllm

## Description

zllm is my implementation of a server with a LLM API that runs models locally on a VPS.

It uses the Golang framework [Fiber](https://github.com/gofiber/fiber) to integrate with [Ollama](https://github.com/ollama/ollama) running as a [server](https://github.com/ollama/ollama?tab=readme-ov-file#building).

## Authenticating

The API uses role-based JWT authentication to secure endpoints:

### Authentication Flow

1. Request a JWT token by providing an API key:
   ```
   POST /auth
   Content-Type: application/json
   
   {
     "api_key": "your_api_key_here"
   }
   ```

2. The server returns a token with role permissions:
   ```json
   {
     "token": "eyJhbGciOiJIUzI1...",
     "role": "user|admin",
     "expires_at": "2025-03-18T12:34:56Z"
   }
   ```

3. Use the token for subsequent requests:
   ```
   GET /list-models
   Authorization: Bearer eyJhbGciOiJIUzI1...
   ```

### API Keys and Roles

- **User API Key**: Standard access to model generation and listing
- **Admin API Key**: Additional access to admin-only endpoints like adding models

API keys should be configured in the `.env` file:
```
API_KEY=your_normal_api_key_here
ADMIN_API_KEY=your_admin_api_key_here
JWT_SECRET=your_secure_jwt_signing_key
```

### Protected Endpoints

- All endpoints except `/auth` require a valid JWT token
- Admin operations (like `/add-model`) require a token with admin role

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