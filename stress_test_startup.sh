#!/bin/bash

echo "🚀 Starting Load Balancer Stress Test Suite..."

# Try to increase the file descriptor limit so we can handle 20k connections
ulimit -n 65535 2>/dev/null || echo "⚠️ Could not increase ulimit (run as sudo/root for max connections)"

# Trap SIGINT (Ctrl+C) to clean up all background jobs so ports aren't left hanging
trap 'echo -e "\n🛑 Stopping all services..."; kill $(jobs -p) 2>/dev/null; exit 0' SIGINT SIGTERM

echo "[1/3] Starting Backend Servers..."
go run cmd/backend/server.go -port 8081 & 
go run cmd/backend/server.go -port 8082 &

echo "[2/3] Starting Maglev Load Balancer..."
go run cmd/lb/main.go &

# Give the servers a moment to bind to their ports
sleep 2
echo "----------------------------------------"

echo "[3/3] Starting Stress Test..."
go run cmd/stresstest/main.go -target 127.0.0.1:8080 -rate 500 -max 20000

# Wait indefinitely until the user presses Ctrl+C
wait