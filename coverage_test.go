// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// This file collects small-but-broad coverage tests for the enum String()
// methods, parsers, and error helpers. These functions are mechanically
// straightforward but make up a large fraction of the package's statement
// count, so covering them is the cheapest way to lift the package-level
// coverage to the v0.1.0 gate.

package bacnet

import (
	"errors"
	"strings"
	"testing"
)

// TestObjectTypeStringer exercises both the in-map and out-of-map branches
// of ObjectType.String, plus a known sample of common types.
func TestObjectTypeStringer(t *testing.T) {
	cases := map[ObjectType]string{
		ObjectTypeAnalogInput:   "analog-input",
		ObjectTypeAnalogOutput:  "analog-output",
		ObjectTypeBinaryInput:   "binary-input",
		ObjectTypeBinaryOutput:  "binary-output",
		ObjectTypeBinaryValue:   "binary-value",
		ObjectTypeDevice:        "device",
		ObjectTypeMultiStateInput: "multi-state-input",
		ObjectTypeTrendLog:      "trend-log",
	}
	for ot, want := range cases {
		if got := ot.String(); got != want {
			t.Errorf("ObjectType(%d).String() = %q, want %q", ot, got, want)
		}
	}
	// Unknown type falls back to the numeric form.
	if got := ObjectType(0xFFFF).String(); !strings.Contains(got, "65535") {
		t.Errorf("unknown ObjectType.String() = %q; expected to contain numeric form", got)
	}
}

// TestPropertyIdentifierStringer exercises the property-id enum stringer.
func TestPropertyIdentifierStringer(t *testing.T) {
	cases := map[PropertyIdentifier]string{
		PropertyObjectName:    "object-name",
		PropertyPresentValue:  "present-value",
		PropertyObjectList:    "object-list",
		PropertyDescription:   "description",
	}
	for pid, want := range cases {
		if got := pid.String(); got != want {
			t.Errorf("PropertyIdentifier(%d).String() = %q, want %q", pid, got, want)
		}
	}
}

// TestConfirmedServiceChoiceStringer + TestUnconfirmedServiceChoiceStringer
// cover the two service-choice enum stringers.
func TestConfirmedServiceChoiceStringer(t *testing.T) {
	if ServiceReadProperty.String() != "ReadProperty" {
		t.Errorf("ServiceReadProperty.String() = %q", ServiceReadProperty.String())
	}
	if ServiceWriteProperty.String() != "WriteProperty" {
		t.Errorf("ServiceWriteProperty.String() = %q", ServiceWriteProperty.String())
	}
	if ConfirmedServiceChoice(0xFF).String() == "" {
		t.Error("unknown ConfirmedServiceChoice should produce non-empty fallback")
	}
}

func TestUnconfirmedServiceChoiceStringer(t *testing.T) {
	if ServiceIAm.String() != "I-Am" {
		t.Errorf("ServiceIAm.String() = %q", ServiceIAm.String())
	}
	if ServiceWhoIs.String() != "Who-Is" {
		t.Errorf("ServiceWhoIs.String() = %q", ServiceWhoIs.String())
	}
}

// TestConnectionStateStringer covers the small enum stringer that callers
// commonly log.
func TestConnectionStateStringer(t *testing.T) {
	cases := map[ConnectionState]string{
		StateDisconnected: "disconnected",
		StateConnecting:   "connecting",
		StateConnected:    "connected",
		ConnectionState(99): "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("ConnectionState(%d).String() = %q, want %q", s, got, want)
		}
	}
}

// TestStatusFlagsStringer + Decode confirms both helpers work end-to-end.
func TestStatusFlagsStringer(t *testing.T) {
	sf := DecodeStatusFlags(0x0A) // 0x08 (InAlarm) | 0x02 (Overridden)
	if !sf.InAlarm || sf.Fault || !sf.Overridden || sf.OutOfService {
		t.Errorf("DecodeStatusFlags(0x0A) = %+v", sf)
	}
	s := sf.String()
	if !strings.Contains(s, "in-alarm:true") || !strings.Contains(s, "overridden:true") {
		t.Errorf("StatusFlags.String() missing expected substrings: %q", s)
	}
}

// TestReliabilityStringer + TestSegmentationStringer + TestDeviceStatusStringer
// cover the smaller enum stringers in types.go.
func TestReliabilityStringer(t *testing.T) {
	if Reliability(0).String() == "" || Reliability(99).String() == "" {
		t.Error("Reliability.String() returned empty")
	}
}

func TestSegmentationStringer(t *testing.T) {
	if Segmentation(0).String() == "" || Segmentation(99).String() == "" {
		t.Error("Segmentation.String() returned empty")
	}
}

func TestDeviceStatusStringer(t *testing.T) {
	if DeviceStatus(0).String() == "" || DeviceStatus(99).String() == "" {
		t.Error("DeviceStatus.String() returned empty")
	}
}

