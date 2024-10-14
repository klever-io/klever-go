#!/bin/bash
set -e

# Function to check and fix permissions if necessary
check_permissions() {
    local dir="$1"
    if [ ! -w "$dir" ]; then
        echo "Warning: Directory $dir is not writable. Attempting to fix permissions..." >&2
        if ! sudo chown klever:klever "$dir"; then
            echo "Error: Failed to change ownership of $dir. Please check your volume mounts." >&2
            return 1
        fi
        echo "Permissions fixed for $dir." >&2
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
        chown klever:klever "$full_path"
        chmod 2750 "$full_path"
    fi
done

# If the user is root, switch to the klever user
if [ "$(id -u)" = "0" ]; then
    echo "Switching to klever user..."
    exec gosu klever "$@"
else
    exec "$@"
fi