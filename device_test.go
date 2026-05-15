// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package bacnet

import (
	"net"
	"testing"
)

// TestNewRemoteDeviceDefaultPort confirms a device at DefaultPort uses the
// 4-byte BACnet address form (IP only, no encoded port).
func TestNewRemoteDeviceDefaultPort(t *testing.T) {
	dev := NewRemoteDevice(1234, net.ParseIP("192.168.1.50"), DefaultPort)

	if dev.ObjectID.Type != ObjectTypeDevice {
		t.Errorf("ObjectID.Type = %v, want ObjectTypeDevice", dev.ObjectID.Type)
	}
	if dev.ObjectID.Instance != 1234 {
		t.Errorf("ObjectID.Instance = %d, want 1234", dev.ObjectID.Instance)
	}
	if dev.MaxAPDULength != MaxAPDULength {
		t.Errorf("MaxAPDULength = %d, want %d", dev.MaxAPDULength, MaxAPDULength)
	}

	if len(dev.Address.Addr) != 4 {
		t.Fatalf("Address.Addr len = %d, want 4 (default-port form)", len(dev.Address.Addr))
	}
	want := []byte{192, 168, 1, 50}
	for i, b := range want {
		if dev.Address.Addr[i] != b {
			t.Errorf("Address.Addr[%d] = %d, want %d", i, dev.Address.Addr[i], b)
		}
	}
}

// TestNewRemoteDeviceCustomPort confirms a non-default port emits the
// 6-byte address form (4-byte IPv4 followed by 2-byte big-endian port).
func TestNewRemoteDeviceCustomPort(t *testing.T) {
	dev := NewRemoteDevice(42, net.ParseIP("10.0.0.1"), 47900)

	if len(dev.Address.Addr) != 6 {
		t.Fatalf("Address.Addr len = %d, want 6 (custom-port form)", len(dev.Address.Addr))
	}

	const port = 47900
	want := []byte{10, 0, 0, 1, byte(port >> 8), byte(port & 0xFF)}
	for i, b := range want {
		if dev.Address.Addr[i] != b {
			t.Errorf("Address.Addr[%d] = %d, want %d", i, dev.Address.Addr[i], b)
		}
	}
}

// TestNewRemoteDeviceIPv6 confirms a non-IPv4 address produces an empty
// Address so subsequent ReadProperty calls fail cleanly rather than send
// malformed packets.
func TestNewRemoteDeviceIPv6(t *testing.T) {
	dev := NewRemoteDevice(7, net.ParseIP("2001:db8::1"), DefaultPort)
	if len(dev.Address.Addr) != 0 {
		t.Errorf("IPv6 input should yield empty Address.Addr; got %x", dev.Address.Addr)
	}
}

// TestRegisterDeviceCachesEntry confirms RegisterDevice inserts an entry
// into the client's device cache that GetDevice can subsequently return.
func TestRegisterDeviceCachesEntry(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	dev := NewRemoteDevice(99, net.ParseIP("172.16.0.5"), DefaultPort)
	c.RegisterDevice(dev)

	got, ok := c.GetDevice(99)
	if !ok {
		t.Fatal("GetDevice(99) returned ok=false after RegisterDevice")
	}
	if got.ObjectID.Instance != 99 {
		t.Errorf("got.ObjectID.Instance = %d, want 99", got.ObjectID.Instance)
	}
}

// TestRegisterDeviceIsIdempotent confirms calling RegisterDevice twice with
// the same instance ID overwrites cleanly without error.
func TestRegisterDeviceIsIdempotent(t *testing.T) {
	c, _ := NewClient()
	c.RegisterDevice(NewRemoteDevice(1, net.ParseIP("10.0.0.1"), DefaultPort))
	c.RegisterDevice(NewRemoteDevice(1, net.ParseIP("10.0.0.2"), DefaultPort))

	got, _ := c.GetDevice(1)
	if got.Address.Addr[3] != 2 {
		t.Errorf("expected second RegisterDevice to win; got address %v", got.Address.Addr)
	}
}