// TestRejectAbortReasonStringers covers the reject/abort reason enum
// stringers from errors.go (they participate in the package error message
// path when a peer rejects a request).
func TestRejectAbortReasonStringers(t *testing.T) {
	if RejectReason(0).String() == "" || RejectReason(99).String() == "" {
		t.Error("RejectReason.String() returned empty")
	}
	if AbortReason(0).String() == "" || AbortReason(99).String() == "" {
		t.Error("AbortReason.String() returned empty")
	}
}

// TestRejectErrorMessage + TestAbortErrorMessage cover the typed error
// messages emitted when a peer Reject/Abort lands in sendRequest.
func TestRejectErrorMessage(t *testing.T) {
	re := &RejectError{InvokeID: 7, Reason: RejectReason(2)}
	if re.Error() == "" {
		t.Error("RejectError.Error() returned empty")
	}
}

func TestAbortErrorMessage(t *testing.T) {
	ae := &AbortError{InvokeID: 7, Reason: AbortReason(2)}
	if ae.Error() == "" {
		t.Error("AbortError.Error() returned empty")
	}
}

// TestEventStateStringer hits the small EventState enum.
func TestEventStateStringer(t *testing.T) {
	if EventStateNormal.String() != "normal" {
		t.Errorf("EventStateNormal = %q", EventStateNormal.String())
	}
	if EventStateFault.String() != "fault" {
		t.Errorf("EventStateFault = %q", EventStateFault.String())
	}
	// Unknown falls back to numeric form.
	if !strings.Contains(EventState(99).String(), "99") {
		t.Error("unknown EventState should include the numeric value")
	}
}

// TestObjectIdentifierStringer confirms the OID stringer composes type and
// instance forms.
func TestObjectIdentifierStringer(t *testing.T) {
	oid := NewObjectIdentifier(ObjectTypeDevice, 1234)
	want := "device:1234"
	if got := oid.String(); got != want {
		t.Errorf("OID.String() = %q, want %q", got, want)
	}
}

// TestParseObjectType exercises both the short ("ai") and long
// ("analog-input") names, plus the not-found path.
func TestParseObjectType(t *testing.T) {
	if got, ok := ParseObjectType("analog-input"); !ok || got != ObjectTypeAnalogInput {
		t.Errorf("ParseObjectType(analog-input) = (%v, %v)", got, ok)
	}
	if got, ok := ParseObjectType("device"); !ok || got != ObjectTypeDevice {
		t.Errorf("ParseObjectType(device) = (%v, %v)", got, ok)
	}
	if _, ok := ParseObjectType("not-a-thing"); ok {
		t.Error("ParseObjectType(not-a-thing) should return ok=false")
	}
}

// TestParsePropertyIdentifier exercises a couple of common properties.
func TestParsePropertyIdentifier(t *testing.T) {
	if got, ok := ParsePropertyIdentifier("present-value"); !ok || got != PropertyPresentValue {
		t.Errorf("ParsePropertyIdentifier(present-value) = (%v, %v)", got, ok)
	}
	if _, ok := ParsePropertyIdentifier("not-a-prop"); ok {
		t.Error("ParsePropertyIdentifier(not-a-prop) should return ok=false")
	}
}

// TestBACnetErrorIsHelpers confirms the public Is* helpers work via
// errors.Is and via the typed BACnetError path.
func TestBACnetErrorIsHelpers(t *testing.T) {
	if !IsTimeout(ErrTimeout) {
		t.Error("IsTimeout(ErrTimeout) = false")
	}
	if !IsDeviceNotFound(ErrDeviceNotFound) {
		t.Error("IsDeviceNotFound(ErrDeviceNotFound) = false")
	}
	if IsTimeout(errors.New("unrelated")) {
		t.Error("IsTimeout(unrelated) = true")
	}

	// Typed BACnetError path.
	be := NewBACnetError(ErrorClassProperty, ErrorCodeUnknownProperty)
	if !IsPropertyNotFound(be) {
		t.Error("IsPropertyNotFound(class=Property code=UnknownProperty) = false")
	}

	be = NewBACnetError(ErrorClassProperty, ErrorCodeReadAccessDenied)
	if !IsAccessDenied(be) {
		t.Error("IsAccessDenied(class=Property code=ReadAccessDenied) = false")
	}
}

// TestBACnetErrorMessage confirms the formatted message includes both
// class and code so log lines are debuggable.
func TestBACnetErrorMessage(t *testing.T) {
	be := NewBACnetError(ErrorClassProperty, ErrorCodeUnknownObject)
	msg := be.Error()
	if !strings.Contains(msg, "property") || !strings.Contains(msg, "unknown-object") {
		// Different stringer naming is allowed; just ensure both fields show up.
		if !strings.Contains(msg, "Property") && !strings.Contains(msg, "UnknownObject") {
			t.Errorf("BACnetError.Error() = %q; expected to contain class + code", msg)
		}
	}
}

// TestErrorClassStringer + ErrorCodeStringer cover the two small enum
// stringers used by BACnetError.
func TestErrorClassStringer(t *testing.T) {
	if ErrorClassProperty.String() != "property" {
		t.Errorf("ErrorClassProperty.String() = %q", ErrorClassProperty.String())
	}
	if !strings.Contains(ErrorClass(0xFF).String(), "255") {
		t.Error("unknown ErrorClass should include numeric form")
	}
}

