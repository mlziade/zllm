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
    GET /llms/models/list
    Authorization: Bearer eyJhbGciOiJIUzI1...
    ````

### API Keys and Roles

*   **User API Key**: Standard access to model generation and listing
*   **Admin API Key**: Additional access to admin-only endpoints like adding models

API keys should be configured in the `.env` file, refer to the [example env](../example.env).

### Protected Endpoints

*   All endpoints except `/auth` require a valid JWT token
*   Admin operations (like `/llms/models/add`) require a token with admin role

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

### LLM Endpoints

#### **GET /llms/models/list**

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

#### **POST /llms/models/add** (Admin only)

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

#### **DELETE /llms/models/delete** (Admin only)

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

#### **POST /llms/generate**

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

#### **POST /llms/generate-streaming**

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
{"model":"gemma3:1b","created_at":"2025-05-11T03:35:22.7883246Z","response":" **","done":false}
{"response": "Digital", "done": false}
{"response": " minds", "done": false}
{"response": " wisdom", "done": false}
{"response": ".", "done": false}
{"model": "gemma3:1b", "created_at": "2025-05-11T03:35:51.9490465Z", "response": "", "done": true, "done_reason": "stop", "context": [105, 2364, 107, 155122, 531, 786, 528, 4889, 1217, 531, 1138, 14470, 573, 2802, 528, 496, 23381, 5941, 236881, 106, 107], "total_duration": 73945015500, "load_duration": 4091883200, "prompt_eval_count": 25, "prompt_eval_duration": 361034000, "eval_count": 1604, "eval_duration": 69489587500}
````

### OCR Endpoints

#### **POST /ocr/extract/image**

Extracts text from an image using multimodal LLMs.

Request:
- Multipart form data with a file field named "file"
- Accepts .png, .jpg, and .jpeg image formats 
- Optional "model" query parameter (defaults to "gemma3:4b")
- Supported models: "gemma3:4b", "llava:7b", "minicpm-v:8b"
- Requires JWT authentication header

Example:
```curl
curl -X POST http://localhost:3000/ocr/extract/image?model=gemma3:4b \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1..." \
  -F "file=@/path/to/your/image.jpg"
````

Response:

````json
{
  "original_text": "Extracted text from the image appears here.",
  "file_processed": "image.jpg",
  "model": "gemma3:4b"
}
````

Error Response (Unsupported Model):

````json
{
  "error": "Unsupported model. Use one of the supported multimodal models.",
  "supported_models": ["gemma3:4b", "llava:7b", "minicpm-v:8b"]
}
````

Error Response (Unsupported File Type):

````json
{
  "error": "Unsupported file type. Only .png, .jpg, and .jpeg images are supported"
}
````

Note: If the LLM response cannot be parsed as structured JSON, you'll receive:

````json
{
  "warning": "Could not parse structured data from LLM response",
  "raw_response": "LLM's raw text response",
  "file_processed": "image.jpg",
  "model": "gemma3:4b"
}
````