## Create Gemini LLM Provider Microservice

Create a core microservice named `geminillm` with hostname `gemini.llm.core` that implements the `Turn` endpoint for the Google Gemini generateContent API. The implementation translates between the Microbus LLM message format (`llmapi.Message`, `llmapi.ToolDef`) and the Gemini API, using the HTTP egress proxy for outbound API calls. Types are imported from the `llmapi` package to ensure a uniform interface across provider microservices.
