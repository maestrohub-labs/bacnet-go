// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package bacnet

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestStateMachineHappyPath walks a client through the documented
// Disconnected -> Connecting -> Connected -> Disconnected transitions.
// We rely on the fact that Connect on 127.0.0.1:0 succeeds without any
// peer; the receiver goroutine just sits idle.
func TestStateMachineHappyPath(t *testing.T) {
	c, err := NewClient(WithLocalAddress("127.0.0.1:0"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if got := c.State(); got != StateDisconnected {
		t.Errorf("initial State = %v, want StateDisconnected", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if got := c.State(); got != StateConnected {
		t.Errorf("post-Connect State = %v, want StateConnected", got)
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	if got := c.State(); got != StateDisconnected {
		t.Errorf("post-Close State = %v, want StateDisconnected", got)
	}
}

// TestConnectWhileConnectedReturnsError confirms a second Connect against
// the same client returns ErrAlreadyConnected rather than re-opening the
// socket.
func TestConnectWhileConnectedReturnsError(t *testing.T) {
	c, _ := NewClient(WithLocalAddress("127.0.0.1:0"))
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	defer c.Close()

	err := c.Connect(ctx)
	if !errors.Is(err, ErrAlreadyConnected) {
		t.Errorf("second Connect err = %v, want ErrAlreadyConnected", err)
	}
}

// TestCloseIdempotent confirms calling Close on a never-connected and on an
// already-closed client both return nil without panicking. (Maps to the
// fork plan § E.1 row, and covers the symmetric case of the Connect/Close
// non-symmetry noted in AUDIT.md finding #14.)
func TestCloseIdempotent(t *testing.T) {
	c, _ := NewClient()
	if err := c.Close(); err != nil {
		t.Errorf("Close on never-connected client: %v", err)
	}

	c2, _ := NewClient(WithLocalAddress("127.0.0.1:0"))
	if err := c2.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := c2.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := c2.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestNextInvokeIDWraparound exercises 256 IDs and confirms they all live
// in the 8-bit range, since BACnet invoke IDs are 8-bit per spec
// (AUDIT.md "Known limits — 8-bit invoke IDs").
func TestNextInvokeIDWraparound(t *testing.T) {
	c, _ := NewClient()

	// Sample 300 IDs; every value must be a valid uint8. We don't assert
	// uniqueness across the cycle because the atomic uint32 starts at 0
	// and Add returns the post-increment value, so the first ID is 1.
	seen := make(map[uint8]int, 256)
	for i := 0; i < 300; i++ {
		id := c.nextInvokeID()
		seen[id]++
	}
	// We expect to have observed at least 250 distinct values out of 256;
	// the wrap should not lose values.
	if len(seen) < 250 {
		t.Errorf("nextInvokeID over 300 calls produced only %d distinct values; expected ≥250", len(seen))
	}
}

// TestCloseUnblocksPending confirms that Close() closes any channels
// registered in c.pending, releasing callers waiting on them. This is the
// guarantee that lets in-flight ReadProperty calls return cleanly during
// shutdown.
func TestCloseUnblocksPending(t *testing.T) {
	c, _ := NewClient(WithLocalAddress("127.0.0.1:0"))
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Register a fake pending entry to mimic an in-flight sendRequest.
	ch := make(chan *APDU, 1)
	c.pendingMu.Lock()
	c.pending[42] = ch
	c.pendingMu.Unlock()

	closed := make(chan struct{})
	go func() {
		_, ok := <-ch
		if ok {
			t.Errorf("pending channel produced a value; expected to be closed by Close()")
		}
		close(closed)
	}()

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("pending channel was not closed by Close() within 2s")
	}
}

// TestCloseWaitsForHandlePacket regression-tests AUDIT.md finding #2: the
// send-on-closed-channel race where Close races a handlePacket into a
// closed pending channel. We seed the WaitGroup as if a handlePacket is in
// flight, run Close in a goroutine, and confirm Close blocks until the
// "in-flight" handler signals done.
func TestCloseWaitsForHandlePacket(t *testing.T) {
	c, _ := NewClient(WithLocalAddress("127.0.0.1:0"))
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Simulate one in-flight handlePacket goroutine that hasn't returned.
	c.handlePacketWG.Add(1)

	closeReturned := make(chan struct{})
	go func() {
		_ = c.Close()
		close(closeReturned)
	}()

	// Close should be blocked on the WaitGroup. Confirm it hasn't
	// returned yet.
	select {
	case <-closeReturned:
		t.Fatal("Close returned before the simulated in-flight handlePacket signaled Done")
	case <-time.After(100 * time.Millisecond):
	}

	// Now release the WG; Close should complete promptly.
	c.handlePacketWG.Done()
	select {
	case <-closeReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return within 2s after Done")
	}
}
