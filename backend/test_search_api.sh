#!/bin/bash
# ============================================================================
# Search API Test Script
# ============================================================================
# Tests all search endpoints
# Usage: ./test_search_api.sh [BASE_URL]
# Example: ./test_search_api.sh http://localhost:8080
# ============================================================================

# Configuration
BASE_URL="${1:-http://localhost:8080}"
API_V1="$BASE_URL/api/v1"

echo "================================================"
echo "SmartTransit Search API Test Script"
echo "================================================"
echo "Base URL: $BASE_URL"
echo "================================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name=$1
    local endpoint=$2
    local method=$3
    local data=$4

    TESTS_RUN=$((TESTS_RUN + 1))
    echo -e "${YELLOW}Test $TESTS_RUN:${NC} $test_name"
    echo "Endpoint: $method $endpoint"

    if [ "$method" == "POST" ]; then
        response=$(curl -s -w "\n%{http_code}" -X POST "$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data")
    else
        response=$(curl -s -w "\n%{http_code}" "$endpoint")
    fi

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    echo "Status: $http_code"
    echo "Response:"
    echo "$body" | jq '.' 2>/dev/null || echo "$body"

    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        echo -e "${GREEN}✓ PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi

    echo "------------------------------------------------"
    echo ""
}

# ============================================================================
# Test 1: Health Check
# ============================================================================
run_test \
    "Search Service Health Check" \
    "$API_V1/search/health" \
    "GET"

# ============================================================================
# Test 2: Basic Search (Replace with actual stop names from your DB)
# ============================================================================
run_test \
    "Basic Trip Search - Colombo to Kandy" \
    "$API_V1/search" \
    "POST" \
    '{
        "from": "Colombo Fort",
        "to": "Kandy"
    }'

# ============================================================================
# Test 3: Search with DateTime
# ============================================================================
TOMORROW=$(date -u -d "+1 day" +"%Y-%m-%dT08:00:00Z" 2>/dev/null || date -u -v+1d +"%Y-%m-%dT08:00:00Z")
run_test \
    "Search with DateTime Filter" \
    "$API_V1/search" \
    "POST" \
    "{
        \"from\": \"Colombo Fort\",
        \"to\": \"Kandy\",
        \"datetime\": \"$TOMORROW\",
        \"limit\": 10
    }"

# ============================================================================
# Test 4: Search with Invalid Stop
# ============================================================================
run_test \
    "Search with Invalid Stop Name" \
    "$API_V1/search" \
    "POST" \
    '{
        "from": "InvalidPlaceThatDoesNotExist",
        "to": "Kandy"
    }'

# ============================================================================
# Test 5: Popular Routes
# ============================================================================
run_test \
    "Get Popular Routes" \
    "$API_V1/search/popular?limit=10" \
    "GET"

# ============================================================================
# Test 6: Autocomplete - Colombo
# ============================================================================
run_test \
    "Autocomplete for 'Colombo'" \
    "$API_V1/search/autocomplete?q=Colombo&limit=10" \
    "GET"

# ============================================================================
# Test 7: Autocomplete - Kandy
# ============================================================================
run_test \
    "Autocomplete for 'Kandy'" \
    "$API_V1/search/autocomplete?q=Kandy&limit=5" \
    "GET"

# ============================================================================
# Test 8: Autocomplete - Short Query (should return minimal results)
# ============================================================================
run_test \
    "Autocomplete with Short Query" \
    "$API_V1/search/autocomplete?q=C&limit=5" \
    "GET"

# ============================================================================
# Test 9: Search with Missing Parameters
# ============================================================================
run_test \
    "Search with Missing 'to' Parameter (Should Fail)" \
    "$API_V1/search" \
    "POST" \
    '{
        "from": "Colombo Fort"
    }'

# ============================================================================
# Test 10: Search with Same Origin and Destination
# ============================================================================
run_test \
    "Search with Same From/To (Should Fail)" \
    "$API_V1/search" \
    "POST" \
    '{
        "from": "Colombo Fort",
        "to": "Colombo Fort"
    }'

# ============================================================================
# Test Results Summary
# ============================================================================
echo "================================================"
echo "TEST RESULTS SUMMARY"
echo "================================================"
echo "Total Tests:  $TESTS_RUN"
echo -e "Passed:       ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed:       ${RED}$TESTS_FAILED${NC}"
echo "================================================"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}ALL TESTS PASSED! ✓${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Test with real stop names from your database"
    echo "2. Verify search results are accurate"
    echo "3. Check search_logs table for analytics"
    echo "4. Integrate with Flutter app"
    exit 0
else
    echo -e "${RED}SOME TESTS FAILED! ✗${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "1. Check if backend server is running"
    echo "2. Verify database migration was successful"
    echo "3. Ensure test data exists in database"
    echo "4. Check backend logs for errors"
    exit 1
fi
