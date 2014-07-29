package gatt

import (
	"errors"
	"fmt"
	"sync"
	"log"
	"strings"
	"encoding/hex"
	"strconv"
	"github.com/davecgh/go-spew/spew"
)


// A Client is a GATT client. Client are single-shot types; once
// a Client has been closed, it cannot be restarted. Instead, create
// a new Client. Only one client may be running at a time.
// XXX: This description came from the Server class, might not make sense...
type Client struct {

	// HCI is the hci device to use, e.g. "hci1".
	// If HCI is "", an hci device will be selected
	// automatically.
	HCI string

	// Closed is an optional callback function that will be called
	// when the Client is closed. err will be any associated error.
	// If the server was closed by calling Close, err may be nil.
	Closed func(error)

	// StateChange is an optional callback function that will be called
	// when the client changes states.
	StateChange func(newState string)

	discoveries map[string]*discoveredDevice

	hci   *hciClient
	l2cap *l2cap

	quitonce sync.Once
	quit     chan struct{}
	err      error
}

type discoveredDevice struct {
	address       string
	addressType   string
	rssi          int8
	advertisement *deviceAdvertisement
}

type deviceAdvertisement struct {
	localName        string
	txPowerLevel     int8
	manufacturerData []byte
	serviceData      []*serviceData
}

type serviceData struct {
  uuid string
	data []byte
}

func (s *Client) Start() error {
	hciDevice := cleanHCIDevice(s.HCI)

	hciShim, err := newCShim("hci-ble-client", hciDevice)
	if err != nil {
		return err
	}

	s.hci = newHCIClient(hciShim)
	event, data, err := s.hci.event()
	if err != nil {
		return err
	}
	if (event != "adapterState") {
		return fmt.Errorf("unexpected hci event: %q", event)
	}
	if data == "unauthorized" {
		return errors.New("unauthorized; does l2cap-ble have the correct permissions?")
	}
	if data != "poweredOn" {
		return fmt.Errorf("unexpected adapter state: %q", data)
	}
	// TODO: If you kill and restart the server quickly, you get event
	// "unsupported". Waiting and then starting again fixes it.
	// Figure out why, and handle it automatically.

	s.StateChange(data)

	go func() {
		for {
			// No need to check s.quit here; if the users closes the server,
			// hci will get killed, which'll cause an error to be returned here.
			event, data, err := s.hci.event()
			log.Printf("event: %s data: %s err:%s", event, data, err)
			if err != nil {
				break
			}

			if event == "adapterState" && s.StateChange != nil {
				s.StateChange(data)
			} else if event == "event" {
				s.handleAdvertisingEvent(data)
			}
		}
		s.close(err)
	}()

	if s.Closed != nil {
		go func() {
			<-s.quit
			s.Closed(s.err)
		}()
	}

/*	l2capShim, err := newCShim("l2cap-ble", hciDevice)
	if err != nil {
		s.close(err)
		return err
	}

	s.l2cap = newL2cap(l2capShim)*/
	return nil
}

func (c *Client) handleAdvertisingEvent(data string) error {

	fields := strings.Split(data, ",");

	log.Printf("Advertising event! : %q", fields)

	address := fields[0]
	addressType := fields[1]
	eir, err := hex.DecodeString(fields[2])
	if err != nil {
		return fmt.Errorf("Failed to parse eir hex data: %s", fields[2])
	}
	rssi, err := strconv.ParseInt(fields[3], 10, 8);
	if err != nil {
		return fmt.Errorf("Failed to parse rssi: %s", fields[3])
	}

	if c.discoveries == nil {
		log.Printf("Initialising discovered devices")
		c.discoveries = make(map[string]*discoveredDevice)
	}

	device := c.discoveries[address]

	if device == nil {
		log.Printf("Discovered a new device: %s", address)
		device = &discoveredDevice{
			address: address,
			addressType: addressType,
		}
		device.advertisement = &deviceAdvertisement{}
		c.discoveries[address] = device
	}

	device.rssi = int8(rssi)

	//advertisement := device.advertisement;

	spew.Dump(device, address, addressType, eir, rssi);

//	log.Printf("Received hci data %s", data);



	return nil

}

func (s *Client) StartDiscovery() error {
	return s.hci.startDiscovery()
}

// Close stops the Client.
func (s *Client) Close() error {
	if !serving() {
		return errors.New("not serving")
	}
	err := s.hci.Close()
	s.hci.Wait()
	/*l2caperr := s.l2cap.close()
	if err == nil {
		err = l2caperr
	}*/
	s.close(err)
	//serverRunningMu.Lock()
	//serverRunning = false
	//serverRunningMu.Unlock()
	return err
}

func (s *Client) close(err error) {
	s.quitonce.Do(func() {
		s.err = err
		close(s.quit)
	})
}
