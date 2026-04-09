## Create ChatGPT LLM Provider Microservice

Create a core microservice named `chatgptllm` with hostname `chatgpt.llm.core` that implements the `Turn` endpoint for the OpenAI Chat Completions API. The implementation translates between the Microbus LLM message format (`llmapi.Message`, `llmapi.Tool`) and the OpenAI API, using the HTTP egress proxy for outbound API calls. Types are imported from the `llmapi` package to ensure a uniform interface across provider microservices.
