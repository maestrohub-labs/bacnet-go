// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package bacnet

import (
	"bytes"
	"errors"
	"testing"
)

// TestBVLCEncodeDecodeRoundTrip confirms that EncodeBVLC + DecodeBVLC
// produce wire-format BVLC headers per BACnet/IP spec § 20.1.1.
func TestBVLCEncodeDecodeRoundTrip(t *testing.T) {
	header := EncodeBVLC(BVLCOriginalUnicastNPDU, 10)
	if len(header) != 4 {
		t.Fatalf("BVLC header length = %d, want 4", len(header))
	}
	if header[0] != 0x81 {
		t.Errorf("BVLC type = 0x%02x, want 0x81 (BACnet/IP)", header[0])
	}
	if header[1] != byte(BVLCOriginalUnicastNPDU) {
		t.Errorf("BVLC function = 0x%02x, want 0x%02x", header[1], byte(BVLCOriginalUnicastNPDU))
	}
	// Total length = 4 (header) + 10 (NPDU)
	totalLen := (uint16(header[2]) << 8) | uint16(header[3])
	if totalLen != 14 {
		t.Errorf("BVLC total length = %d, want 14", totalLen)
	}

	decoded, err := DecodeBVLC(header)
	if err != nil {
		t.Fatalf("DecodeBVLC: %v", err)
	}
	if decoded.Type != BVLCTypeBACnetIP || decoded.Function != BVLCOriginalUnicastNPDU || decoded.Length != 14 {
		t.Errorf("decoded BVLC = %+v, want type=0x81 function=0x%02x length=14",
			decoded, byte(BVLCOriginalUnicastNPDU))
	}
}

// TestDecodeBVLCTooShort confirms a BVLC header < 4 bytes returns the
// sentinel ErrInvalidBVLC without panicking.
func TestDecodeBVLCTooShort(t *testing.T) {
	for _, in := range [][]byte{nil, {}, {0x81}, {0x81, 0x0a}, {0x81, 0x0a, 0x00}} {
		_, err := DecodeBVLC(in)
		if !errors.Is(err, ErrInvalidBVLC) {
			t.Errorf("DecodeBVLC(%v) err = %v, want ErrInvalidBVLC", in, err)
		}
	}
}

// TestDecodeNPDURejectsUnsupportedVersion confirms only NPDU version 0x01
// is accepted; any other version errors out.
func TestDecodeNPDURejectsUnsupportedVersion(t *testing.T) {
	// Version byte = 0x02, then a control byte.
	_, _, err := DecodeNPDU([]byte{0x02, 0x00})
	if !errors.Is(err, ErrInvalidNPDU) {
		t.Errorf("DecodeNPDU with version 0x02 err = %v, want ErrInvalidNPDU", err)
	}
}

// TestDecodeAPDUMalformed feeds the APDU decoder a variety of malformed
// inputs (truncated, unknown PDU type, random bytes) and verifies it
// returns an error rather than panicking or returning a partial result.
func TestDecodeAPDUMalformed(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
	}{
		{"empty", []byte{}},
		{"nil", nil},
		{"truncated confirmed request", []byte{0x00, 0x05, 0x01}}, // need >= 4 bytes
		{"truncated complex ack", []byte{0x30, 0x01}},             // need >= 3 bytes
		{"truncated simple ack", []byte{0x20, 0x01}},
		{"truncated unconfirmed", []byte{0x10}},
		{"unknown pdu type 0x90", []byte{0x90, 0x01, 0x02, 0x03}},
		{"random bytes", []byte{0xde, 0xad, 0xbe, 0xef}}, // 0xd0 = unknown PDU type
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("DecodeAPDU(%x) panicked: %v", tc.in, r)
				}
			}()
			_, err := DecodeAPDU(tc.in)
			if err == nil {
				t.Errorf("DecodeAPDU(%x) returned nil error", tc.in)
			}
		})
	}
}

