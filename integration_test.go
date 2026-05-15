// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

//go:build integration

// Integration tests against a live BACnet/IP simulator. Run with:
//
//	go test -tags=integration -race ./...
//
// Configuration via environment variables:
//
//	BACNET_SIM_HOST       (default: 127.0.0.1)
//	BACNET_SIM_PORT       (default: 47810)
//	BACNET_SIM_DEVICE_ID  (default: 1234)
//
// The defaults match the docker-compose setup in this repo's parent
// directory (`shamaan0086/bacnet-sim`, mapped to host port 47810).
//
// If the simulator is unreachable, every test in this file is skipped
// with a clear "sim not reachable" message — they do not fail. This
// keeps CI runs without Docker green and surfaces sim-down conditions
// without false alarms.

package bacnet

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

var (
	simInitOnce  sync.Once
	simReachable bool
	simSkipMsg   string
	simHost      string
	simPort      int
	simDeviceID  uint32
)

func loadSimConfig() {
	simHost = os.Getenv("BACNET_SIM_HOST")
	if simHost == "" {
		simHost = "127.0.0.1"
	}

	portStr := os.Getenv("BACNET_SIM_PORT")
	if portStr == "" {
		simPort = 47810
	} else {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			simPort = 47810
		} else {
			simPort = p
		}
	}

	devStr := os.Getenv("BACNET_SIM_DEVICE_ID")
	if devStr == "" {
		simDeviceID = 1234
	} else {
		d, err := strconv.ParseUint(devStr, 10, 32)
		if err != nil {
			simDeviceID = 1234
		} else {
			simDeviceID = uint32(d)
		}
	}
}

