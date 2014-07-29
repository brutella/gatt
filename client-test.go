// +build ignore

package main

import (
  "log"
  "os"
  "os/signal"
  "github.com/ninjasphere/gatt"
  //"github.com/davecgh/go-spew/spew"
)

func main() {

  client := &gatt.Client{
    StateChange: func(newState string) {
      log.Println("Client state change: ", newState)
    },
    /*Advertisement: func(device *gatt.DiscoveredDevice) {
      log.Printf("Advertisement address:%s rssi:%d", device.Address, device.Rssi)
      spew.Dump(device);
    },*/
    Rssi: func(address string, rssi int8) {
      log.Printf("Rssi update address:%s rssi:%d", address, rssi)
      //spew.Dump(device);
    },
  }

  err := client.Start()

  if (err != nil) {
    log.Fatalf("Failed to start client: %s", err)
  }

  err = client.StartDiscovery();
  if (err != nil) {
    log.Fatalf("Failed to start discovery: %s", err)
  }

  c := make(chan os.Signal, 1)
  signal.Notify(c, os.Interrupt, os.Kill)

  // Block until a signal is received.
  s := <-c
  log.Println("Got signal:", s)
}
