// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package bacnet

import (
	"testing"
)

// TestObjectIdentifierEncoding round-trips a (type, instance) pair through
// the BACnet 32-bit encoding: 10 bits of type in the high half, 22 bits of
// instance in the low half.
func TestObjectIdentifierEncoding(t *testing.T) {
	cases := []struct {
		name     string
		objType  ObjectType
		instance uint32
		want     uint32
	}{
		{"analog-input 1", ObjectTypeAnalogInput, 1, (0 << 22) | 1},
		{"device 1234", ObjectTypeDevice, 1234, (8 << 22) | 1234},
		{"max instance", ObjectTypeBinaryOutput, 0x3FFFFF, (4 << 22) | 0x3FFFFF},
		{"vendor type", ObjectType(512), 7, (512 << 22) | 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oid := NewObjectIdentifier(tc.objType, tc.instance)
			got := oid.Encode()
			if got != tc.want {
				t.Errorf("Encode((%d, %d)) = 0x%08x, want 0x%08x",
					tc.objType, tc.instance, got, tc.want)
			}

			back := DecodeObjectIdentifier(got)
			if back.Type != tc.objType || back.Instance != tc.instance {
				t.Errorf("Decode(0x%08x) = (%d, %d), want (%d, %d)",
					got, back.Type, back.Instance, tc.objType, tc.instance)
			}
		})
	}
}

// TestObjectIdentifierInstanceMasking confirms instance values are masked
// to the low 22 bits on encode (spec: instance field is 22 bits wide).
func TestObjectIdentifierInstanceMasking(t *testing.T) {
	// Instance with high bits set beyond 22 bits gets truncated.
	oid := NewObjectIdentifier(ObjectTypeDevice, 0xFFFFFFFF)
	encoded := oid.Encode()
	// Low 22 bits = 0x3FFFFF, type = 8 in upper 10 bits
	want := (uint32(8) << 22) | 0x3FFFFF
	if encoded != want {
		t.Errorf("Encode masking failed: got 0x%08x, want 0x%08x", encoded, want)
	}
}

// TestArrayAllConstant verifies the BACnet ArrayAll sentinel matches the
// spec value of 0xFFFFFFFF.
func TestArrayAllConstant(t *testing.T) {
	if ArrayAll != 0xFFFFFFFF {
		t.Errorf("ArrayAll = 0x%08x, want 0xFFFFFFFF", uint32(ArrayAll))
	}
}
