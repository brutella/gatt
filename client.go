package gatt

import (
	"errors"
	"fmt"
	"sync"
	"log"
	"strings"
	"encoding/hex"
	"strconv"
	//"bytes"
	"github.com/davecgh/go-spew/spew"
	//"encoding/binary"
	//"os"
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

	Discover func(device *DiscoveredDevice, final bool)

	discoveries map[string]*DiscoveredDevice

	hci   *hciClient
	l2cap *l2cap

	quitonce sync.Once
	quit     chan struct{}
	err      error
}

type DiscoveredDevice struct {
	Address        string
	AddressType    string
	Rssi           int8
	Advertisement  DeviceAdvertisement
	discoveryCount int
}

type DeviceAdvertisement struct {
	LocalName        string
	TxPowerLevel     int8
	ManufacturerData []byte
	ServiceData      []ServiceData
	ServiceUuids     map[string]bool
}

type ServiceData struct {
  Uuid string
	Data []byte
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

	if s.StateChange != nil {
		s.StateChange(data)
	}

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
		c.discoveries = make(map[string]*DiscoveredDevice)
	}

	device := c.discoveries[address]

	if device == nil {
		log.Printf("Discovered a new device: %s", address)
		device = &DiscoveredDevice{
			Address: address,
			AddressType: addressType,
			Advertisement: DeviceAdvertisement{
				ServiceUuids: make(map[string]bool),
				ServiceData: make([]ServiceData, 0),
			},
		}
		c.discoveries[address] = device
	}

	device.Rssi = int8(rssi)

	advertisement := &device.Advertisement;

	i := 0

	log.Printf("eir length: %d", len(eir))

	for (i+1) < len(eir) {

		length := int(eir[i])
		dataType := int(eir[i+1]) // https://www.bluetooth.org/en-us/specification/assigned-numbers/generic-access-profile

		log.Printf("Reading eir data length:%d type:%d from pos:%d", length, dataType, i)

		if (i + length + 1) > len(eir) {
      log.Printf("Invalid EIR data, out of range of buffer length");
      break;
    }

		payload := eir[i+2:i+2+length-1]

		log.Printf("Payload dump:")
		spew.Dump(payload)

		switch dataType {
			case 0x01: // Flags
				/*
				  Bluetooth Core Specification:
				  Vol. 3, Part C, section 8.1.3 (v2.1 + EDR, 3.0 + HS and 4.0)
				  Vol. 3, Part C, sections 11.1.3 and 18.1 (v4.0)
				  Core Specification Supplement, Part A, section 1.3
				*/
				log.Printf("EIR Flags: %s", strconv.FormatInt(int64(payload[0]), 2))

			case 0x02, // Incomplete List of 16-bit Service Class UUID
					 0x03, // Complete List of 16-bit Service Class UUIDs
           0x06, // Incomplete List of 128-bit Service Class UUIDs
           0x07: // Complete List of 128-bit Service Class UUIDs

				uuidLength := 2; // 16-bit
				if dataType > 0x03 {
					uuidLength = 16 // 128-bit
				}

				for j := 0; j < len(payload); j += uuidLength {
          serviceUuid := hex.EncodeToString(payload[j:j+uuidLength])
					advertisement.ServiceUuids[serviceUuid] = true
				}

			case 0x08, // Shortened Local Name
			     0x09: // Complete Local Name
				advertisement.LocalName = string(payload)

			case 0x0a: // Tx Power Level
        advertisement.TxPowerLevel = int8(payload[0])

			case 0x16: // Service Data, there can be multiple occurences
				advertisement.ServiceData = append(advertisement.ServiceData, ServiceData{
					Uuid: hex.EncodeToString(payload[0:2]),
					Data: payload[3:],
				})

			case 0xff: // Manufacturer Specific Data
        advertisement.ManufacturerData = payload

			default:
				log.Printf("Unhandled eir data type: 0x%x", dataType)
		}

		i += (length + 1)
	}

	device.discoveryCount += 1

	if c.Discover != nil {
		c.Discover(device, device.discoveryCount % 2 == 0)
	}

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