// TestReadPropertyEncoding hand-computes the expected wire bytes for the
// ReadProperty service-payload portion of a confirmed request and asserts
// the encoder emits them. Covers the BACnet § 15.5.1.1 frame for the
// common (object-id, property-id) case with no ArrayIndex.
//
// For Device:1234 / Object_Name (PropertyIdentifier 77):
//   ObjectIdentifier = (8 << 22) | 1234 = 0x020004D2
//   [0] context tag for OID:           0x0C 0x02 0x00 0x04 0xD2
//   [1] context tag for property (77): 0x19 0x4D
func TestReadPropertyEncoding(t *testing.T) {
	oid := NewObjectIdentifier(ObjectTypeDevice, 1234)

	got := append([]byte{}, EncodeContextObjectIdentifier(0, oid)...)
	got = append(got, EncodeContextEnumerated(1, uint32(PropertyObjectName))...)

	want := []byte{
		0x0C, 0x02, 0x00, 0x04, 0xD2, // [0] OID = device 1234
		0x19, 0x4D, //                    [1] property = 77 (object-name)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("ReadProperty payload mismatch\n got: %x\nwant: %x", got, want)
	}
}

// TestArrayAllEncoding confirms that wrapping ArrayAll (0xFFFFFFFF) in a
// context-tagged unsigned integer produces the spec-defined 5-byte
// sequence for "[2] context tag, length 4, value 0xFFFFFFFF".
func TestArrayAllEncoding(t *testing.T) {
	got := EncodeContextUnsigned(2, ArrayAll)
	want := []byte{0x2C, 0xFF, 0xFF, 0xFF, 0xFF}
	if !bytes.Equal(got, want) {
		t.Errorf("ArrayAll-as-context-[2] payload mismatch\n got: %x\nwant: %x", got, want)
	}
}

// TestEncodeUnsignedWidthSelection confirms the variable-length encoding of
// unsigned integers per BACnet spec — 1 byte for values < 256, 2 bytes up
// to 65535, etc.
func TestEncodeUnsignedWidthSelection(t *testing.T) {
	cases := []struct {
		v    uint32
		want []byte
	}{
		{0, []byte{0x00}},
		{255, []byte{0xFF}},
		{256, []byte{0x01, 0x00}},
		{65535, []byte{0xFF, 0xFF}},
		{65536, []byte{0x01, 0x00, 0x00}},
		{0xFFFFFF, []byte{0xFF, 0xFF, 0xFF}},
		{0x01000000, []byte{0x01, 0x00, 0x00, 0x00}},
		{0xFFFFFFFF, []byte{0xFF, 0xFF, 0xFF, 0xFF}},
	}
	for _, tc := range cases {
		got := EncodeUnsigned(tc.v)
		if !bytes.Equal(got, tc.want) {
			t.Errorf("EncodeUnsigned(0x%x) = %x, want %x", tc.v, got, tc.want)
		}
	}
}

// TestDecodeUnsignedRoundTrip confirms decode is the inverse of encode for
// the four supported widths.
func TestDecodeUnsignedRoundTrip(t *testing.T) {
	for _, v := range []uint32{0, 1, 255, 256, 65535, 65536, 0xFFFFFF, 0xFFFFFFFF} {
		encoded := EncodeUnsigned(v)
		decoded := DecodeUnsigned(encoded)
		if decoded != v {
			t.Errorf("DecodeUnsigned(EncodeUnsigned(0x%x)) = 0x%x", v, decoded)
		}
	}
}

// TestEncodeRealRoundTrip confirms float32 encoding via IEEE 754 big-endian.
func TestEncodeRealRoundTrip(t *testing.T) {
	for _, v := range []float32{0, 1, -1, 75.5, 1e-9, 1e9} {
		got := DecodeReal(EncodeReal(v))
		if got != v {
			t.Errorf("Real round-trip lost data: %v -> %v", v, got)
		}
	}
}

// TestTagShortFormSelection verifies the encoder picks the short tag form
// for length < 5 and tag-number < 15 (per BACnet spec § 20.2.1.3.1).
func TestTagShortFormSelection(t *testing.T) {
	got := EncodeTag(2, TagClassContext, 1)
	if len(got) != 1 {
		t.Errorf("expected short-form tag (1 byte), got %d bytes: %x", len(got), got)
	}
	// (2 << 4) | (1 << 3) | 1 = 0x29
	if got[0] != 0x29 {
		t.Errorf("short-form tag byte = 0x%02x, want 0x29", got[0])
	}
}

