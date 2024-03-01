//go:build linux
// +build linux

package control

import (
	"bufio"
	"context"
	"fmt"
	"log"

	"go.bug.st/serial"
)

const (
	DELIM           = 0x3
	RFID_PACKET_LEN = 16
)

type RfidCardId []byte

func (rId RfidCardId) Repr() string {
	return fmt.Sprintf("%x", rId)
}

type RfidController struct {
	serialPort string
	port       serial.Port
}

func NewRfidController(serialPort string) *RfidController {
	mode := &serial.Mode{
		BaudRate: 9600,
	}
	port, err := serial.Open(serialPort, mode)
	if err != nil {
		log.Fatal(err)
	}
	rfidController := RfidController{
		serialPort: serialPort,
		port:       port,
	}
	return &rfidController
}

func (r *RfidController) ReadCardId(ctx context.Context) (RfidCardId, error) {
	var err error
	var buffer []byte
	cardReady := make(chan bool)

	go func() {
		reader := bufio.NewReader(r.port)
		for {
			buffer, err = reader.ReadBytes(DELIM)
			if err != nil {
				return
			}
			if len(buffer) == RFID_PACKET_LEN {
				cardReady <- true
				return
			}
		}
	}()

	select {
	case <-ctx.Done(): // timeout
		err = fmt.Errorf("coult not read card by timeout")
		goto DONE
	case <-cardReady:
		goto DONE
	}
DONE:
	if err != nil {
		return nil, err
	}
	return buffer[1:11], nil
}
