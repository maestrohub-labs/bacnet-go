// Copyright 2026 maestrohub-labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

// Command read demonstrates the minimal flow to discover a BACnet device on
// the local network and read its Object_Name property.
//
// Usage:
//
//	go run ./examples/read
//
// The example broadcasts a Who-Is to the local subnet, waits up to 5 seconds
// for I-Am responses, and then reads Object_Name from the first device found.
// It is intended as a smoke test against a local BACnet simulator.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	bacnet "github.com/maestrohub-labs/bacnet-go"
)

func main() {
	client, err := bacnet.NewClient(
		bacnet.WithTimeout(3 * time.Second),
		bacnet.WithRetries(3),
	)
	if err != nil {
		log.Fatalf("new client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()

	devices, err := client.WhoIs(ctx, bacnet.WithDiscoveryTimeout(5*time.Second))
	if err != nil {
		log.Fatalf("WhoIs: %v", err)
	}
	if len(devices) == 0 {
		fmt.Println("no devices discovered")
		return
	}

	for _, dev := range devices {
		fmt.Printf("device %d at %v (vendor %d)\n",
			dev.ObjectID.Instance, dev.Address, dev.VendorID)
	}

	target := devices[0].ObjectID.Instance
	name, err := client.ReadProperty(ctx, target,
		bacnet.NewObjectIdentifier(bacnet.ObjectTypeDevice, target),
		bacnet.PropertyObjectName,
	)
	if err != nil {
		log.Fatalf("ReadProperty(device %d, Object_Name): %v", target, err)
	}
	fmt.Printf("device %d Object_Name = %v\n", target, name)
}
