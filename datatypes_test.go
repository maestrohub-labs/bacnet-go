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
	"bytes"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------
// BitString
// ---------------------------------------------------------------------

func TestBitStringStatusFlagsAllSet(t *testing.T) {
	// In-alarm, fault, overridden, out-of-service all true => high nibble
	// 0xF0, with 4 unused bits.
	bs, err := DecodeBitString([]byte{0x04, 0xF0})
	if err != nil {
		t.Fatalf("DecodeBitString: %v", err)
	}
	if bs.Len() != 4 {
		t.Fatalf("Len: want 4, got %d", bs.Len())
	}
	sf := bs.StatusFlags()
	if !(sf.InAlarm && sf.Fault && sf.Overridden && sf.OutOfService) {
		t.Fatalf("StatusFlags: want all true, got %+v", sf)
	}
}

func TestBitStringStatusFlagsInAlarmOnly(t *testing.T) {
	// Bit 0 set only => 0x80 with 4 unused bits.
	bs, _ := DecodeBitString([]byte{0x04, 0x80})
	sf := bs.StatusFlags()
	if !(sf.InAlarm && !sf.Fault && !sf.Overridden && !sf.OutOfService) {
		t.Fatalf("StatusFlags: want in-alarm only, got %+v", sf)
	}
}

func TestBitStringRoundTrip(t *testing.T) {
	// 13 bits: 1010101010101 => bytes [0xAA, 0xA8] with 3 unused bits.
	bits := []bool{true, false, true, false, true, false, true, false, true, false, true, false, true}
	bs := NewBitStringFromBits(bits)

	if bs.Len() != len(bits) {
		t.Fatalf("Len: want %d, got %d", len(bits), bs.Len())
	}
	for i, want := range bits {
		if got := bs.Bit(i); got != want {
			t.Errorf("Bit(%d): want %v, got %v", i, want, got)
		}
	}

	// Round-trip through encode + decode.
	encoded := EncodeBitString(bs)
	decoded, err := DecodeBitString(encoded)
	if err != nil {
		t.Fatalf("DecodeBitString round-trip: %v", err)
	}
	if !reflect.DeepEqual(bs, decoded) {
		t.Fatalf("round-trip mismatch:\n  before: %+v\n  after:  %+v", bs, decoded)
	}
}

func TestBitStringRejectsBadUnusedBits(t *testing.T) {
	if _, err := DecodeBitString([]byte{0x08, 0x00}); err == nil {
		t.Fatal("expected error for unused-bits > 7")
	}
	if _, err := DecodeBitString([]byte{0xFF}); err == nil {
		t.Fatal("expected error for unused-bits > 7 (single-byte form)")
	}
}

func TestBitStringRejectsEmpty(t *testing.T) {
	if _, err := DecodeBitString(nil); err == nil {
		t.Fatal("expected error decoding empty BitString payload")
	}
}

// ---------------------------------------------------------------------
// Date
// ---------------------------------------------------------------------

func TestDateRoundTrip(t *testing.T) {
	d := Date{Year: 2026, Month: 5, Day: 15, DayOfWeek: 5}
	encoded := EncodeDate(d)
	if !bytes.Equal(encoded, []byte{126, 5, 15, 5}) {
		t.Fatalf("encoded: want [126 5 15 5], got %v", encoded)
	}
	decoded := DecodeDate(encoded)
	if decoded != d {
		t.Fatalf("round-trip: want %+v, got %+v", d, decoded)
	}
}

func TestDateWildcards(t *testing.T) {
	d := Date{Year: DateWildcardYear, Month: DateWildcardField, Day: DateWildcardField, DayOfWeek: DateWildcardField}
	encoded := EncodeDate(d)
	if !bytes.Equal(encoded, []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
		t.Fatalf("wildcard encoding: %v", encoded)
	}
	decoded := DecodeDate(encoded)
	if decoded != d {
		t.Fatalf("wildcard round-trip: want %+v, got %+v", d, decoded)
	}
}

func TestDateString(t *testing.T) {
	if s := (Date{Year: 2026, Month: 5, Day: 15}).String(); s != "2026-05-15" {
		t.Errorf("Date.String: want 2026-05-15, got %q", s)
	}
	if s := (Date{Year: DateWildcardYear, Month: DateWildcardField, Day: DateWildcardField}).String(); s != "?-?-?" {
		t.Errorf("wildcard Date.String: want ?-?-?, got %q", s)
	}
}

// ---------------------------------------------------------------------
// Time
// ---------------------------------------------------------------------

func TestTimeRoundTrip(t *testing.T) {
	tm := Time{Hour: 14, Minute: 30, Second: 45, Hundredths: 50}
	encoded := EncodeTime(tm)
	if !bytes.Equal(encoded, []byte{14, 30, 45, 50}) {
		t.Fatalf("encoded: %v", encoded)
	}
	if decoded := DecodeTime(encoded); decoded != tm {
		t.Fatalf("round-trip: %+v", decoded)
	}
}

func TestTimeWildcardString(t *testing.T) {
	tm := Time{Hour: 14, Minute: TimeWildcardField, Second: 30, Hundredths: TimeWildcardField}
	if s := tm.String(); s != "14:?:30.?" {
		t.Errorf("Time.String: want 14:?:30.?, got %q", s)
	}
}

// ---------------------------------------------------------------------
// CharacterString character set handling.
// ---------------------------------------------------------------------

