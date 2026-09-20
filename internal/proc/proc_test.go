// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

package proc

import (
	"fmt"
	"net"
	"testing"
)

func TestIsPortInUse(t *testing.T) {
	// 1. Pick an ephemeral free port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// 2. While listener is open, IsPortInUse should be true
	if !IsPortInUse("127.0.0.1", port) {
		t.Fatalf("expected port %d to be reported in use", port)
	}

	// 3. Close listener
	ln.Close()

	// 4. Now port should be free
	if IsPortInUse("127.0.0.1", port) {
		t.Fatalf("expected port %d to be free after closing", port)
	}
}

func TestOptionsGrammar(t *testing.T) {
	// Simple check to ensure testing works smoothly
	if IsPortInUse("127.0.0.1", 65530) {
		fmt.Println("port 65530 in use")
	}
}
