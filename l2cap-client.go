// TODO: Figure out about how to structure things for multiple
// OS / BLE interface configurations. Build tags? Subpackages?

package gatt

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"syscall"

	"github.com/davecgh/go-spew/spew"
)

// newL2cap uses s to provide l2cap access.
func newL2capClient(address string, publicAddress bool) (*l2capClient, error) {

	addressType := "random"
	if publicAddress {
		addressType = "public"
	}

	shim, err := newCShim("l2cap-ble-client", address, addressType)
	if err != nil {
		return nil, err
	}

	c := &l2capClient{
		shim:                   shim,
		readbuf:                bufio.NewReader(shim),
		scanner:                bufio.NewScanner(shim),
		mtu:                    23,
		address:                address,
		publicAddress:          publicAddress,
		commands:               make(chan *l2capClientCommand),
		data:                   make(chan []byte),
		connected:              make(chan struct{}),
		quit:                   make(chan struct{}),
		notification:           make(chan *Notification),
		currentCommandComplete: make(chan bool),
	}

	go c.eventloop()
	go c.commandLoop()

	return c, nil
}

type l2capClient struct {
	shim                   shim
	readbuf                *bufio.Reader
	scanner                *bufio.Scanner
	sendmu                 sync.Mutex // serializes writes to the shim
	mtu                    uint16
	address                string
	publicAddress          bool
	handles                *handleRange
	security               security
	serving                bool
	currentCommand         *l2capClientCommand
	currentCommandComplete chan bool
	commands               chan *l2capClientCommand
	quit                   chan struct{}
	connected              chan struct{}
	data                   chan []byte
	notification           chan *Notification
}

type l2capClientCommand struct {
	buffer        []byte
	callback      func([]byte)
	writeCallback func()
}

type Notification struct {
	Handle uint16
	Data   []byte
}

func (c *l2capClient) close() error {
	//close the c shim to close server
	err := c.shim.Signal(syscall.SIGINT)
	c.shim.Wait()

	c.shim.Close()
	if err != nil {
		println("Failed to send message to l2cap: ", err)
	}
	//call c.quit when close signal of shim arrives

	c.quit <- struct{}{}
	close(c.quit)
	//c.serving = false

	return nil
}

const ATT_OP_HANDLE_NOTIFY = 0x1b
const ATT_OP_HANDLE_IND = 0x1d

func (c *l2capClient) eventloop() error {
	for {
		//log.Printf("waiting for scanner data")
		c.scanner.Scan()
		err := c.scanner.Err()
		s := c.scanner.Text()

		log.Printf("l2cap-client Received: %s", s)

		if err != nil {
			//return err
		}

		f := strings.Fields(s)
		switch f[0] {
		case "bind":
			log.Printf("Got bind event: %s", f[1])
		case "connect":
			log.Printf("Got connect event: %s", f[1])
			if f[1] == "success" {
				c.connected <- struct{}{}
			}
		case "disconnect":
			log.Printf("Got disconnect event")
			c.close()
			return nil
		case "data":
			data, err := hex.DecodeString(f[1])
			if err != nil {
				log.Fatalf("Failed to parse l2cap-client hex data event: %s", s)
			}
			//log.Println("Got data event")

			commandId := data[0]

			switch commandId {
			case ATT_OP_HANDLE_NOTIFY, ATT_OP_HANDLE_IND:
				//log.Print("It's a Notification!")

				var handle uint16
				err := binary.Read(bytes.NewReader(data[1:3]), binary.LittleEndian, &handle)
				if err != nil {
					log.Fatalf("Failed to read notification handle : %s", err)
				}

				c.notification <- &Notification{
					Handle: handle,
					Data:   data[3:],
				}
			default:
				if c.currentCommand != nil {
					log.Print("There is a current command waiting for a response: %s", c.currentCommand)
					command := c.currentCommand
					c.currentCommandComplete <- true
					go command.callback(data)
				} else {
					log.Print("No-one cares about the data i just recieved!")
					spew.Dump(data)
				}
			}

		case "log":
			log.Printf("hci-l2cap-client: %s", s)
		default:
			log.Fatalf("unexpected event type: %s", s)
		}

	}
	return nil
}

func (c *l2capClient) commandLoop() error {
	for {
		c.currentCommand = nil

		command := <-c.commands

		log.Printf("Got a command to send: %s", command.buffer)

		c.send(command.buffer)

		if command.callback != nil {
			log.Printf("Command has a callback... waiting for data") // XXX timeout?
			c.currentCommand = command
			<-c.currentCommandComplete
		}

	}
	return nil
}

const GATT_PRIM_SVC_UUID = 0x2800