func TestDecodeCharacterStringUTF8(t *testing.T) {
	data := append([]byte{CharSetUTF8}, []byte("café 雷")...)
	if got := DecodeCharacterString(data); got != "café 雷" {
		t.Errorf("UTF-8: %q", got)
	}
}

func TestDecodeCharacterStringUCS2(t *testing.T) {
	// "Hi" in UCS-2 BE: 0x0048 0x0069
	data := []byte{CharSetUCS2, 0x00, 0x48, 0x00, 0x69}
	if got := DecodeCharacterString(data); got != "Hi" {
		t.Errorf("UCS-2: %q", got)
	}
}

func TestDecodeCharacterStringUCS2Truncated(t *testing.T) {
	// One full code point then a trailing odd byte. Trim and continue.
	data := []byte{CharSetUCS2, 0x00, 0x48, 0x69}
	if got := DecodeCharacterString(data); got != "H" {
		t.Errorf("UCS-2 truncated: %q", got)
	}
}

func TestDecodeCharacterStringISO88591(t *testing.T) {
	// "café" in Latin-1: 'c', 'a', 'f', é=0xE9
	data := []byte{CharSetISO88591, 'c', 'a', 'f', 0xE9}
	if got := DecodeCharacterString(data); got != "café" {
		t.Errorf("Latin-1: %q", got)
	}
}

// ---------------------------------------------------------------------
// decodePropertyValue: round-trip for every application tag.
// ---------------------------------------------------------------------

func TestDecodePropertyValueBitString(t *testing.T) {
	c := newTestClient(t)
	// Encode in-alarm + fault (bits 0 and 1 set, others clear): 4 bits,
	// unused=4, byte=0xC0.
	bs := NewBitStringFromBits([]bool{true, true, false, false})
	wire := EncodeBitStringTag(bs)

	v, err := c.decodePropertyValue(wire)
	if err != nil {
		t.Fatalf("decodePropertyValue: %v", err)
	}
	got, ok := v.(BitString)
	if !ok {
		t.Fatalf("type: want BitString, got %T", v)
	}
	sf := got.StatusFlags()
	if !(sf.InAlarm && sf.Fault && !sf.Overridden && !sf.OutOfService) {
		t.Fatalf("StatusFlags: %+v", sf)
	}
}

func TestDecodePropertyValueDate(t *testing.T) {
	c := newTestClient(t)
	d := Date{Year: 2026, Month: 5, Day: 15, DayOfWeek: 5}
	wire := EncodeDateTag(d)

	v, err := c.decodePropertyValue(wire)
	if err != nil {
		t.Fatalf("decodePropertyValue: %v", err)
	}
	if got, ok := v.(Date); !ok || got != d {
		t.Fatalf("Date: want %+v, got %+v (type %T)", d, v, v)
	}
}

func TestDecodePropertyValueTime(t *testing.T) {
	c := newTestClient(t)
	tm := Time{Hour: 14, Minute: 30, Second: 45, Hundredths: 50}
	wire := EncodeTimeTag(tm)

	v, err := c.decodePropertyValue(wire)
	if err != nil {
		t.Fatalf("decodePropertyValue: %v", err)
	}
	if got, ok := v.(Time); !ok || got != tm {
		t.Fatalf("Time: want %+v, got %+v (type %T)", tm, v, v)
	}
}

func TestDecodePropertyValueAllScalars(t *testing.T) {
	c := newTestClient(t)

	cases := []struct {
		name string
		wire []byte
		want interface{}
	}{
		{"unsigned", EncodeUnsignedTag(1476), uint32(1476)},
		{"real", EncodeRealTag(72.5), float32(72.5)},
		{"enumerated", EncodeEnumeratedTag(3), uint32(3)},
		{"oid", EncodeObjectIdentifierTag(NewObjectIdentifier(ObjectTypeAnalogInput, 7)), NewObjectIdentifier(ObjectTypeAnalogInput, 7)},
		{"string", EncodeCharacterStringTag("Hello"), "Hello"},
		{"boolean-true", []byte{0x11}, true},
		{"boolean-false", []byte{0x10}, false},
		{"null", []byte{0x00}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.decodePropertyValue(tc.wire)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s: want %v (%T), got %v (%T)", tc.name, tc.want, tc.want, got, got)
			}
		})
	}
}

func TestDecodePropertyValueOctetStringIsCopied(t *testing.T) {
	// OctetString contents must be returned as a defensive copy — the
	// caller mustn't accidentally hold a slice into the receiver buffer.
	c := newTestClient(t)
	wire := []byte{0x65, 0x01, 0x02, 0x03} // OctetString tag (6), len=5 ext, value 1,2,3 — wait, build properly
	// Better: build via EncodeTag.
	wire = append(EncodeTag(uint8(TagOctetString), TagClassApplication, 3), 0x01, 0x02, 0x03)

	v, err := c.decodePropertyValue(wire)
	if err != nil {
		t.Fatalf("decodePropertyValue: %v", err)
	}
	got, ok := v.([]byte)
	if !ok {
		t.Fatalf("type: want []byte, got %T", v)
	}
	// Mutate the source after the call — the returned slice must not
	// have changed (i.e. it's a copy).
	for i := range wire {
		wire[i] = 0xFF
	}
	if !bytes.Equal(got, []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("OctetString aliasing: got %v after source mutation", got)
	}
}
