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

import "strings"

// BitString represents a BACnet BitString (application tag 8). Per
// ASHRAE 135 § 20.2.10, a BitString is encoded as one prefix byte
// holding the count of unused bits in the last data byte (0-7),
// followed by zero or more data bytes carrying the bits MSB-first
// (bit position 0 is the high bit of the first data byte).
//
// The struct keeps the unused-bits count separately so a BitString of
// any size (Status_Flags is 4 bits, Protocol_Object_Types_Supported is
// 64+ bits) can round-trip without loss.
type BitString struct {
	UnusedBits uint8
	Bytes      []byte
}

// Len returns the meaningful bit count.
func (b BitString) Len() int {
	if len(b.Bytes) == 0 {
		return 0
	}
	return len(b.Bytes)*8 - int(b.UnusedBits)
}

// Bit returns the value of bit position i, where bit 0 is the MSB of
// the first byte. Out-of-range indices return false.
func (b BitString) Bit(i int) bool {
	if i < 0 || i >= b.Len() {
		return false
	}
	return b.Bytes[i/8]&(0x80>>uint(i%8)) != 0
}

// String renders the bits in `b0 b1 b2 ...` order for debugging.
func (b BitString) String() string {
	if b.Len() == 0 {
		return "{}"
	}
	var sb strings.Builder
	sb.WriteByte('{')
	for i := 0; i < b.Len(); i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		if b.Bit(i) {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	sb.WriteByte('}')
	return sb.String()
}

// StatusFlags interprets the first 4 bits of the BitString as the
// BACnet Status_Flags property (in-alarm, fault, overridden,
// out-of-service). Bit positions follow the spec: bit 0 = in-alarm at
// the MSB of byte 0.
//
// This is preferable to the package-level DecodeStatusFlags(byte) helper
// when working with a value read off the wire as a BitString, because
// it uses the spec's bit ordering (0x80, 0x40, 0x20, 0x10) rather than
// the lower-nibble masks used by DecodeStatusFlags.
func (b BitString) StatusFlags() StatusFlags {
	return StatusFlags{
		InAlarm:      b.Bit(0),
		Fault:        b.Bit(1),
		Overridden:   b.Bit(2),
		OutOfService: b.Bit(3),
	}
}

// DecodeBitString parses the BitString payload that follows an application
// tag header. Returns ErrInvalidResponse if the unused-bits prefix is
// missing or out of range (>7), per the spec.
func DecodeBitString(payload []byte) (BitString, error) {
	if len(payload) == 0 {
		return BitString{}, ErrInvalidResponse
	}
	unused := payload[0]
	if unused > 7 {
		return BitString{}, ErrInvalidResponse
	}
	// If there are no data bytes the unused-bits count must be zero.
	if len(payload) == 1 && unused != 0 {
		return BitString{}, ErrInvalidResponse
	}
	bs := BitString{
		UnusedBits: unused,
		Bytes:      make([]byte, len(payload)-1),
	}
	copy(bs.Bytes, payload[1:])
	return bs, nil
}

// EncodeBitString encodes the payload bytes (without the application
// tag header). The first byte is the unused-bits count; remaining
// bytes are the bits packed MSB-first.
func EncodeBitString(b BitString) []byte {
	out := make([]byte, 1+len(b.Bytes))
	out[0] = b.UnusedBits
	copy(out[1:], b.Bytes)
	return out
}

// EncodeBitStringTag encodes a BitString with an application tag (tag 8).
func EncodeBitStringTag(b BitString) []byte {
	data := EncodeBitString(b)
	tag := EncodeTag(uint8(TagBitString), TagClassApplication, len(data))
	return append(tag, data...)
}

// NewBitStringFromBits builds a BitString from a slice of bool bit
// values, ordered with bits[0] at the MSB of the first byte. Useful for
// constructing inputs to EncodeBitStringTag in tests and WriteProperty
// calls.
func NewBitStringFromBits(bits []bool) BitString {
	if len(bits) == 0 {
		return BitString{}
	}
	nBytes := (len(bits) + 7) / 8
	out := BitString{
		UnusedBits: uint8(nBytes*8 - len(bits)),
		Bytes:      make([]byte, nBytes),
	}
	for i, v := range bits {
		if v {
			out.Bytes[i/8] |= 0x80 >> uint(i%8)
		}
	}
	return out
}