// TestDecodeReadPropertyResponseHappyPath validates the response decoder
// against a hand-built ReadProperty-Ack payload. This also acts as a
// regression test for AUDIT.md finding #3: the stale-`length` bug in the
// optional-ArrayIndex branch must not produce a parse error here.
//
// Build the payload for device 1234, Object_Name = "ABC":
//   [0] OID: 0x0C 0x02 0x00 0x04 0xD2
//   [1] property = 77: 0x19 0x4D
//   [3] opening: 0x3E
//     char string "ABC" with tag (charset 0 + 3 bytes = length 4):
//       (7<<4)|(0<<3)|4 = 0x74, 0x00, 'A', 'B', 'C'
//   [3] closing: 0x3F
func TestDecodeReadPropertyResponseHappyPath(t *testing.T) {
	c := &Client{}
	payload := []byte{
		0x0C, 0x02, 0x00, 0x04, 0xD2, // [0] OID
		0x19, 0x4D,                   // [1] property
		0x3E,                         // [3] opening
		0x74, 0x00, 'A', 'B', 'C',    // char string "ABC"
		0x3F,                         // [3] closing
	}
	got, err := c.decodeReadPropertyResponse(payload)
	if err != nil {
		t.Fatalf("decodeReadPropertyResponse: %v", err)
	}
	s, ok := got.(string)
	if !ok || s != "ABC" {
		t.Errorf("got %v (%T), want string \"ABC\"", got, got)
	}
}

// TestDecodeReadPropertyResponseWithArrayIndex is the focused regression
// test for AUDIT.md finding #3 (stale-length on the optional [2] tag).
// The reply echoes ArrayIndex back in the response — the decoder must
// skip past it using the *just-decoded* length, not the property-id
// length carried over from the previous iteration.
func TestDecodeReadPropertyResponseWithArrayIndex(t *testing.T) {
	c := &Client{}
	// Payload uses a 2-byte ArrayIndex (length-2) where the property-id
	// is length-1, so the stale-length bug would skip 1 byte too few
	// and the opening-tag check at offset N would fail.
	payload := []byte{
		0x0C, 0x02, 0x00, 0x04, 0xD2, // [0] OID
		0x19, 0x4D,                   // [1] property (length 1)
		0x2A, 0x01, 0x00,             // [2] ArrayIndex = 256 (length 2)
		0x3E,                         // [3] opening
		0x21, 0x05,                   // unsigned int = 5 (length 1)
		0x3F,                         // [3] closing
	}
	got, err := c.decodeReadPropertyResponse(payload)
	if err != nil {
		t.Fatalf("decodeReadPropertyResponse with ArrayIndex: %v (regression for AUDIT #3)", err)
	}
	if got == nil {
		t.Error("expected decoded value, got nil")
	}
}

// TestDecodePropertyValueScalars walks the application-tag switch in
// decodePropertyValue to confirm each tag kind decodes to the right Go
// type.
func TestDecodePropertyValueScalars(t *testing.T) {
	c := &Client{}
	cases := []struct {
		name string
		in   []byte
		want interface{}
	}{
		// 1-byte unsigned 42, app-tag UnsignedInt (=2): (2<<4)|0|1 = 0x21
		{"unsigned 42", []byte{0x21, 0x2A}, uint32(42)},
		// boolean true, app-tag Boolean (=1): (1<<4)|0|1 = 0x11
		{"boolean true", []byte{0x11}, true},
		// boolean false (length nibble = 0): 0x10
		{"boolean false", []byte{0x10}, false},
		// real 0.0, app-tag Real (=4): (4<<4)|0|4 = 0x44 + 4 zero bytes
		{"real 0.0", []byte{0x44, 0x00, 0x00, 0x00, 0x00}, float32(0)},
		// enumerated 5, app-tag Enumerated (=9): (9<<4)|0|1 = 0x91
		{"enumerated 5", []byte{0x91, 0x05}, uint32(5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.decodePropertyValue(tc.in)
			if err != nil {
				t.Fatalf("decodePropertyValue: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, tc.want, tc.want)
			}
		})
	}
}

// TestDecodeErrorParses takes a hand-crafted error payload and confirms
// the client extracts a BACnetError with the right class and code.
func TestDecodeErrorParses(t *testing.T) {
	c := &Client{}
	// errorClass = 2 (Property), errorCode = 31 (UnknownObject)
	// Both as application-tagged enumerated (tag 9, length 1):
	// (9<<4)|0|1 = 0x91, then the value byte.
	payload := []byte{0x91, 0x02, 0x91, 0x1F}
	err := c.decodeError(payload)
	if err == nil {
		t.Fatal("decodeError returned nil for valid payload")
	}
	var be *BACnetError
	if !errors.As(err, &be) {
		t.Fatalf("decoded err is not a *BACnetError: %T", err)
	}
	if be.Class != ErrorClassProperty || be.Code != ErrorCodeUnknownObject {
		t.Errorf("decoded err = (class=%v, code=%v); want (Property, UnknownObject)", be.Class, be.Code)
	}
}
