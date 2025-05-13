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
    GET /llm/model/list
    Authorization: Bearer eyJhbGciOiJIUzI1...
    ````

### API Keys and Roles

*   **User API Key**: Standard access to model generation and listing
*   **Admin API Key**: Additional access to admin-only endpoints like adding models

API keys should be configured in the `.env` file, refer to the [example env](../example.env).

### Protected Endpoints

*   All endpoints except `/auth` require a valid JWT token
*   Admin operations (like `/llm/model/add`, `/llm/model/delete`, `/job/list`) require a token with admin role

---

## Endpoints

### Auth Endpoints

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

---

### LLM Endpoints

#### **POST /llm/generate**

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

#### **POST /llm/generate/streaming**

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

#### **POST /llm/model/add** *(Admin only)*

Pulls a new model from the Ollama library.

Request:

````json
{
  "model": "llama2:7b",
  "pull_options": {
    "insecure": false
  }
}
````

Response:

````json
{
  "status": "success"
}
````

#### **DELETE /llm/model/delete** *(Admin only)*

Deletes a model from the Ollama instance.

Request:

````json
{
  "model": "llama2:7b"
}
````

Response:

````json
{
  "status": "success",
  "message": "Model deleted successfully"
}
````

#### **GET /llm/model/list**

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

#### **POST /llm/multimodal/extract/image**

Extracts text from an image using multimodal LLMs.

Request:
- Multipart form data with a file field named "file"
- Accepts .png, .jpg, and .jpeg image formats 
- Required "model" query parameter (supported: "gemma3:4b", "llava:7b", "minicpm-v:8b")
- Requires JWT authentication header

Example:
```curl
curl -X POST http://localhost:3000/llm/multimodal/extract/image?model=gemma3:4b \
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

---

### Job Endpoints

#### **POST /job/generate**

Create an asynchronous job to generate a response.

Request:

````json
{
  "model": "llama2:7b",
  "prompt": "Explain quantum computing in simple terms",
  "options": {
    "temperature": 0.7,
    "max_tokens": 500
  }
}
````

Response:

````json
{
  "job_id": "job-123",
  "status": "pending"
}
````

#### **POST /job/multimodal/extract/image**

Create an asynchronous job to extract text from an image.

Request:
- Multipart form data with a file field named "file"
- Required "model" query parameter

Response:

````json
{
  "job_id": "job-456",
  "status": "pending"
}
````

#### **GET /job/:id/status**

Check asynchronous job status.

Response:

````json
{
  "created_at": "2025-05-11T20:30:44.0283394-03:00",
  "fulfilled_at": "2025-05-11T20:30:49.1500282-03:00",
  "job_id": "5e0b39c9-a62c-487b-8776-94bfe05f5c54",
  "job_type": "generate",
  "status": "fulfilled"
}
````

#### **GET /job/:id**

Retrieve asynchronous job and its result, if available.

Query parameter:
- `result=true` to include the result (if retrievable)

Response (fulfilled, with result):

````json
{
  "id": "5e0b39c9-a62c-487b-8776-94bfe05f5c54",
  "status": "fulfilled",
  "created_at": "2025-05-11T20:30:44.0283394-03:00",
  "fulfilled_at": "2025-05-11T20:30:49.1500282-03:00",
  "job_type": "generate",
  "prompt": "Who is the president of Brazil?",
  "model": "gemma3:1b",
  "result": "{\"model\":\"gemma3:1b\",\"response\":\"As of today, November 2, 2023, the President of Brazil is Luiz Inácio Lula da Silva, often referred to as Lula. \\n\\nIt’s always a good idea to double-check with a reliable news source for the very latest updates!\"}"
}
````

Response (fulfilled, without result):

````json
{
  "id": "5e0b39c9-a62c-487b-8776-94bfe05f5c54",
  "status": "fulfilled",
  "created_at": "2025-05-11T20:30:44.0283394-03:00",
  "fulfilled_at": "2025-05-11T20:30:49.1500282-03:00",
  "job_type": "generate",
  "prompt": "Who is the president of Brazil?",
  "model": "gemma3:1b"
}
````

Response (result expired):

````json
{
  "error": "Result expired"
}
````

Response (job not found):

````json
{
  "error": "Job not found"
}
````

#### **GET /job/:id/result**

Retrieve asynchronous job result.

Response (fulfilled):

````json
{
  "job_id": "job-123",
  "result": { /* result object, e.g. LLM output or OCR result */ }
}
````

Response (not fulfilled):

````json
{
  "status": "pending",
  "message": "Job not fulfilled yet"
}
````

Response (expired):

````json
{
  "error": "Result expired"
}
````

#### **GET /job/list** *(Admin only)*

List the last previous jobs.

Query parameters:
- `limit` (optional): number of jobs to return (default 10)
- `result` (optional, boolean): whether to include job results in the response (default: false)

If `result=true`, each job object will include a `result` field with the job's result (if available). If `result=false` (default), the `result` field is omitted.

Example response (`result=true`):

````json
{
    "jobs": [
        {
            "id": "5e0b39c9-a62c-487b-8776-94bfe05f5c54",
            "created_at": "2025-05-11T20:30:44.0283394-03:00",
            "fulfilled_at": "2025-05-11T20:30:49.1500282-03:00",
            "status": "fulfilled",
            "job_type": "generate",
            "input": "{\"model\":\"gemma3:1b\",\"prompt\":\"Who is the president of Brazil?\"}",
            "result": "{\"model\":\"gemma3:1b\",\"response\":\"As of today, November 2, 2023, the President of Brazil is Luiz Inácio Lula da Silva, often referred to as Lula. \\n\\nIt’s always a good idea to double-check with a reliable news source for the very latest updates!\"}"
        }
    ]
}
````

Example response (`result=false`):

````json
{
    "jobs": [
        {
            "id": "5e0b39c9-a62c-487b-8776-94bfe05f5c54",
            "created_at": "2025-05-11T20:30:44.0283394-03:00",
            "fulfilled_at": "2025-05-11T20:30:49.1500282-03:00",
            "status": "fulfilled",
            "job_type": "generate",
            "input": "{\"model\":\"gemma3:1b\",\"prompt\":\"Who is the president of Brazil?\"}"
        }
    ]
}
````

---