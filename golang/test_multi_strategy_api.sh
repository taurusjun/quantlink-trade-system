#!/bin/bash
# Multi-Strategy API Integration Test Script
# 多策略 API 集成测试脚本

set -e

API_HOST="${API_HOST:-localhost}"
API_PORT="${API_PORT:-9301}"
API_BASE="http://${API_HOST}:${API_PORT}/api/v1"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "════════════════════════════════════════════════════════════"
echo "  Multi-Strategy API Integration Test"
echo "  API: ${API_BASE}"
echo "════════════════════════════════════════════════════════════"
echo ""

# Test counter
PASSED=0
FAILED=0

# Helper function to test an endpoint
test_endpoint() {
    local method="$1"
    local endpoint="$2"
    local description="$3"
    local expected_field="$4"

    echo -n "Testing: ${description}... "

    if [ "$method" = "GET" ]; then
        response=$(curl -s -X GET "${API_BASE}${endpoint}")
    else
        response=$(curl -s -X POST "${API_BASE}${endpoint}")
    fi

    # Check if response contains success: true
    if echo "$response" | grep -q '"success":true'; then
        # Check for expected field if specified
        if [ -n "$expected_field" ]; then
            if echo "$response" | grep -q "$expected_field"; then
                echo -e "${GREEN}PASSED${NC}"
                ((PASSED++))
            else
                echo -e "${RED}FAILED${NC} (missing: $expected_field)"
                echo "Response: $response"
                ((FAILED++))
            fi
        else
            echo -e "${GREEN}PASSED${NC}"
            ((PASSED++))
        fi
    else
        echo -e "${RED}FAILED${NC}"
        echo "Response: $response"
        ((FAILED++))
    fi
}

# Helper function to test error response
test_error() {
    local method="$1"
    local endpoint="$2"
    local description="$3"

    echo -n "Testing: ${description}... "

    if [ "$method" = "GET" ]; then
        response=$(curl -s -X GET "${API_BASE}${endpoint}")
    else
        response=$(curl -s -X POST "${API_BASE}${endpoint}")
    fi

    # For error tests, we expect success: false
    if echo "$response" | grep -q '"success":false'; then
        echo -e "${GREEN}PASSED${NC} (expected error)"
        ((PASSED++))
    else
        echo -e "${YELLOW}UNEXPECTED${NC} (got success instead of error)"
        echo "Response: $response"
        ((FAILED++))
    fi
}

echo "────────────────────────────────────────────────────────────"
echo "1. Health & Status Endpoints"
echo "────────────────────────────────────────────────────────────"

test_endpoint "GET" "/health" "Health check" '"status":"ok"'
test_endpoint "GET" "/trader/status" "Trader status" '"running"'

echo ""
echo "────────────────────────────────────────────────────────────"
echo "2. Dashboard Overview"
echo "────────────────────────────────────────────────────────────"

test_endpoint "GET" "/dashboard/overview" "Dashboard overview" '"total_strategies"'
test_endpoint "GET" "/dashboard/overview" "Multi-strategy flag" '"multi_strategy"'

echo ""
echo "────────────────────────────────────────────────────────────"
echo "3. Strategies List"
echo "────────────────────────────────────────────────────────────"

test_endpoint "GET" "/strategies" "Strategies list" '"strategies"'
test_endpoint "GET" "/strategies" "Strategy count" '"count"'

echo ""
echo "────────────────────────────────────────────────────────────"
echo "4. Single Strategy Operations"
echo "────────────────────────────────────────────────────────────"

# Get first strategy ID from config (should be ag_pairwise for multi-strategy test)
test_endpoint "GET" "/strategies/ag_pairwise" "Get strategy details" '"id"'
test_endpoint "GET" "/strategies/cu_passive" "Get second strategy" '"id"'

echo ""
echo "────────────────────────────────────────────────────────────"
echo "5. Strategy Activation/Deactivation"
echo "────────────────────────────────────────────────────────────"

test_endpoint "POST" "/strategies/ag_pairwise/activate" "Activate strategy" '"active":true'
sleep 1
test_endpoint "POST" "/strategies/ag_pairwise/deactivate" "Deactivate strategy" '"active":false'

echo ""
echo "────────────────────────────────────────────────────────────"
echo "6. Realtime Indicators"
echo "────────────────────────────────────────────────────────────"

test_endpoint "GET" "/indicators/realtime" "Realtime indicators" '"strategies"'
test_endpoint "GET" "/indicators/realtime" "Timestamp field" '"timestamp"'

echo ""
echo "────────────────────────────────────────────────────────────"
echo "7. Positions"
echo "────────────────────────────────────────────────────────────"

test_endpoint "GET" "/positions" "Positions list" 'success'
test_endpoint "GET" "/positions/summary" "Positions summary" '"total_positions"'

echo ""
echo "────────────────────────────────────────────────────────────"
echo "8. Error Handling"
echo "────────────────────────────────────────────────────────────"

test_error "GET" "/strategies/nonexistent_strategy" "Non-existent strategy (expect 404)"
test_error "POST" "/strategies/nonexistent_strategy/activate" "Activate non-existent (expect error)"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  Test Summary"
echo "════════════════════════════════════════════════════════════"
echo -e "  Passed: ${GREEN}${PASSED}${NC}"
echo -e "  Failed: ${RED}${FAILED}${NC}"
echo "════════════════════════════════════════════════════════════"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
