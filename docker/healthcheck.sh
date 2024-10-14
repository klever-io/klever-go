#!/bin/bash

# Configurable variables with defaults
LOG_DIR=${LOG_DIR:-"/opt/klever-blockchain/logs"}
STALE_THRESHOLD=${STALE_THRESHOLD:-20}
INVALID_NONCE_THRESHOLD=${INVALID_NONCE_THRESHOLD:-300}  # 5 minutes in seconds

NONCE_FILE="/tmp/klever_last_nonce"
LAST_UPDATE_FILE="/tmp/klever_last_update_time"
INVALID_NONCE_START_FILE="/tmp/klever_invalid_nonce_start"

# Function to get the latest nonce and timestamp
get_latest_nonce_and_time() {
    latest_log=$(find "$LOG_DIR" -name "*.log" -type f -print0 | xargs -0 ls -t | head -n1)
    if [ -n "$latest_log" ]; then
        log_line=$(tac "$latest_log" | grep -m 1 "meta block has been committed successfully")
        nonce=$(echo "$log_line" | grep -oP "nonce = \K\d+")
        timestamp=$(echo "$log_line" | grep -oP "\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}")
        echo "$nonce $timestamp"
    else
        echo "0 $(date +"%Y-%m-%d %H:%M:%S")"
    fi
}

current_time=$(date +%s)
read current_nonce timestamp <<< $(get_latest_nonce_and_time)

# Ensure current_nonce is a number
if ! [[ "$current_nonce" =~ ^[0-9]+$ ]]; then
    if [ ! -f "$INVALID_NONCE_START_FILE" ]; then
        echo "$current_time" > "$INVALID_NONCE_START_FILE"
        echo "Error: Invalid nonce value. Starting invalid nonce timer."
        exit 1
    else
        invalid_nonce_start=$(cat "$INVALID_NONCE_START_FILE")
        invalid_nonce_duration=$((current_time - invalid_nonce_start))
        if [ $invalid_nonce_duration -ge $INVALID_NONCE_THRESHOLD ]; then
            echo "Error: Invalid nonce persisted for over $((INVALID_NONCE_THRESHOLD / 60)) minutes. Healthcheck failed."
            exit 1
        else
            echo "Error: Invalid nonce value. Healthcheck failed. Duration: $invalid_nonce_duration seconds."
            exit 1
        fi
    fi
else
    # If we get a valid nonce, remove the invalid nonce start file
    rm -f "$INVALID_NONCE_START_FILE"
fi

if [ ! -f "$NONCE_FILE" ] || [ ! -f "$LAST_UPDATE_FILE" ]; then
    echo "$current_nonce" > "$NONCE_FILE"
    echo "$current_time" > "$LAST_UPDATE_FILE"
    echo "First run or files missing. Healthcheck passed."
    exit 0
fi

last_nonce=$(cat "$NONCE_FILE")
last_update=$(cat "$LAST_UPDATE_FILE")

# Ensure last_nonce and last_update are numbers
if ! [[ "$last_nonce" =~ ^[0-9]+$ ]] || ! [[ "$last_update" =~ ^[0-9]+$ ]]; then
    echo "Error: Invalid stored nonce or update time. Resetting."
    echo "$current_nonce" > "$NONCE_FILE"
    echo "$current_time" > "$LAST_UPDATE_FILE"
    exit 0
fi

time_since_last_update=$((current_time - last_update))

if [ "$time_since_last_update" -gt "$STALE_THRESHOLD" ]; then
    # If the healthcheck wasn't called within the stale threshold, reset as first run
    echo "$current_nonce" > "$NONCE_FILE"
    echo "$current_time" > "$LAST_UPDATE_FILE"
    echo "Healthcheck was stale, treated as first run. Nonce and timestamp updated."
    exit 0
elif [ "$current_nonce" -gt "$last_nonce" ]; then
    echo "$current_nonce" > "$NONCE_FILE"
    echo "$current_time" > "$LAST_UPDATE_FILE"
    echo "Status: Healthy - Chain is progressing (Nonce: $current_nonce, Last update: $timestamp)"
    exit 0
else
    echo "Status: Healthy - Last update within stale threshold (Time since last update: ${time_since_last_update}s, Threshold: ${STALE_THRESHOLD}s)"
    exit 0
fi