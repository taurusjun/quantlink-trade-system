#!/bin/bash
# HTTP REST API Control Script for QuantlinkTrader
# Alternative to Unix signal control (startTrade.sh/stopTrade.sh)
# 提供现代化的 HTTP API 控制接口

# Parse arguments
COMMAND=$1
STRATEGY_ID=$2
API_PORT=${3:-9201}  # Default to 9201

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

usage() {
    echo "Usage: $0 <command> <strategy_id> [api_port]"
    echo ""
    echo "Commands:"
    echo "  activate      - Activate strategy (start trading)"
    echo "  deactivate    - Deactivate strategy (squareoff)"
    echo "  status        - Get strategy status"
    echo "  trader-status - Get trader status"
    echo "  health        - Get health check"
    echo ""
    echo "Examples:"
    echo "  $0 activate 92201           # Activate strategy 92201 on port 9201"
    echo "  $0 deactivate 92201         # Deactivate strategy 92201"
    echo "  $0 status 92201             # Get strategy 92201 status"
    echo "  $0 activate 93201 9301      # Activate strategy 93201 on port 9301"
    echo ""
    exit 1
}

if [ -z "$COMMAND" ]; then
    usage
fi

# Base URL
BASE_URL="http://localhost:${API_PORT}/api/v1"

case "$COMMAND" in
    activate)
        if [ -z "$STRATEGY_ID" ]; then
            echo -e "${RED}Error: strategy_id required for activate command${NC}"
            usage
        fi

        echo "════════════════════════════════════════════════════════════"
        echo "Activating Strategy via HTTP API"
        echo "════════════════════════════════════════════════════════════"
        echo "Strategy ID: $STRATEGY_ID"
        echo "API Port:    $API_PORT"
        echo ""

        RESPONSE=$(curl -s -X POST "${BASE_URL}/strategy/activate")

        if [ $? -eq 0 ]; then
            echo "$RESPONSE" | jq '.'
            if [ $(echo "$RESPONSE" | jq -r '.success') == "true" ]; then
                echo ""
                echo -e "${GREEN}✓ Strategy activated successfully${NC}"
            else
                echo ""
                echo -e "${RED}✗ Failed to activate strategy${NC}"
                exit 1
            fi
        else
            echo -e "${RED}✗ Failed to connect to API server${NC}"
            echo "Make sure the trader is running and API is enabled"
            exit 1
        fi
        ;;

    deactivate)
        if [ -z "$STRATEGY_ID" ]; then
            echo -e "${RED}Error: strategy_id required for deactivate command${NC}"
            usage
        fi

        echo "════════════════════════════════════════════════════════════"
        echo "Deactivating Strategy via HTTP API (Squareoff)"
        echo "════════════════════════════════════════════════════════════"
        echo "Strategy ID: $STRATEGY_ID"
        echo "API Port:    $API_PORT"
        echo ""

        RESPONSE=$(curl -s -X POST "${BASE_URL}/strategy/deactivate")

        if [ $? -eq 0 ]; then
            echo "$RESPONSE" | jq '.'
            if [ $(echo "$RESPONSE" | jq -r '.success') == "true" ]; then
                echo ""
                echo -e "${GREEN}✓ Strategy deactivated successfully${NC}"
            else
                echo ""
                echo -e "${RED}✗ Failed to deactivate strategy${NC}"
                exit 1
            fi
        else
            echo -e "${RED}✗ Failed to connect to API server${NC}"
            exit 1
        fi
        ;;

    status)
        RESPONSE=$(curl -s -X GET "${BASE_URL}/strategy/status")

        if [ $? -eq 0 ]; then
            echo "$RESPONSE" | jq '.'
        else
            echo -e "${RED}✗ Failed to connect to API server${NC}"
            exit 1
        fi
        ;;

    trader-status)
        RESPONSE=$(curl -s -X GET "${BASE_URL}/trader/status")

        if [ $? -eq 0 ]; then
            echo "$RESPONSE" | jq '.'
        else
            echo -e "${RED}✗ Failed to connect to API server${NC}"
            exit 1
        fi
        ;;

    health)
        RESPONSE=$(curl -s -X GET "${BASE_URL}/health")

        if [ $? -eq 0 ]; then
            echo "$RESPONSE" | jq '.'
            if [ $(echo "$RESPONSE" | jq -r '.success') == "true" ]; then
                echo ""
                echo -e "${GREEN}✓ Trader is healthy${NC}"
            fi
        else
            echo -e "${RED}✗ Failed to connect to API server${NC}"
            exit 1
        fi
        ;;

    *)
        echo -e "${RED}Error: Unknown command '$COMMAND'${NC}"
        usage
        ;;
esac

echo "════════════════════════════════════════════════════════════"