// TestTagExtendedNumber confirms the extended tag-number encoding kicks in
// when tag-number >= 15.
func TestTagExtendedNumber(t *testing.T) {
	got := EncodeTag(20, TagClassApplication, 3)
	// First byte = 0xF0 (extended tag indicator), then 20 as the tag-number byte.
	if got[0] != 0xF0 || got[1] != 20 {
		t.Errorf("extended-tag-number prefix = %02x %02x, want F0 14", got[0], got[1])
	}
}

// TestTagExtendedLengthOneByte covers lengths 5..253 → 5 in the length
// nibble + one length byte.
func TestTagExtendedLengthOneByte(t *testing.T) {
	got := EncodeTag(3, TagClassApplication, 100)
	// First byte = (3<<4) | (0<<3) | 5 = 0x35
	if got[0] != 0x35 {
		t.Errorf("tag header byte = 0x%02x, want 0x35", got[0])
	}
	if got[1] != 100 {
		t.Errorf("extended length byte = %d, want 100", got[1])
	}
}

// TestTagExtendedLengthTwoByte covers lengths 254..65535 → 5 in the length
// nibble + marker 254 + 2 length bytes.
func TestTagExtendedLengthTwoByte(t *testing.T) {
	got := EncodeTag(3, TagClassApplication, 1000)
	if got[1] != 254 {
		t.Errorf("expected 254 marker for 2-byte length, got %d", got[1])
	}
	encoded := uint16(got[2])<<8 | uint16(got[3])
	if encoded != 1000 {
		t.Errorf("encoded length = %d, want 1000", encoded)
	}
}

// TestEncodeBooleanTagFlavors confirms the BACnet quirk: application-tagged
// booleans encode their value in the length nibble (no payload byte), but
// context-tagged booleans get a 1-byte payload.
func TestEncodeBooleanTagFlavors(t *testing.T) {
	if !bytes.Equal(EncodeBooleanTag(true), []byte{0x11}) {
		t.Errorf("EncodeBooleanTag(true) = %x, want 11", EncodeBooleanTag(true))
	}
	if !bytes.Equal(EncodeBooleanTag(false), []byte{0x10}) {
		t.Errorf("EncodeBooleanTag(false) = %x, want 10", EncodeBooleanTag(false))
	}
	// Context-tagged: tag header + 1-byte payload (0/1).
	ctxTrue := EncodeContextBoolean(5, true)
	if len(ctxTrue) != 2 || ctxTrue[1] != 1 {
		t.Errorf("EncodeContextBoolean(5, true) = %x", ctxTrue)
	}
}

// TestEncodeRealAndRealTag confirms application-tagged real is 5 bytes:
// 1 tag header + 4 IEEE-754 payload.
func TestEncodeRealAndRealTag(t *testing.T) {
	tagged := EncodeRealTag(1.0)
	if len(tagged) != 5 {
		t.Fatalf("EncodeRealTag length = %d, want 5", len(tagged))
	}
	// Tag byte: (TagReal=4 << 4) | 0 | 4 = 0x44
	if tagged[0] != 0x44 {
		t.Errorf("real tag header = 0x%02x, want 0x44", tagged[0])
	}
}

// TestEncodeDoubleRoundTrip verifies IEEE-754 big-endian round-trip for
// float64. Confirms 8-byte payload width.
func TestEncodeDoubleRoundTrip(t *testing.T) {
	for _, v := range []float64{0, 1, -1, 3.14159265358979, 1e-100, 1e100} {
		raw := EncodeDouble(v)
		if len(raw) != 8 {
			t.Errorf("EncodeDouble(%v) length = %d, want 8", v, len(raw))
		}
		if got := DecodeDouble(raw); got != v {
			t.Errorf("Double round-trip lost data: %v -> %v", v, got)
		}
	}
}

// TestEncodeSignedRoundTrip exercises all four width branches of the
// variable-length signed encoder.
func TestEncodeSignedRoundTrip(t *testing.T) {
	cases := []int32{0, 1, -1, 127, -128, 128, -129, 32767, -32768, 32768, -32769,
		8388607, -8388608, 8388608, -8388609, 0x7FFFFFFF, -0x80000000}
	for _, v := range cases {
		encoded := EncodeSigned(v)
		decoded := DecodeSigned(encoded)
		if decoded != v {
			t.Errorf("DecodeSigned(EncodeSigned(%d)) = %d", v, decoded)
		}
	}
}