// probeSimulator runs once per test process: spin up a short-lived
// client, register the target device by IP, and try to read its
// Object_Name. If that fails we record an unreachable verdict that all
// subsequent integration tests can read and Skip on.
func probeSimulator(t *testing.T) {
	simInitOnce.Do(func() {
		loadSimConfig()

		client, err := NewClient(
			WithLocalAddress("127.0.0.1:0"),
			WithTimeout(3*time.Second),
		)
		if err != nil {
			simSkipMsg = "NewClient: " + err.Error()
			return
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.Connect(ctx); err != nil {
			simSkipMsg = "Connect: " + err.Error()
			return
		}

		client.RegisterDevice(NewRemoteDevice(simDeviceID, net.ParseIP(simHost), simPort))

		_, err = client.ReadProperty(ctx, simDeviceID,
			NewObjectIdentifier(ObjectTypeDevice, simDeviceID),
			PropertyObjectName,
		)
		if err != nil {
			simSkipMsg = "probe ReadProperty(Object_Name): " + err.Error()
			return
		}

		simReachable = true
	})

	if !simReachable {
		t.Skipf("BACnet simulator at %s:%d not reachable (%s). "+
			"Bring it up with `make sim-up` from the bacnet-go directory, "+
			"or run docker compose up -d from the project that hosts the "+
			"docker-compose.yaml.", simHost, simPort, simSkipMsg)
	}
}

// newConnectedClient is the shared fixture: a connected client with the
// sim's device pre-registered, ready for ReadProperty / RPM calls.
// Caller is responsible for Close().
func newConnectedClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(
		WithLocalAddress("127.0.0.1:0"),
		WithTimeout(3*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		c.Close()
		t.Fatalf("Connect: %v", err)
	}

	c.RegisterDevice(NewRemoteDevice(simDeviceID, net.ParseIP(simHost), simPort))
	return c
}

// TestIntegration_ConnectReadDisconnect is the smoke test: open a client,
// read the device's Object_Name, tear down. Asserts no error and a
// non-empty name.
func TestIntegration_ConnectReadDisconnect(t *testing.T) {
	probeSimulator(t)

	c := newConnectedClient(t)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	name, err := c.ReadProperty(ctx, simDeviceID,
		NewObjectIdentifier(ObjectTypeDevice, simDeviceID),
		PropertyObjectName,
	)
	if err != nil {
		t.Fatalf("ReadProperty(Object_Name): %v", err)
	}
	s, ok := name.(string)
	if !ok || s == "" {
		t.Fatalf("Object_Name = %v (%T); expected non-empty string", name, name)
	}
	t.Logf("device %d Object_Name = %q", simDeviceID, s)
}

// TestIntegration_ReadPropertyMultiple confirms a single round-trip can
// fetch three properties: Object_Name, Vendor_Name, Object_Identifier.
// Uses the BACnet-spec property list rather than relying on optional
// properties some simulators may not implement.
func TestIntegration_ReadPropertyMultiple(t *testing.T) {
	probeSimulator(t)

	c := newConnectedClient(t)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deviceOID := NewObjectIdentifier(ObjectTypeDevice, simDeviceID)
	values, err := c.ReadPropertyMultiple(ctx, simDeviceID, []ReadPropertyRequest{
		{ObjectID: deviceOID, PropertyID: PropertyObjectName},
		{ObjectID: deviceOID, PropertyID: PropertyVendorName},
		{ObjectID: deviceOID, PropertyID: PropertyObjectIdentifier},
	})
	if err != nil {
		t.Fatalf("ReadPropertyMultiple: %v", err)
	}
	if len(values) < 3 {
		t.Fatalf("ReadPropertyMultiple returned %d values, want at least 3", len(values))
	}
	for _, pv := range values {
		t.Logf("  %s.%s = %v", pv.ObjectID, pv.PropertyID, pv.Value)
	}
}

// TestIntegration_ObjectListLength reads the Object_List property of the
// device with ArrayIndex=0, which by spec returns the list length as a
// uint. Asserts the count is positive — most simulators expose at least
// the Device object plus a handful of analog/binary objects.
func TestIntegration_ObjectListLength(t *testing.T) {
	probeSimulator(t)

	c := newConnectedClient(t)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Per BACnet spec, ArrayIndex=0 returns the array length.
	val, err := c.ReadProperty(ctx, simDeviceID,
		NewObjectIdentifier(ObjectTypeDevice, simDeviceID),
		PropertyObjectList,
		WithArrayIndex(0),
	)
	if err != nil {
		t.Fatalf("ReadProperty(Object_List, ArrayIndex=0): %v", err)
	}

	// The simulator should report at least 1 object (itself).
	switch v := val.(type) {
	case uint32:
		if v < 1 {
			t.Errorf("Object_List length = %d, want >= 1", v)
		} else {
			t.Logf("device %d has %d objects", simDeviceID, v)
		}
	default:
		t.Errorf("Object_List length type = %T, want uint32", val)
	}
}

// TestIntegration_ReconnectAfterDisconnect runs three full Connect/Close
// cycles to confirm the receiver-goroutine and pending-map lifecycle is
// stable across teardowns. This is the regression test for the
// send-on-closed-channel race fixed in AUDIT #2.
func TestIntegration_ReconnectAfterDisconnect(t *testing.T) {
	probeSimulator(t)

	for i := 0; i < 3; i++ {
		c, err := NewClient(
			WithLocalAddress("127.0.0.1:0"),
			WithTimeout(3*time.Second),
		)
		if err != nil {
			t.Fatalf("cycle %d NewClient: %v", i, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := c.Connect(ctx); err != nil {
			cancel()
			t.Fatalf("cycle %d Connect: %v", i, err)
		}

		c.RegisterDevice(NewRemoteDevice(simDeviceID, net.ParseIP(simHost), simPort))
		_, err = c.ReadProperty(ctx, simDeviceID,
			NewObjectIdentifier(ObjectTypeDevice, simDeviceID),
			PropertyObjectName,
		)
		cancel()
		if err != nil {
			c.Close()
			t.Fatalf("cycle %d ReadProperty: %v", i, err)
		}

		if err := c.Close(); err != nil {
			t.Fatalf("cycle %d Close: %v", i, err)
		}
		if c.State() != StateDisconnected {
			t.Errorf("cycle %d post-Close state = %v, want Disconnected", i, c.State())
		}
	}
}

// TestIntegration_PingLatency asserts a single ReadProperty round-trip
// against localhost completes in under 100 ms. Lets us catch egregious
// regressions in the request/response path (e.g. introducing a per-RPC
// 100 ms sleep). The 100 ms ceiling is generous for a loopback BACnet
// transaction; real measured latency on a healthy sim is single-digit
// milliseconds.
func TestIntegration_PingLatency(t *testing.T) {
	probeSimulator(t)

	c := newConnectedClient(t)
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Warm-up: one ReadProperty to populate any caches before measuring.
	_, _ = c.ReadProperty(ctx, simDeviceID,
		NewObjectIdentifier(ObjectTypeDevice, simDeviceID),
		PropertyObjectName,
	)

	start := time.Now()
	_, err := c.ReadProperty(ctx, simDeviceID,
		NewObjectIdentifier(ObjectTypeDevice, simDeviceID),
		PropertyObjectName,
	)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ReadProperty: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("ReadProperty round-trip = %v, want < 100ms", elapsed)
	}
	t.Logf("ReadProperty(loopback) latency: %v", elapsed)
}

// silence the unused-import nag in environments where errors is only
// referenced from `errors.As` paths that other build tags strip out.
var _ = errors.New
