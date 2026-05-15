// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package transport

import (
	"context"
	"errors"
	"net"
	"runtime"
	"testing"
	"time"
)

// TestSendReceiveLoopback wires two UDPTransport instances on 127.0.0.1
// and confirms a payload sent by one is received by the other.
func TestSendReceiveLoopback(t *testing.T) {
	rx := NewUDPTransport("127.0.0.1:0")
	if err := rx.Open(context.Background()); err != nil {
		t.Fatalf("rx.Open: %v", err)
	}
	defer rx.Close()

	tx := NewUDPTransport("127.0.0.1:0")
	if err := tx.Open(context.Background()); err != nil {
		t.Fatalf("tx.Open: %v", err)
	}
	defer tx.Close()

	rxAddr := rx.LocalAddr().(*net.UDPAddr)

	payload := []byte{0x81, 0x0a, 0x00, 0x11, 0xde, 0xad, 0xbe, 0xef}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := tx.Send(ctx, rxAddr, payload); err != nil {
		t.Fatalf("tx.Send: %v", err)
	}

	got, _, err := rx.Receive(ctx)
	if err != nil {
		t.Fatalf("rx.Receive: %v", err)
	}
	if !bytesEqual(got, payload) {
		t.Fatalf("payload mismatch: got %x, want %x", got, payload)
	}
}

// TestCloseDuringRead verifies that Close() on an open transport unblocks
// a concurrent Receive() call. Without this guarantee, a graceful client
// shutdown would deadlock waiting for the receiver loop to exit.
func TestCloseDuringRead(t *testing.T) {
	tr := NewUDPTransport("127.0.0.1:0")
	if err := tr.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Block on Receive in a goroutine with a long deadline. Close() must
	// kick it out faster than the deadline would.
	gotErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _, err := tr.Receive(ctx)
		gotErr <- err
	}()

	// Give the goroutine a moment to enter the syscall.
	time.Sleep(50 * time.Millisecond)

	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case err := <-gotErr:
		// Any error is fine; the contract is "Receive unblocks". Verify
		// it's a "use of closed network connection" rather than a deadline.
		var nerr net.Error
		if errors.As(err, &nerr) && nerr.Timeout() {
			t.Fatalf("Receive returned a timeout error, expected closed-conn error: %v", err)
		}
	case <-time.After(2 * time.Second):
		// One stack dump on failure helps tell stuck-in-Read from stuck-elsewhere.
		var buf [4096]byte
		n := runtime.Stack(buf[:], true)
		t.Fatalf("Receive did not unblock within 2s after Close.\nGoroutine stacks:\n%s", buf[:n])
	}
}

// TestCloseIdempotent confirms Close() is safe to call multiple times on
// an open and on an already-closed transport.
func TestCloseIdempotent(t *testing.T) {
	tr := NewUDPTransport("127.0.0.1:0")
	if err := tr.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	if !tr.IsClosed() {
		t.Fatal("IsClosed() returned false after Close")
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