// TestEncodeCharacterStringPrefixesCharset confirms the encoder prepends
// the UTF-8 character-set byte (0) as required by BACnet spec § 20.2.9.
func TestEncodeCharacterStringPrefixesCharset(t *testing.T) {
	got := EncodeCharacterString("hi")
	want := []byte{0x00, 'h', 'i'}
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeCharacterString(hi) = %x, want %x", got, want)
	}

	// Decode discards the charset byte.
	if DecodeCharacterString(got) != "hi" {
		t.Errorf("DecodeCharacterString lost data")
	}
}

// TestEncodeObjectIdentifierTag confirms the application-tagged OID is the
// expected 5 bytes: tag header (0xC4: TagObjectID=12, app, length 4) + 4
// bytes of encoded OID.
func TestEncodeObjectIdentifierTag(t *testing.T) {
	oid := NewObjectIdentifier(ObjectTypeAnalogInput, 1)
	got := EncodeObjectIdentifierTag(oid)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0] != 0xC4 {
		t.Errorf("OID tag header = 0x%02x, want 0xC4", got[0])
	}
}

// TestEncodeEnumeratedTagWidth confirms the enumerated tag uses the
// variable-width unsigned encoding.
func TestEncodeEnumeratedTagWidth(t *testing.T) {
	got := EncodeEnumeratedTag(85)
	// (TagEnumerated=9 << 4) | (0 << 3) | 1 = 0x91, then 1 byte = 85.
	if !bytes.Equal(got, []byte{0x91, 0x55}) {
		t.Errorf("EncodeEnumeratedTag(85) = %x, want 91 55", got)
	}
}

// TestDecodeTagNumberShortForm checks the simplest path of the tag decoder.
func TestDecodeTagNumberShortForm(t *testing.T) {
	// Tag header (5 << 4) | (1 << 3) | 2 = 0x5A — context tag 5, length 2.
	tagNum, class, length, headerLen, err := DecodeTagNumber([]byte{0x5A})
	if err != nil {
		t.Fatalf("DecodeTagNumber: %v", err)
	}
	if tagNum != 5 || class != TagClassContext || length != 2 || headerLen != 1 {
		t.Errorf("got (tagNum=%d, class=%d, length=%d, headerLen=%d), want (5, Context, 2, 1)",
			tagNum, class, length, headerLen)
	}
}

// TestDecodeTagNumberOpeningClosing confirms the special length encodings
// for constructed-data opening (-1) and closing (-2) markers.
func TestDecodeTagNumberOpeningClosing(t *testing.T) {
	// Opening tag for context tag 3: (3 << 4) | (1 << 3) | 0x06 = 0x3E
	_, _, length, _, err := DecodeTagNumber([]byte{0x3E})
	if err != nil || length != -1 {
		t.Errorf("opening tag length = %d, want -1 (err=%v)", length, err)
	}

	// Closing tag for context tag 3: 0x3F
	_, _, length, _, err = DecodeTagNumber([]byte{0x3F})
	if err != nil || length != -2 {
		t.Errorf("closing tag length = %d, want -2 (err=%v)", length, err)
	}
}

// TestDecodeTagNumberExtendedNumber confirms the extended tag-number form
// (0xF0 prefix) decodes correctly.
func TestDecodeTagNumberExtendedNumber(t *testing.T) {
	// 0xF8 = extended tag, context class, length 0. Tag number = 25 (0x19).
	tagNum, class, _, headerLen, err := DecodeTagNumber([]byte{0xF8, 0x19})
	if err != nil {
		t.Fatalf("DecodeTagNumber extended: %v", err)
	}
	if tagNum != 25 || class != TagClassContext || headerLen != 2 {
		t.Errorf("got (tagNum=%d, class=%d, headerLen=%d), want (25, Context, 2)",
			tagNum, class, headerLen)
	}
}

// TestDecodeTagNumberTooShort confirms the decoder errors cleanly on
// truncated tag input rather than panicking.
func TestDecodeTagNumberTooShort(t *testing.T) {
	_, _, _, _, err := DecodeTagNumber([]byte{})
	if !errors.Is(err, ErrInvalidAPDU) {
		t.Errorf("empty input err = %v, want ErrInvalidAPDU", err)
	}
}

