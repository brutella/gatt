// +build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"os/signal"

	"github.com/davecgh/go-spew/spew"
	"github.com/ninjasphere/gatt"
)

type WaypointPayload struct {
	Sequence    uint8
	AddressType uint8
	Rssi        int8
	Valid       uint8
}

func main() {

	client := &gatt.Client{
		StateChange: func(newState string) {
			log.Println("Client state change: ", newState)
		},
		/*Rssi: func(address string, rssi int8) {
		  log.Printf("Rssi update address:%s rssi:%d", address, rssi)
		  //spew.Dump(device);
		},*/
	}

	/*
	  Waypoint notification characteristic {
	    "startHandle": 45,
	    "properties": 16, (useNotify = true, useIndicate = false)
	    "valueHandle": 46,
	    "uuid": "fff4",
	    "endHandle": 48,
	  }
	*/

	client.Advertisement = func(device *gatt.DiscoveredDevice) {
		log.Printf("Advertisement address:%s rssi:%d", device.Address, device.Rssi)

		if device.Advertisement.LocalName != "Ninja Sphere Tag" {
			err := client.Connect(device.Address, device.PublicAddress)
			if err != nil {
				log.Printf("Connect error:%s", err)
			}
		}

		device.Connected = func() {
			log.Print("Connected to:")
			spew.Dump(device.Advertisement)

			// XXX: Yes, magic numbers.... this enables the notification from our Waypoints
			client.Notify(device.Address, true, 45, 48, true, false)
		}

		device.Notification = func(notification *gatt.Notification) {
			log.Printf("Got the notification!")

			//XXX: Add the ieee into the payload somehow??
			var payload WaypointPayload
			err := binary.Read(bytes.NewReader(notification.Data), binary.LittleEndian, &payload)
			if err != nil {
				log.Fatalf("Failed to read waypoint payload : %s", err)
			}

			ieee := notification.Data[5:]

			spew.Dump("ieee:", ieee, payload)
		}

	}

	err := client.Start()

	if err != nil {
		log.Fatalf("Failed to start client: %s", err)
	}

	err = client.StartScanning(false)
	if err != nil {
		log.Fatalf("Failed to start scanning: %s", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	s := <-c
	log.Println("Got signal:", s)
}
