#!/bin/bash
set -e

# Function to check and fix permissions if necessary
check_permissions() {
    local dir="$1"
    if [ ! -w "$dir" ]; then
        echo "Warning: Directory $dir is not writable. Attempting to fix permissions..." >&2
        # In Alpine with su-exec, we can't escalate privileges
        # This warning helps identify permission issues
        echo "Error: Cannot fix permissions without root. Please ensure volume mounts have correct ownership (UID/GID 999)." >&2
        return 1
    fi
    return 0
}

# Check and fix permissions for necessary directories
for dir in ${KLEVER_DIRS}; do
    full_path="${KLEVER_HOME}/${dir}"
    if [ -d "$full_path" ]; then
        if ! check_permissions "$full_path"; then
            exit 1
        fi
    else
        echo "Warning: Directory $full_path does not exist. Creating it..." >&2
        mkdir -p "$full_path"
        chmod 0750 "$full_path"
    fi
done

# If the user is root, switch to the klever user using su-exec
if [ "$(id -u)" = "0" ]; then
    echo "Switching to klever user..."
    exec su-exec klever "$@"
else
    exec "$@"
fi
