## Create Claude LLM Provider Microservice

Create a core microservice named `claudellm` with hostname `claude.llm.core` that implements the `Turn` endpoint for the Claude (Anthropic) LLM provider. The implementation translates between the Microbus LLM message format (`llmapi.Message`, `llmapi.ToolDef`) and the Claude Messages API, using the HTTP egress proxy for outbound API calls. Types are imported from the `llmapi` package to ensure a uniform interface across provider microservices.
