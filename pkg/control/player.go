//go:build linux
// +build linux

package control

import (
	"context"
	"log/slog"
	"sync"
	"time"

	_ "github.com/vkl/rfidplayer/pkg/logging"
	"github.com/warthog618/gpiod"
)

const (
	OPT_SENSOR_PIN = 17
	ENCODER_PIN0   = 22
	ENCODER_PIN1   = 27
	RFID_RESET_PIN = 26
)

type PlayerController struct {
	chromecastController *ChromecastControl
	cardController       *CardController
	mutex                sync.Mutex
	volume               int
	maxVolume            int
	event                chan interface{}
	ctx                  context.Context
	cancel               context.CancelFunc
	optPin               *gpiod.Line
	encPins              *gpiod.Lines
	rfidResetPin         *gpiod.Line
	rfidController       *RfidController
}

type EncoderEvent struct{}

func (p *PlayerController) OptSensorHandler(e gpiod.LineEvent) {
	switch e.Type {
	case gpiod.LineEventRisingEdge:
		slog.Debug("card inserted")
		p.rfidResetPin.SetValue(1)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cardId, err := p.rfidController.ReadCardId(ctx)
		if err != nil {
			slog.Error(err.Error())
			return
		}
		slog.Debug(cardId.Repr())
		var card Card
		var ok bool
		if card, ok = p.cardController.Cards[cardId.Repr()]; !ok {
			slog.Warn("no such card", "cardId", cardId.Repr())
			return
		}
		p.maxVolume = int(card.MaxVolume * 100)
		p.chromecastController.PlayCard(card)
		volume, ok := p.chromecastController.GetVolume()
		if ok {
			p.volume = int(volume * 100)
		}
		p.ctx, p.cancel = context.WithCancel(context.Background())
		go p.Encoder()
	case gpiod.LineEventFallingEdge:
		slog.Debug("card pulled")
		p.rfidResetPin.SetValue(0)
		p.chromecastController.Control(STOP)
		if p.cancel != nil {
			p.cancel()
			p.cancel = nil
		}
	}
}

func (p *PlayerController) Encoder() {
	encCount := p.volume
	values := make([]int, 2)
	var encState, newState int
	for {
		// <-p.event
		select {
		case <-p.ctx.Done():
			slog.Debug("close encoder function")
			return
		default:
			p.encPins.Values(values)
			newState = values[0]<<1 | values[1]
			switch encState {
			case 2:
				if newState == 3 {
					encCount += 1
				}
				if newState == 0 {
					encCount -= 1
				}
			case 0:
				if newState == 2 {
					encCount += 1
				}
				if newState == 1 {
					encCount -= 1
				}
			case 1:
				if newState == 0 {
					encCount += 1
				}
				if newState == 3 {
					encCount -= 1
				}
			case 3:
				if newState == 1 {
					encCount += 1
				}
				if newState == 2 {
					encCount -= 1
				}
			}
			encState = newState
		}
		time.Sleep(1 * time.Millisecond)
		if encCount > p.maxVolume {
			encCount = p.maxVolume
		} else if encCount < 0 {
			encCount = 0
		}
		if encCount%5 == 0 && p.volume != encCount {
			p.volume = encCount
			p.chromecastController.SetVolume(float64(p.volume) / 100)
		}
	}
}

func (p *PlayerController) EncoderHandler() func(e gpiod.LineEvent) {
	return func(e gpiod.LineEvent) {
		p.event <- nil
	}
}

func NewPlayerController(
	chromecastController *ChromecastControl,
	cardController *CardController,
) (*PlayerController, error) {

	player := &PlayerController{
		chromecastController: chromecastController,
		cardController:       cardController,
		mutex:                sync.Mutex{},
		event:                make(chan interface{}),
		rfidController:       NewRfidController("/dev/serial0"),
	}

	// ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	// defer cancel()
	// rfidController.ReadCard(ctx)

	chip, _ := gpiod.NewChip("gpiochip0")
	slog.Debug("info", "lines", chip.Lines())
	var err error

	player.rfidResetPin, err = chip.RequestLine(
		RFID_RESET_PIN,
		gpiod.WithPullUp,
		gpiod.AsOutput(0),
	)

	player.optPin, err = chip.RequestLine(
		OPT_SENSOR_PIN,
		gpiod.WithPullUp,
		gpiod.WithEventHandler(player.OptSensorHandler),
		gpiod.WithBothEdges,
	)
	if err != nil {
		return nil, err
	}

	player.encPins, err = chip.RequestLines(
		[]int{ENCODER_PIN0, ENCODER_PIN1},
		gpiod.AsInput,
		gpiod.WithPullUp,
		gpiod.WithEventHandler(player.EncoderHandler()),
		gpiod.WithBothEdges,
	)
	if err != nil {
		return nil, err
	}

	return player, nil

}
