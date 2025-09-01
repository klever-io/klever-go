#!/bin/bash

# Script to help visualize and analyze Klever consensus traces

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Klever Consensus Trace Analyzer ===${NC}"
echo

# Function to show usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  -f FILE    Analyze trace file"
    echo "  -l         List recent trace files"
    echo "  -s         Show summary of traces"
    echo "  -t         Show trace tree view"
    echo "  -c         Show consensus statistics"
    echo "  -o N       Show N slowest operations"
    echo "  -a         Show all analysis views"
    echo "  -h         Show this help"
    echo
    echo "Examples:"
    echo "  $0 -f trace.json -a           # Analyze trace file with all views"
    echo "  $0 -l                          # List recent trace files"
    echo "  $0 -f trace.json -o 10         # Show 10 slowest operations"
    exit 1
}

# Parse arguments
TRACE_FILE=""
SHOW_LIST=false
SHOW_SUMMARY=false
SHOW_TREE=false
SHOW_STATS=false
SHOW_SLOW=0
SHOW_ALL=false

while getopts "f:lstco:ah" opt; do
    case $opt in
        f) TRACE_FILE="$OPTARG" ;;
        l) SHOW_LIST=true ;;
        s) SHOW_SUMMARY=true ;;
        t) SHOW_TREE=true ;;
        c) SHOW_STATS=true ;;
        o) SHOW_SLOW="$OPTARG" ;;
        a) SHOW_ALL=true ;;
        h) usage ;;
        *) usage ;;
    esac
done

# Build the analyzer if needed
ANALYZER="/tmp/trace-analyze"
if [ ! -f "$ANALYZER" ]; then
    echo -e "${YELLOW}Building trace analyzer...${NC}"
    go build -o "$ANALYZER" ./tools/tracing/cmd/analyze
    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to build analyzer${NC}"
        exit 1
    fi
fi

# List recent trace files
if [ "$SHOW_LIST" = true ]; then
    echo -e "${GREEN}Recent trace files:${NC}"
    find . -name "trace*.json" -type f -mtime -1 -exec ls -lh {} \; 2>/dev/null | tail -10
    echo
    find ./traces -name "*.json" -type f -mtime -1 -exec ls -lh {} \; 2>/dev/null | tail -10
    exit 0
fi

# If no file specified, try to find the most recent one
if [ -z "$TRACE_FILE" ]; then
    TRACE_FILE=$(find . -name "trace*.json" -type f -mtime -1 2>/dev/null | head -1)
    if [ -z "$TRACE_FILE" ]; then
        TRACE_FILE=$(find ./traces -name "*.json" -type f -mtime -1 2>/dev/null | head -1)
    fi
    
    if [ -z "$TRACE_FILE" ]; then
        echo -e "${RED}No trace file found. Use -f to specify a file or -l to list available files.${NC}"
        exit 1
    fi
    
    echo -e "${YELLOW}Using recent trace file: $TRACE_FILE${NC}"
    echo
fi

# Check if file exists
if [ ! -f "$TRACE_FILE" ]; then
    echo -e "${RED}Error: File '$TRACE_FILE' not found${NC}"
    exit 1
fi

# Run the analyzer with appropriate options
ANALYZER_OPTS=""

if [ "$SHOW_ALL" = true ]; then
    ANALYZER_OPTS="-all"
else
    if [ "$SHOW_SUMMARY" = true ] || [ "$SHOW_TREE" = false ] && [ "$SHOW_STATS" = false ] && [ "$SHOW_SLOW" -eq 0 ]; then
        ANALYZER_OPTS="$ANALYZER_OPTS -summary"
    fi
    
    if [ "$SHOW_TREE" = true ]; then
        ANALYZER_OPTS="$ANALYZER_OPTS -tree"
    fi
    
    if [ "$SHOW_STATS" = true ]; then
        ANALYZER_OPTS="$ANALYZER_OPTS -stats"
    fi
    
    if [ "$SHOW_SLOW" -gt 0 ]; then
        ANALYZER_OPTS="$ANALYZER_OPTS -slow $SHOW_SLOW"
    fi
fi

# Run the analyzer
echo -e "${GREEN}Analyzing: $TRACE_FILE${NC}"
echo
$ANALYZER -file "$TRACE_FILE" $ANALYZER_OPTS

# Show tips
echo
echo -e "${BLUE}Tips:${NC}"
echo "  • Use -a to see all analysis views"
echo "  • Use -o 20 to see the 20 slowest operations"
echo "  • Export traces with: KLEVER_TRACING_SAVE_ON_EXIT=true"
echo "  • View in Zipkin UI: Upload the JSON file at http://localhost:9411"