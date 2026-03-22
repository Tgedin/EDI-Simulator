#!/bin/sh
# Start Ollama server in background, pull the model, then wait.
/bin/ollama serve &
sleep 5
/bin/ollama pull qwen2.5:3b
wait
