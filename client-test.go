// +build ignore

package main

import (
  "log"
  "os"
  "os/signal"
  "github.com/ninjasphere/gatt"
  "github.com/davecgh/go-spew/spew"
)

func main() {

  client := &gatt.Client{
    StateChange: func(newState string) {
      log.Println("Client state change: ", newState)
    },
    Discover: func(device *gatt.DiscoveredDevice, final bool) {
      log.Printf("Discovered device final:%t", final)
      spew.Dump(device);
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
