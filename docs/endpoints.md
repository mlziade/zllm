# ZLLM API Documentation

## Authentication

The API uses role-based JWT authentication to secure endpoints.

### Authentication Flow

1.  Request a JWT token by providing an API key:

    ````json
    POST /auth
    Content-Type: application/json

    {
      "api_key": "your_api_key_here"
    }
    ````
2.  The server returns a token with role permissions:

    ````json
    {
      "token": "eyJhbGciOiJIUzI1...",
      "role": "user|admin",
      "expires_at": "2025-03-18T12:34:56Z"
    }
    ````
3.  Use the token for subsequent requests:

    ````json
    GET /list-models
    Authorization: Bearer eyJhbGciOiJIUzI1...
    ````

### API Keys and Roles

*   **User API Key**: Standard access to model generation and listing
*   **Admin API Key**: Additional access to admin-only endpoints like adding models

API keys should be configured in the `.env` file, refer to the [example env](../example.env).

### Protected Endpoints

*   All endpoints except `/auth` require a valid JWT token
*   Admin operations (like `/add-model`) require a token with admin role

## Endpoints

### Authentication

#### **POST /auth**

Authenticates a user and returns a JWT token.

Request:

````json
{
  "api_key": "your_api_key_here"
}
````

Response:

````json
{
  "token": "eyJhbGciOiJIUzI1...",
  "role": "user|admin",
  "expires_at": "2025-03-18T12:34:56Z"
}
````

### Model Management

#### **GET /list-models**

Lists all available models on the Ollama instance.

Request:
- No request body needed
- Requires JWT authentication header

Response:

````json
{
  "models": [
    {
      "name": "llama2",
      "size": "7B",
      "modified_at": "2023-10-15T14:22:31Z",
      "quantization": "Q4_0"
    },
    {
      "name": "mistral",
      "size": "7B",
      "modified_at": "2023-11-05T09:14:27Z",
      "quantization": "Q5_K_M"
    }
  ]
}
````

#### **POST /add-model** (Admin only)

Pulls a new model from the Ollama library.

Request:

````json
{
  "name": "llama2:7b",
  "pull_options": {
    "insecure": false
  }
}
````

Response:

````json
{
  "status": "success",
  "message": "Model llama2:7b successfully added",
  "model_info": {
    "name": "llama2",
    "size": "7B",
    "modified_at": "2023-10-15T14:22:31Z",
    "quantization": "Q4_0"
  }
}
````

#### **DELETE /delete-model** (Admin only)

Deletes a model from the Ollama instance.

Request:

````json
{
  "name": "llama2:7b"
}
````

Response:

````json
{
  "status": "success",
  "message": "Model llama2:7b successfully deleted"
}
````

### Text Generation

#### **POST /generate**

Generates text from a prompt using a specified model (non-streaming).

Request:

````json
{
  "model": "llama2:7b",
  "prompt": "Explain quantum computing in simple terms",
  "options": {
    "temperature": 0.7,
    "max_tokens": 500,
    "top_p": 0.9,
    "frequency_penalty": 0,
    "presence_penalty": 0
  }
}
````

Response:

````json
{
  "model": "llama2:7b",
  "created_at": "2023-10-16T12:34:56Z",
  "response": "Quantum computing is like regular computing but instead of using bits that are either 0 or 1, it uses quantum bits or 'qubits' that can be both 0 and 1 at the same time thanks to a property called superposition...",
  "done": true,
  "total_duration": 2415000000,
  "load_duration": 5261000,
  "prompt_eval_count": 13,
  "prompt_eval_duration": 85002000,
  "eval_count": 286,
  "eval_duration": 2324736000
}
````

#### **POST /generate-streaming**

Generates text from a prompt with streaming response.

Request:

````json
{
  "model": "llama2:7b",
  "prompt": "Write a short poem about AI",
  "options": {
    "temperature": 0.8,
    "max_tokens": 200,
    "top_p": 0.95,
    "frequency_penalty": 0.1,
    "presence_penalty": 0.1
  },
  "stream": true
}
````

Response: A stream of JSON objects with partial responses.

````json
{"response": "Digital", "done": false}
{"response": " minds", "done": false}
{"response": " of", "done": false}
{"response": " silicon", "done": false}
{"response": " dreams,\n", "done": false}
{"response": "Learning", "done": false}
{"response": " with", "done": false}
{"response": " each", "done": false}
{"response": " passing", "done": false}
{"response": " moment", "done": false}
{"response": ".\n", "done": false}
{"response": "A", "done": false}
{"response": " dance", "done": false}
{"response": " of", "done": false}
{"response": " numbers", "done": false}
{"response": ",\n", "done": false}
{"response": "Creating", "done": false}
{"response": " tomorrow", "done": false}
{"response": "'s", "done": false}
{"response": " wisdom", "done": false}
{"response": ".", "done": false}
{"response": "", "done": true, "total_duration": 1356000000, "load_duration": 4892000, "prompt_eval_count": 11, "prompt_eval_duration": 42003000, "eval_count": 132, "eval_duration": 1309105000}
````

### OCR (Optical Character Recognition)

#### **POST /ocr-image**

Extracts text from an image using Tesseract OCR.

Request:
- Multipart form data with a file field named "file"
- Accepts .png, .jpg, and .jpeg image formats 
- Optional "lang" query parameter (defaults to "en", also supports "pt")
- Requires JWT authentication header

Example:
```curl
curl -X POST http://localhost:3000/ocr-image?lang=en \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1..." \
  -F "file=@/path/to/your/image.jpg"
````

Response:

````json
{
  "text": "Extracted text from the image appears here.",
  "file_processed": "image.jpg"
}