// TestDecodeObjectIdentifierFromBytes confirms the bytes→OID helper inverts
// EncodeObjectIdentifier.
func TestDecodeObjectIdentifierFromBytes(t *testing.T) {
	want := NewObjectIdentifier(ObjectTypeBinaryOutput, 99)
	got := DecodeObjectIdentifierFromBytes(EncodeObjectIdentifier(want))
	if got != want {
		t.Errorf("round-trip OID via bytes = %+v, want %+v", got, want)
	}

	// Wrong length returns zero value.
	if got := DecodeObjectIdentifierFromBytes([]byte{1, 2, 3}); got != (ObjectIdentifier{}) {
		t.Errorf("3-byte input should yield zero OID, got %+v", got)
	}
}

// TestDecodeStatusFlags exercises the 4-bit flag mapping.
func TestDecodeStatusFlags(t *testing.T) {
	// 0x0F = all four flags set
	got := DecodeStatusFlags(0x0F)
	if !got.InAlarm || !got.Fault || !got.Overridden || !got.OutOfService {
		t.Errorf("0x0F decoded to %+v; expected all flags set", got)
	}
	// 0x00 = nothing set
	got = DecodeStatusFlags(0x00)
	if got.InAlarm || got.Fault || got.Overridden || got.OutOfService {
		t.Errorf("0x00 decoded to %+v; expected all flags clear", got)
	}
}

// TestEncodeNPDUUnicast confirms the 2-byte NPDU header for the common
// "unicast, no routing" case.
func TestEncodeNPDUUnicast(t *testing.T) {
	got := EncodeNPDU(true, NPDUControlPriorityNormal)
	want := []byte{0x01, byte(NPDUControlExpectingReply)}
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeNPDU(expectReply=true, normal) = %x, want %x", got, want)
	}
}

// TestEncodeNPDUWithDest confirms the routed NPDU header includes the
// destination network, address length, address bytes, and hop count.
func TestEncodeNPDUWithDest(t *testing.T) {
	got := EncodeNPDUWithDest(42, []byte{0xAA, 0xBB}, 255, false, NPDUControlPriorityNormal)
	want := []byte{
		0x01,                                 // version
		byte(NPDUControlDestSpecifier),       // control, dest only
		0x00, 0x2A,                           // dest net = 42
		0x02,                                 // address length = 2
		0xAA, 0xBB,                           // address
		0xFF,                                 // hop count
	}
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeNPDUWithDest = %x, want %x", got, want)
	}
}

// TestEncodeUnconfirmedRequest confirms the unconfirmed PDU header is a
// single PDU-type byte followed by the service-choice byte and payload.
func TestEncodeUnconfirmedRequest(t *testing.T) {
	got := EncodeUnconfirmedRequest(ServiceWhoIs, []byte{0xde, 0xad})
	want := []byte{byte(PDUTypeUnconfirmedRequest), byte(ServiceWhoIs), 0xde, 0xad}
	if !bytes.Equal(got, want) {
		t.Errorf("EncodeUnconfirmedRequest = %x, want %x", got, want)
	}
}

// TestEncodeConfirmedRequestHeader confirms the 4-byte confirmed-request
// header layout and that the service-choice byte lands in the right slot.
func TestEncodeConfirmedRequestHeader(t *testing.T) {
	got := EncodeConfirmedRequest(7, ServiceReadProperty, []byte{0xaa, 0xbb}, 0, 5)
	if got[0] != byte(PDUTypeConfirmedRequest) {
		t.Errorf("PDU type byte = 0x%02x, want 0x%02x", got[0], byte(PDUTypeConfirmedRequest))
	}
	// max-segments<<4 | max-APDU = (0<<4)|5 = 0x05
	if got[1] != 0x05 {
		t.Errorf("max-seg/APDU byte = 0x%02x, want 0x05", got[1])
	}
	if got[2] != 7 {
		t.Errorf("invokeID = %d, want 7", got[2])
	}
	if got[3] != byte(ServiceReadProperty) {
		t.Errorf("service choice = 0x%02x, want 0x%02x", got[3], byte(ServiceReadProperty))
	}
}
