#!/bin/bash

SERVER_CONTAINER_NAME="server"
SERVER_PORT="12345"
NETWORK_NAME="tp0_testing_net"
TEST_MESSAGE="Hello, Server!"
RESULT="fail"

RESPONSE=$(docker run --rm --network $NETWORK_NAME alpine sh -c "echo $TEST_MESSAGE | nc $SERVER_CONTAINER_NAME $SERVER_PORT")

if [ "$RESPONSE" = "$TEST_MESSAGE" ]; then
  RESULT="success"
fi

echo "action: test_echo_server | result: $RESULT"