#!/bin/bash

# List of example folders
EXAMPLES=(
  "realtime-chat"
  "reverse-proxy"
  "graceful-shutdown"
  "secure-web-app"
  "app-engine"
  "group-routes"
  "send_chunked_data"
  "assets-in-binary"
  "grpc"
  "server-sent-event"
  "auto-tls"
  "http-pusher"
  "struct-lvl-validations"
  "basic"
  "http2"
  "template"
  "cookie"
  "multiple-service"
  "upload-file"
  "custom-validation"
  "new_relic"
  "versioning"
  "favicon"
  "otel"
  "websocket"
  "file-binding"
  "ratelimiter"
  "forward-proxy"
  "realtime-advanced"
)

DIFF_FILE="new-relic-instrumentation.diff"

# Iterate over each example folder
for EXAMPLE_NAME in "${EXAMPLES[@]}"; do
  EXAMPLE_PATH="./end-to-end-tests/gin-examples/$EXAMPLE_NAME"
  
  # Check if the example path exists
  if [ ! -d "$EXAMPLE_PATH" ]; then
    echo "Directory $EXAMPLE_PATH does not exist. Skipping..."
    continue
  fi

  # Run the Go command
  go run . -path "$EXAMPLE_PATH"
  
  # Check if the Go command failed due to a missing package
  if [ $? -ne 0 ]; then
    echo "Go command failed for $EXAMPLE_NAME. Attempting to resolve missing packages..."
    
    # Attempt to resolve missing packages
    go mod tidy
    
    # Retry the Go command
    go run . -path "$EXAMPLE_PATH"
    
    # Check if the retry was successful
    if [ $? -ne 0 ]; then
      echo "Go command failed again for $EXAMPLE_NAME. Skipping..."
      continue
    fi
  fi

  # Check if the .diff file was generated
  if [ ! -f "$DIFF_FILE" ]; then
    echo "Error: $DIFF_FILE not found in the current directory for $EXAMPLE_NAME."
    continue
  fi

  # Move the .diff file to the example folder
  mv "$DIFF_FILE" "$EXAMPLE_PATH"

  echo "Moved $DIFF_FILE to $EXAMPLE_PATH"
done