func (c *l2capClient) discoverServices() {
	c.queueCommand(&l2capClientCommand{
		buffer: makeReadByGroupRequest(0x0001, 0xffff, GATT_PRIM_SVC_UUID),
		callback: func(response []byte) {

			log.Printf("discoverServices response! %s", response)

			opcode := response[0]
			//i := 0

			if opcode == readByGroupResponse {
				dataType := uint8(response[1])
				num := uint8(len(response)-2) / dataType

				log.Printf("dataType:%x number of services:%d", dataType, num)
				log.Printf("TODO: Parse the rest")

				/*for (i = 0; i < num; i++) {
				  services.push({
				    startHandle: data.readUInt16LE(2 + i * dataType + 0),
				    endHandle: data.readUInt16LE(2 + i * dataType + 2),
				    uuid: (dataType == 6) ? data.readUInt16LE(2 + i * dataType + 4).toString(16) : data.slice(2 + i * dataType + 4).slice(0, 16).toString('hex').match(/.{1,2}/g).reverse().join('')
				  });
				}*/
			}

			/*if (opcode !== ATT_OP_READ_BY_GROUP_RESP || services[services.length - 1].endHandle === 0xffff) {
			    var serviceUuids = [];
			    for (i = 0; i < services.length; i++) {
			      if (uuids.length === 0 || uuids.indexOf(services[i].uuid) !== -1) {
			        serviceUuids.push(services[i].uuid);
			      }

			      this._services[services[i].uuid] = services[i];
			    }
			    this.emit('servicesDiscover', this._address, serviceUuids);
			  } else {
			    this._queueCommand(this.readByGroupRequest(services[services.length - 1].endHandle + 1, 0xffff, GATT_PRIM_SVC_UUID), callback);
			  }*/
		},
	})
}

func (c *l2capClient) queueCommand(command *l2capClientCommand) {
	log.Print("Queuing command")
	c.commands <- command
}

const GATT_CLIENT_CHARAC_CFG_UUID = 0x2902
const ATT_OP_READ_BY_TYPE_RESP = 0x09

func (c *l2capClient) notify(enable bool, startHandle uint16, endHandle uint16, useNotify bool, useIndicate bool) {

	c.queueCommand(&l2capClientCommand{
		buffer: makeReadByTypeRequest(startHandle, endHandle, GATT_CLIENT_CHARAC_CFG_UUID),
		callback: func(data []byte) {
			log.Printf("Got notify response %x", data)

			type Response struct {
				Opcode, DataType uint8
				Handle, Value    uint16
			}

			//XXX: Fix this dumb code.
			var response Response
			err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &response)
			if err != nil {
				log.Fatalf("Failed to read first notification response : %s", err)
			}

			if response.Opcode != ATT_OP_READ_BY_TYPE_RESP {
				log.Printf("Got the wrong response type in notify!! %x", response.Opcode) //XXX:
				return
			}

			if enable {
				if useNotify {
					response.Value |= 0x0001
				} else if useIndicate {
					response.Value |= 0x0002
				}
			} else {
				if useNotify {
					response.Value &= 0xfffe
				} else if useIndicate {
					response.Value &= 0xfffd
				}
			}

			buf := new(bytes.Buffer)
			err = binary.Write(buf, binary.LittleEndian, response.Value)
			if err != nil {
				log.Fatalf("Notify binary.Write failed: %s", err)
			}

			c.queueCommand(&l2capClientCommand{
				buffer: makeWriteRequest(response.Handle, buf.Bytes(), false),
				callback: func(data []byte) {
					log.Printf("Got notify final response %s", data)
					spew.Dump(data)
				},
			})
		},
	})
}

func (c *l2capClient) send(b []byte) error {
	if len(b) > int(c.mtu) {
		panic(fmt.Errorf("cannot send %x: mtu %d", b, c.mtu))
	}

	log.Printf("L2CAP-client: Sending %x", b)
	c.sendmu.Lock()
	_, err := fmt.Fprintf(c.shim, "%x\n", b)
	c.sendmu.Unlock()
	return err
}

/* Command Definitions XXX: Clean this up */

const readByGroupRequest uint8 = 0x10
const readByGroupResponse uint8 = 0x11

type readByGroupRequestPacket struct {
	startHandle uint16
	endHandle   uint16
	groupUuid   uint16
}

func makeReadByGroupRequest(startHandle uint16, endHandle uint16, groupUuid uint16) []byte {
	return encodeCommand(readByGroupRequest, &readByGroupRequestPacket{
		startHandle: startHandle,
		endHandle:   endHandle,
		groupUuid:   groupUuid,
	})
}

const readByTypeRequest uint8 = 0x08
const readByTypeResponse uint8 = 0x09

type readByTypeRequestPacket struct {
	startHandle uint16
	endHandle   uint16
	groupUuid   uint16
}

func makeReadByTypeRequest(startHandle uint16, endHandle uint16, groupUuid uint16) []byte {
	return encodeCommand(readByTypeRequest, &readByTypeRequestPacket{
		startHandle: startHandle,
		endHandle:   endHandle,
		groupUuid:   groupUuid,
	})
}

const writeRequest = 0x12
const writeCommand = 0x52

func makeWriteRequest(handle uint16, data []byte, withoutResponse bool) []byte {
	var command uint8
	if withoutResponse {
		command = writeCommand
	} else {
		command = writeRequest
	}
	return append(encodeCommand(command, handle), data...)
}

func encodeCommand(id uint8, command interface{}) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, id)

	err := binary.Write(buf, binary.LittleEndian, command)
	if err != nil {
		fmt.Printf("binary.Write failed encoding command:0x%x - %s", id, err)
	}

	return buf.Bytes()
}

func (c *l2capClient) disconnect() error {
	return c.shim.Signal(syscall.SIGHUP)
}
