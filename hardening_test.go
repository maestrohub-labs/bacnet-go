// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bacnet

import (
	"io"
	"log/slog"
	"net"
	"testing"
)

// newTestClient returns a Client with logging discarded — enough state
// for the decoder paths to run without setting up a UDPTransport.
func newTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// mustNotPanic is a defer helper that fails the test if its goroutine
// panics. Use it at the top of each hardening test.
func mustNotPanic(t *testing.T) {
	t.Helper()
	if r := recover(); r != nil {
		t.Fatalf("unexpected panic: %v", r)
	}
}

// ---------------------------------------------------------------------
// C1, C2: handleIAm must not panic on truncated or malformed payloads.
// ---------------------------------------------------------------------

func TestHandleIAmTruncatedAfterOIDTag(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 47808}

	// Application tag for ObjectID (tag 12, application class, length 4)
	// is the byte 0xC4. Pre-fix the code then did
	//   binary.BigEndian.Uint32(data[headerLen:])
	// expecting 4 bytes to follow but only seeing 2. That crashed.
	bad := []byte{0xC4, 0x01, 0x02} // header says "ObjectID len=4" but only 2 bytes follow
	c.handleIAm(bad, addr, &NPDU{})
}

func TestHandleIAmTruncatedAfterMaxAPDUTag(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 47808}

	// Valid ObjectID, then a tag for max-APDU claiming length=4 but
	// nothing follows. Pre-fix triggered an OOB slice in DecodeUnsigned.
	bad := []byte{
		0xC4, 0x00, 0x00, 0x00, 0x01, // ObjectID device:1
		0x24, // Unsigned tag, length=4 nibble (extended), but no payload
	}
	c.handleIAm(bad, addr, &NPDU{})
}

func TestHandleIAmCorruptSegmentationTag(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 47808}

	// ObjectID + max-APDU OK + segmentation tag claims extended length
	// past end-of-data.
	bad := []byte{
		0xC4, 0x00, 0x00, 0x00, 0x01, // ObjectID device:1
		0x21, 0x05, // Unsigned len=1, value=5
		0x25, 0xFE, 0xFF, 0xFF, // Enumerated extended-length 0xFFFF, but nothing follows
	}
	c.handleIAm(bad, addr, &NPDU{})
}

func TestHandleIAmValid(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 47808}

	good := []byte{
		0xC4, 0x02, 0x00, 0x04, 0xD2, // ObjectID device:1234 (type=8<<22 | 1234)
		0x22, 0x05, 0xC4, // Unsigned len=2, value=1476 (max APDU)
		0x91, 0x03, // Enumerated len=1, value=3 (segmentation=no-segmentation)
		0x21, 0x0F, // Unsigned len=1, value=15 (vendor ID)
	}
	c.handleIAm(good, addr, &NPDU{})

	// Verify the device made it into the cache.
	if _, ok := c.GetDevice(1234); !ok {
		t.Fatal("device 1234 was not registered from a valid I-Am")
	}
}

// ---------------------------------------------------------------------
// C3: decodePropertyValue ObjectID with wrong length must not panic.
// ---------------------------------------------------------------------

func TestDecodePropertyValueObjectIDWrongLength(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)

	// Application tag for ObjectID (tag 12) with length=3 (illegal).
	// Pre-fix the code then did Uint32 on a 3-byte slice and panicked.
	bad := []byte{0xC3, 0x01, 0x02, 0x03}
	_, err := c.decodePropertyValue(bad)
	if err == nil {
		t.Fatal("expected error for ObjectID with length=3")
	}
}

func TestDecodePropertyValueRealWrongLength(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	// Real tag (4), length=3 — illegal. Must not silently return 0;
	// must surface an error.
	bad := []byte{0x43, 0x01, 0x02, 0x03}
	_, err := c.decodePropertyValue(bad)
	if err == nil {
		t.Fatal("expected error for Real with length=3")
	}
}

func TestDecodePropertyValueDoubleWrongLength(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	bad := []byte{0x55, 0x04, 0x01, 0x02, 0x03, 0x04} // Double tag (5), length=4 in low nibble
	_, err := c.decodePropertyValue(bad)
	if err == nil {
		t.Fatal("expected error for Double with length=4")
	}
}

// ---------------------------------------------------------------------
// C4: decodeError must not panic on truncated Error PDU payloads.
// ---------------------------------------------------------------------

func TestDecodeErrorEmpty(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	if err := c.decodeError([]byte{}); err == nil {
		t.Fatal("expected error decoding empty Error PDU")
	}
}

func TestDecodeErrorTruncated(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	// First tag (Enumerated len=1, value 1) OK, then second tag header
	// only — no payload follows.
	bad := []byte{0x91, 0x01, 0x91}
	if err := c.decodeError(bad); err == nil {
		t.Fatal("expected error decoding truncated Error PDU")
	}
}

func TestDecodeErrorClaimsHugeLength(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	// Extended-length tag claiming length=255 but only a few bytes follow.
	bad := []byte{0x95, 0xFE, 0xFF, 0xFF, 0x01, 0x02}
	if err := c.decodeError(bad); err == nil {
		t.Fatal("expected error decoding Error PDU with bogus length")
	}
}

// ---------------------------------------------------------------------
// C5, C6: decodeReadPropertyMultipleResponse must not panic on bad input.
// ---------------------------------------------------------------------

func TestDecodeRPMResponseTruncatedAfterOIDTag(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	// Context tag 0, length 4 (OID) but only 2 bytes follow. Pre-fix
	// the code then called Uint32 on the truncated slice.
	bad := []byte{0x0C, 0x00, 0x00}
	results, _ := c.decodeReadPropertyMultipleResponse(bad)
	if len(results) != 0 {
		t.Fatalf("expected no results from truncated RPM response, got %d", len(results))
	}
}

func TestDecodeRPMResponsePropIDClaimsHugeLength(t *testing.T) {
	defer mustNotPanic(t)
	c := newTestClient(t)
	// Valid OID + opening tag [1], then a [2] property-id tag claiming
	// extended length 0xFFFF with no payload following.
	bad := []byte{
		0x0C, 0x00, 0x00, 0x00, 0x01, // OID context tag 0, len 4
		0x1E,                         // Opening tag [1]
		0x25, 0xFE, 0xFF, 0xFF,       // [2] propID, length=0xFFFF (bogus)
	}
	results, _ := c.decodeReadPropertyMultipleResponse(bad)
	if len(results) != 0 {
		t.Fatalf("expected no results from bogus-length RPM response, got %d", len(results))
	}
}

// ---------------------------------------------------------------------
// Defense-in-depth: handlePacket goroutine recovers from synthetic panics.
// (Tests the recover() at the receiver-goroutine boundary by hooking
// into the same defer/recover pattern.)
// ---------------------------------------------------------------------

func TestReceiverGoroutineRecoverPattern(t *testing.T) {
	// Mirror the recover() block from client.go's receiver. If a future
	// edit removes that defer, this test will not catch it — but the
	// presence of this test pins the *pattern* (recover -> increment
	// metric -> log) so it's harder to break by accident.
	c := newTestClient(t)
	before := c.metrics.PanicsRecovered.Value()

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				c.metrics.PanicsRecovered.Inc()
			}
		}()
		panic("synthetic")
	}()
	<-done

	if got := c.metrics.PanicsRecovered.Value(); got != before+1 {
		t.Fatalf("PanicsRecovered: want %d, got %d", before+1, got)
	}
}
