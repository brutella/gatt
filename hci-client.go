package gatt

import (
	"bufio"
	"errors"
	"log"
	"strings"
	"syscall"
)

func newHCIClient(s shim) *hciClient {
	c := &hciClient{
		shim:    s,
		readbuf: bufio.NewReader(s),
	}
	return c
}

type hciClient struct {
	shim
	readbuf *bufio.Reader
}

// event returns the next available HCI event, blocking if needed.
func (c *hciClient) event() (string, string, error) {
	for {
		s, err := c.readbuf.ReadString('\n')
		if err != nil {
			return "", "", err
		}
		// log.Print("hci:" + s)

		f := strings.Fields(s)
		if f[0] != "log" && len(f) != 2 {
			return "", "", errors.New("badly formed event: " + s)
		}
		switch f[0] {
		case "adapterState":
			return f[0], f[1], nil
		case "hciDeviceId":
			// log.Printf("HCI device id %s", f[1])
			continue
		case "log":
			log.Printf("hci-ble-client: %s", s)
			continue
		case "event":
			return f[0], f[1], nil
		default:
			return "", "", errors.New("unexpected event type: " + s)
		}
	}
}

func (c *hciClient) startScanning(allowDuplicates bool) error {
	log.Printf("hci-client Starting discovery")
	if allowDuplicates {
		return c.shim.Signal(syscall.SIGUSR2)
	} else {
		return c.shim.Signal(syscall.SIGUSR1)
	}
}

func (c *hciClient) stopScanning() error {
	log.Printf("hci-client Starting discovery")
	return c.shim.Signal(syscall.SIGHUP)
}
