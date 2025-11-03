#!/bin/bash
# Docker Build Test Script
# Tests all Docker image variants to ensure they build successfully

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="kleverapp/klever-go"
VERSION="${VERSION:-test}"
TEST_TAG_SUFFIX="-test-$(date +%s)"

# Statistics
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "INFO")
            echo -e "${BLUE}[INFO]${NC} $message"
            ;;
        "SUCCESS")
            echo -e "${GREEN}[✓]${NC} $message"
            ;;
        "ERROR")
            echo -e "${RED}[✗]${NC} $message"
            ;;
        "WARN")
            echo -e "${YELLOW}[!]${NC} $message"
            ;;
    esac
}

# Function to test a Docker build
test_build() {
    local name=$1
    local dockerfile=$2
    local tag=$3

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    print_status "INFO" "Testing: $name"
    print_status "INFO" "  Dockerfile: $dockerfile"
    print_status "INFO" "  Tag: $tag"

    # Check if Dockerfile exists
    if [ ! -f "$dockerfile" ]; then
        print_status "ERROR" "  Dockerfile not found: $dockerfile"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    # Attempt to build
    if docker build \
        --build-arg arg_version="$VERSION" \
        -t "$tag" \
        -f "$dockerfile" \
        . > /tmp/docker-build-$name.log 2>&1; then
        print_status "SUCCESS" "  Build succeeded: $name"
        PASSED_TESTS=$((PASSED_TESTS + 1))

        # Get image size
        local size=$(docker images "$tag" --format "{{.Size}}" | head -n1)
        print_status "INFO" "  Image size: $size"

        # Clean up test image
        docker rmi "$tag" > /dev/null 2>&1 || true

        return 0
    else
        print_status "ERROR" "  Build failed: $name"
        print_status "ERROR" "  Check logs: /tmp/docker-build-$name.log"
        tail -n 20 /tmp/docker-build-$name.log | sed 's/^/    /'
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

# Function to check prerequisites
check_prerequisites() {
    print_status "INFO" "Checking prerequisites..."

    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        print_status "ERROR" "Docker is not installed"
        exit 1
    fi

    # Check if Docker is running
    if ! docker info &> /dev/null; then
        print_status "ERROR" "Docker daemon is not running"
        exit 1
    fi

    # Check if we're in the right directory
    if [ ! -f "docker/Dockerfile" ]; then
        print_status "ERROR" "Please run this script from the project root directory"
        exit 1
    fi

    # Check if go.mod exists
    if [ ! -f "go.mod" ]; then
        print_status "ERROR" "go.mod not found - not a Go project?"
        exit 1
    fi

    print_status "SUCCESS" "Prerequisites check passed"
    echo ""
}

# Main test execution
main() {
    print_status "INFO" "====================================="
    print_status "INFO" "Docker Build Test Suite"
    print_status "INFO" "====================================="
    echo ""

    check_prerequisites

    print_status "INFO" "Starting Docker image build tests..."
    print_status "INFO" "Version: $VERSION"
    echo ""

    # Test Debian-based images
    print_status "INFO" "-------------------------------------"
    print_status "INFO" "Testing Debian-based images"
    print_status "INFO" "-------------------------------------"
    echo ""

    test_build "debian-full" \
        "docker/Dockerfile" \
        "$REPO:$VERSION$TEST_TAG_SUFFIX"
    echo ""

    test_build "debian-validator" \
        "docker/Dockerfile.validator" \
        "$REPO:val-$VERSION$TEST_TAG_SUFFIX"
    echo ""

    # Test Alpine-based images
    print_status "INFO" "-------------------------------------"
    print_status "INFO" "Testing Alpine-based images"
    print_status "INFO" "-------------------------------------"
    echo ""

    test_build "alpine-full" \
        "docker/Dockerfile.alpine" \
        "$REPO:alpine-$VERSION$TEST_TAG_SUFFIX"
    echo ""

    test_build "alpine-validator" \
        "docker/Dockerfile.alpine.validator" \
        "$REPO:alpine-val-$VERSION$TEST_TAG_SUFFIX"
    echo ""

    # Print summary
    print_status "INFO" "====================================="
    print_status "INFO" "Test Summary"
    print_status "INFO" "====================================="
    echo ""
    print_status "INFO" "Total tests:   $TOTAL_TESTS"
    print_status "SUCCESS" "Passed tests:  $PASSED_TESTS"

    if [ $FAILED_TESTS -gt 0 ]; then
        print_status "ERROR" "Failed tests:  $FAILED_TESTS"
    else
        print_status "INFO" "Failed tests:  $FAILED_TESTS"
    fi

    if [ $SKIPPED_TESTS -gt 0 ]; then
        print_status "WARN" "Skipped tests: $SKIPPED_TESTS"
    fi

    echo ""

    # Exit with appropriate code
    if [ $FAILED_TESTS -gt 0 ]; then
        print_status "ERROR" "Some tests failed!"
        exit 1
    else
        print_status "SUCCESS" "All tests passed!"
        exit 0
    fi
}

# Run main function
main "$@"
