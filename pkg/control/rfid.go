//go:build linux
// +build linux

package control

import (
	"context"
	"fmt"
	"time"

	"go.bug.st/serial"
)

const (
	RFID_PACKET_LEN = 16
)

type RfidCardId []byte

func (rId RfidCardId) Repr() string {
	return fmt.Sprintf("%x", rId)
}

type RfidController struct {
	serialPort string
}

func NewRfidController(serialPort string) *RfidController {
	rfidController := RfidController{
		serialPort: serialPort,
	}
	return &rfidController
}

func (r *RfidController) ReadCardId(ctx context.Context) (RfidCardId, error) {
	mode := &serial.Mode{
		BaudRate: 9600,
	}
	port, err := serial.Open(r.serialPort, mode)
	defer port.Close()
	if err != nil {
		return nil, err
	}
	if err := port.SetReadTimeout(1 * time.Second); err != nil {
		return nil, err
	}
	buffer := make([]byte, 0)
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("could not read card by timeout")
		default:
			buf := make([]byte, 16)
			n, err := port.Read(buf)
			if err != nil {
				return nil, err
			}
			buffer = append(buffer, buf[0:n]...)
			if len(buffer) == RFID_PACKET_LEN {
				goto DONE
			}
		}
	}
DONE:
	return buffer[1:11], nil
}
