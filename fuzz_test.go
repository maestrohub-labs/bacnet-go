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

// All fuzz targets below assert ONE invariant: feeding arbitrary bytes
// to the decoder must not panic. The test harness implicitly fails on
// any panic in the fuzz body, so we don't need an explicit check.
//
// To actively fuzz (instead of just replaying the corpus during a
// normal `go test`), use:
//
//   go test -run='^$' -fuzz=FuzzHandlePacket -fuzztime=30s ./...

// FuzzHandlePacket throws arbitrary bytes at the full receive path —
// BVLC, NPDU, APDU, and any service-level handler the type-byte selects.
// This is the most important target because handlePacket is the entry
// point for every wire-supplied packet at the receiver-goroutine boundary.
func FuzzHandlePacket(f *testing.F) {
	// Seed with a few packets that hit interesting branches.
	f.Add([]byte{}) // empty
	f.Add([]byte{0x81, 0x0A, 0x00, 0x0C, 0x01, 0x00, 0x10, 0x08}) // BVLC unicast + minimal Who-Is
	f.Add([]byte{0x81, 0x0A, 0x00, 0x14, 0x01, 0x00, 0x30, 0x01, 0x0C,
		0xC4, 0x02, 0x00, 0x00, 0x01, 0x29, 0x4D, 0x3E, 0x44, 0x42, 0x91, 0x00, 0x3F})

	c := newTestFuzzClient(f)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 47808}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Wrap in the same recover as the production receiver so
		// fuzzing tests the defense-in-depth layer too. If the recover
		// catches anything, the test fails — fuzzing should drive the
		// defenders, not the safety net.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic in handlePacket: %v\ninput: %x", r, data)
			}
		}()
		c.handlePacket(data, addr)
	})
}

// FuzzDecodePropertyValue is the second-most-important target — this
// is what gets called for every property value returned by a server,
// for every application tag we support. It also covers the BitString /
// Date / Time decoders added in this release.
func FuzzDecodePropertyValue(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})       // null
	f.Add([]byte{0x10})       // boolean false
	f.Add([]byte{0x11})       // boolean true
	f.Add([]byte{0x21, 0x05}) // unsigned 5
	f.Add([]byte{0x44, 0x42, 0x91, 0x00, 0x00}) // real
	f.Add([]byte{0x82, 0x04, 0xC0})             // bit-string
	f.Add([]byte{0xA4, 126, 5, 15, 5})          // date
	f.Add([]byte{0xB4, 14, 30, 45, 50})         // time
	f.Add([]byte{0xC4, 0x02, 0x00, 0x00, 0x01}) // object-id

	c := newTestFuzzClient(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic in decodePropertyValue: %v\ninput: %x", r, data)
			}
		}()
		_, _ = c.decodePropertyValue(data)
	})
}

// FuzzDecodeError targets the Error-PDU decode path that runs synchronously
// inside sendRequest for every error reply. A panic here would propagate
// up to the caller's goroutine rather than the recover()-guarded receiver,
// so the bound-check correctness matters even more here.
func FuzzDecodeError(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x91, 0x01, 0x91, 0x02}) // class=1, code=2
	f.Add([]byte{0x95, 0xFE, 0xFF, 0xFF, 0x00})

	c := newTestFuzzClient(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic in decodeError: %v\ninput: %x", r, data)
			}
		}()
		_ = c.decodeError(data)
	})
}

// FuzzDecodeReadPropertyResponse targets the full RP-Ack decode path,
// including the call-through to decodePropertyValue.
func FuzzDecodeReadPropertyResponse(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{
		0x0C, 0x02, 0x00, 0x00, 0x01, // OID context tag 0, len 4
		0x19, 0x4D,                   // PropID context tag 1, len 1 = ObjectName
		0x3E,                         // Opening tag [3]
		0x75, 0x06, 0x00, 'H', 'e', 'l', 'l', 'o', // CharString
		0x3F, // Closing tag [3]
	})

	c := newTestFuzzClient(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic in decodeReadPropertyResponse: %v\ninput: %x", r, data)
			}
		}()
		_, _ = c.decodeReadPropertyResponse(data)
	})
}

// FuzzDecodeReadPropertyMultipleResponse targets the RPM-Ack decode
// path; this is the most complex decoder in the library and the most
// likely landing zone for a subtle bounds bug.
func FuzzDecodeReadPropertyMultipleResponse(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{
		0x0C, 0x02, 0x00, 0x00, 0x01, // OID
		0x1E,                         // [1] opening
		0x29, 0x4D,                   // [2] propID = ObjectName
		0x4E,                         // [4] opening
		0x75, 0x06, 0x00, 'H', 'e', 'l', 'l', 'o',
		0x4F, // [4] closing
		0x1F, // [1] closing
	})

	c := newTestFuzzClient(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic in decodeReadPropertyMultipleResponse: %v\ninput: %x", r, data)
			}
		}()
		_, _ = c.decodeReadPropertyMultipleResponse(data)
	})
}

// newTestFuzzClient mirrors newTestClient but takes a *testing.F instead
// of *testing.T. Logging is discarded so fuzzing isn't slowed by I/O.
func newTestFuzzClient(f *testing.F) *Client {
	f.Helper()
	c, err := NewClient(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err != nil {
		f.Fatalf("NewClient: %v", err)
	}
	return c
}
