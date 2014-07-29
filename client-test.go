// +build ignore

package main

import (
  "log"
  "os"
  "os/signal"
  "github.com/ninjasphere/gatt"
)

func main() {

  client := &gatt.Client{
    StateChange: func(newState string) {
      log.Println("Client state change: ", newState)
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
