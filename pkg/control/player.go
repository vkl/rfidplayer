//go:build linux
// +build linux

package control

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/vkl/rfidplayer/pkg/logging"
	"github.com/warthog618/gpiod"
)

const (
	OPT_SENSOR_PIN      = 17
	ENCODER_PIN0        = 22
	ENCODER_PIN1        = 27
	RFID_RESET_PIN      = 26
	BTN_PIN             = 5
	RED_LED             = 19
	GREEN_LED           = 13
	BLUE_LED            = 6
	BTN_PLAY_PUSH_DELAY = 1000 * time.Millisecond
	BTN_NEXT_PUSH_DELAY = 3000 * time.Millisecond
	BTN_PREV_PUSH_DELAY = 6000 * time.Millisecond
)

type PlayerController struct {
	chromecastController *ChromecastControl
	cardController       *CardController
	mutex                sync.Mutex
	volume               int
	maxVolume            int
	event                chan interface{}
	ctx                  context.Context
	ledCtx               context.Context
	cancel               context.CancelFunc
	ledCancel            context.CancelFunc
	optPin               *gpiod.Line
	encPins              *gpiod.Lines
	rgbPins              *gpiod.Lines
	rfidResetPin         *gpiod.Line
	rfidController       *RfidController
	btnLastRisingTime    time.Duration
	btnLastFallenTime    time.Duration
}

type EncoderEvent struct{}

func (p *PlayerController) ReadAndPlayCard() {
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
	cardReady := make(chan bool)
	cardError := make(chan error)
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		for {
			if p.chromecastController.PlayCard(card) {
				goto DONE
			}
			select {
			case <-ctxTimeout.Done():
				cardError <- fmt.Errorf("could not play card '%s' by timeout", cardId.Repr())
				cancelTimeout()
				return
			default:
				time.Sleep(1000 * time.Millisecond)
			}
		}
	DONE:
		cancelTimeout()
		cardReady <- true
	}()
	ticker := time.NewTicker(500 * time.Millisecond)
	vals := make([]int, 3)
	for {
		select {
		case <-ticker.C:
			p.rgbPins.Values(vals)
			vals[0] ^= 1
			p.rgbPins.SetValues(vals)
		case <-cardReady:
			goto CARD_READY
		case err := <-cardError:
			slog.Error(err.Error())
			vals[0] = 0
			p.rgbPins.SetValues(vals)
			return
		}
	}
CARD_READY:
	volume, ok := p.chromecastController.GetVolume()
	if ok {
		p.volume = int(volume * 100)
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.Encoder()
	p.rgbPins.SetValues([]int{1, 0, 1})
}

func (p *PlayerController) OptSensorHandler(e gpiod.LineEvent) {
	switch e.Type {
	case gpiod.LineEventRisingEdge:
		slog.Debug("card inserted")
		p.ReadAndPlayCard()
	case gpiod.LineEventFallingEdge:
		slog.Debug("card pulled")
		p.rfidResetPin.SetValue(0)
		p.chromecastController.Control(STOP)
		if p.cancel != nil {
			p.cancel()
			p.cancel = nil
		}
		p.rgbPins.SetValues([]int{0, 1, 1})
	}
}

func (p *PlayerController) Encoder() {
	encCount := p.volume
	values := make([]int, 2)
	var encState, newState int
	for {
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

// One-button interface
func (p *PlayerController) BtnHandler(e gpiod.LineEvent) {
	if e.Type == gpiod.LineEventRisingEdge {
		p.btnLastRisingTime = e.Timestamp
		p.ledCtx, p.ledCancel = context.WithCancel(context.Background())
		go LedControl(p.ledCtx, p.rgbPins)
	} else if e.Type == gpiod.LineEventFallingEdge {
		btnPushTime := e.Timestamp - p.btnLastRisingTime
		if p.ledCancel != nil {
			p.ledCancel()
		}
		switch {
		case btnPushTime < BTN_PLAY_PUSH_DELAY:
			if p.chromecastController.CastStatus().MediaStatus == "PAUSED" {
				p.chromecastController.Control(PLAY)
				p.rgbPins.SetValues([]int{1, 0, 1})
			} else if p.chromecastController.CastStatus().MediaStatus == "PLAYING" ||
				p.chromecastController.CastStatus().MediaStatus == "BUFFERING" {
				p.chromecastController.Control(PAUSE)
				p.rgbPins.SetValues([]int{1, 1, 0})
			}
		case btnPushTime > BTN_PLAY_PUSH_DELAY && btnPushTime < BTN_NEXT_PUSH_DELAY:
			p.chromecastController.Control(NEXT)
			p.rgbPins.SetValues([]int{1, 0, 1})
		case btnPushTime > BTN_NEXT_PUSH_DELAY && btnPushTime < BTN_PREV_PUSH_DELAY:
			p.chromecastController.Control(PREV)
			p.rgbPins.SetValues([]int{1, 0, 1})
		default:
			p.rgbPins.SetValues([]int{1, 0, 1})
		}
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
		gpiod.WithBothEdges,
	)
	if err != nil {
		return nil, err
	}

	_, err = chip.RequestLine(
		BTN_PIN,
		gpiod.AsInput,
		gpiod.WithBothEdges,
		gpiod.WithEventHandler(player.BtnHandler),
		gpiod.LineBiasPullDown,
		gpiod.WithDebounce(10*time.Millisecond),
	)
	if err != nil {
		return nil, err
	}

	player.rgbPins, err = chip.RequestLines(
		[]int{RED_LED, GREEN_LED, BLUE_LED},
		gpiod.AsOutput(0, 1, 1),
	)

	// check if card already inserted
	// and read and play card
	val, _ := player.optPin.Value()
	if val == 1 {
		player.ReadAndPlayCard()
	}

	return player, nil

}

func LedControl(ctx context.Context, leds *gpiod.Lines) {
	ticker := time.NewTicker(10 * time.Millisecond)
	count := 0
	maxCount := 20
	start := time.Now()
	vals := make([]int, 3)
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start)
			count++
			switch {
			case elapsed < BTN_PLAY_PUSH_DELAY:
				continue
			case elapsed > BTN_PLAY_PUSH_DELAY && elapsed < BTN_NEXT_PUSH_DELAY:
			case elapsed > BTN_NEXT_PUSH_DELAY && elapsed < BTN_PREV_PUSH_DELAY:
				maxCount = 10
			default:
				vals[1] = 0
				leds.SetValues(vals)
				continue
			}
			if count >= maxCount {
				leds.Values(vals)
				vals[1] ^= 1
				leds.SetValues(vals)
				count = 0
			}
		case <-ctx.Done():
			return
		}
	}
}
