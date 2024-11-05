#!/bin/bash

# List of example folders
EXAMPLES=(
  "realtime-chat"
  "reverse-proxy"
  "graceful-shutdown"
  "secure-web-app"
  "group-routes"
  "send_chunked_data"
  "grpc"
  "server-sent-event"
  "http-pusher"
  "struct-lvl-validations"
  "basic"
  "http2"
  "template"
  "cookie"
  "multiple-service"
  "upload-file"
  "custom-validation"
  "versioning"
  "favicon"
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

  # Check if the .diff file exists in the example folder
  if [ -f "$EXAMPLE_PATH/$DIFF_FILE" ]; then
    # Remove the .diff file from the example folder
    rm "$EXAMPLE_PATH/$DIFF_FILE"
    echo "Removed $DIFF_FILE from $EXAMPLE_PATH"
  else
    echo "$DIFF_FILE not found in $EXAMPLE_PATH. Skipping..."
  fi
done