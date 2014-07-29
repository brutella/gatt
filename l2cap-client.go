// TODO: Figure out about how to structure things for multiple
// OS / BLE interface configurations. Build tags? Subpackages?

package gatt

import (
  "bufio"
  "errors"
  "strings"
  "sync"
  "syscall"
  "log"
)

// newL2cap uses s to provide l2cap access.
func newL2capClient(s shim) *l2cap {
  c := &l2cap{
    shim:    s,
    readbuf: bufio.NewReader(s),
    scanner: bufio.NewScanner(s),
    mtu:     23,
  }
  return c
}

type l2capClient struct {
  shim     shim
  readbuf  *bufio.Reader
  scanner  *bufio.Scanner
  sendmu   sync.Mutex // serializes writes to the shim
  mtu      uint16
  handles  *handleRange
  security security
  serving  bool
  quit     chan struct{}
}

func (c *l2capClient) start() error {
  if c.serving {
    return errors.New("already serving")
  }
  c.serving = true
//	return c.eventloop()
  go c.eventloop()
  return nil
}

func (c *l2capClient) close() error {
  if !c.serving {
    return errors.New("not serving")
  }
  //close the c shim to close server
  err := c.shim.Signal(syscall.SIGINT)
  c.shim.Wait()
  c.serving = false

  //c.shim.Close()
  if err != nil {
    println("Failed to send message to l2cap: ", err)
  }
  //call c.quit when close signal of shim arrives
  /*
  c.quit <- struct{}{}
  close(c.quit)*/
  //c.serving = false

  return nil
}

func (c *l2capClient) eventloop() error {
  for {
    c.scanner.Scan()
    err := c.scanner.Err()
    s := c.scanner.Text()

    log.Printf("l2cap-client Received: %s", s)

    if err != nil {
      //return err
    }

    f := strings.Fields(s)
    if len(f) == 0 {
      continue
    } else if len(f) < 2 && f[0] != "close" {
      continue
    }

  }
  return nil
}

func (c *l2capClient) disconnect() error {
  return c.shim.Signal(syscall.SIGHUP)
}
