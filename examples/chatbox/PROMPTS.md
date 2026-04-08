## Create Chatbox Demo LLM Provider

Create an example microservice named `chatbox` with hostname `chatbox.example` that implements the `Turn` endpoint as a demo LLM provider. It pattern-matches math questions (e.g., "what is 6 * 7?") and generates tool calls to the calculator service. It also serves a `/demo` web page that drives the LLM service with the chatbox as the provider, demonstrating the tool-calling flow end-to-end without a real API key.
