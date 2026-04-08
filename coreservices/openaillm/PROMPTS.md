## Create OpenAI LLM Provider Microservice

Create a core microservice named `openaillm` with hostname `openai.llm.core` that implements the `Turn` endpoint for the OpenAI Chat Completions API. The implementation translates between the Microbus LLM message format (`llmapi.Message`, `llmapi.ToolDef`) and the OpenAI API, using the HTTP egress proxy for outbound API calls. Types are imported from the `llmapi` package to ensure a uniform interface across provider microservices.
