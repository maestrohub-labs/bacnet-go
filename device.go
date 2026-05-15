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
)

// NewRemoteDevice constructs a DeviceInfo for a BACnet/IP device whose
// IP address, port, and Device-Instance number are known out of band —
// i.e. without going through WhoIs/I-Am discovery.
//
// The returned DeviceInfo can be passed to (*Client).RegisterDevice to
// populate the client's internal device cache, after which ReadProperty
// and ReadPropertyMultiple calls keyed on deviceID resolve directly to
// this address without issuing a Who-Is broadcast.
//
// If ip is not a 4-byte IPv4 address (or an IPv4-in-IPv6 representation
// that .To4() can reduce), the returned DeviceInfo has an empty Address,
// and subsequent ReadProperty calls will fail with "invalid device
// address format". BACnet/IPv6 is not supported.
//
// MaxAPDULength defaults to MaxAPDULength (1476). Override the returned
// struct's MaxAPDULength field if the target device negotiated a smaller
// value during its earlier I-Am.
func NewRemoteDevice(deviceID uint32, ip net.IP, port int) DeviceInfo {
	addr := Address{Net: 0}
	if ip4 := ip.To4(); ip4 != nil {
		if port == DefaultPort {
			// 4-byte form: server uses DefaultPort.
			a := make([]byte, 4)
			copy(a, ip4)
			addr.Addr = a
		} else {
			// 6-byte form: IP + 2-byte big-endian port.
			a := make([]byte, 6)
			copy(a[:4], ip4)
			a[4] = byte(port >> 8)
			a[5] = byte(port)
			addr.Addr = a
		}
	}

	return DeviceInfo{
		ObjectID:      ObjectIdentifier{Type: ObjectTypeDevice, Instance: deviceID},
		Address:       addr,
		MaxAPDULength: MaxAPDULength,
	}
}

// RegisterDevice inserts a known DeviceInfo into the client's device
// cache, allowing ReadProperty / ReadPropertyMultiple calls to resolve
// the device's address without first issuing a Who-Is. Pair with
// NewRemoteDevice when you know the device IP and instance number out
// of band.
//
// Idempotent: calling RegisterDevice twice with the same instance ID
// overwrites the prior entry. Returns immediately; does not contact the
// network.
//
// Note: RegisterDevice does not populate MaxAPDULength/Segmentation/
// VendorID via I-Am. Callers that need negotiated APDU sizing should
// call Connect followed by a single targeted WhoIs(ctx,
// WithDeviceRange(id, id)) instead.
func (c *Client) RegisterDevice(device DeviceInfo) {
	d := device
	c.devicesMu.Lock()
	c.devices[d.ObjectID.Instance] = &d
	c.devicesMu.Unlock()
}
