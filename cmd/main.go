package main

import (
	"log/slog"
	"os"

	"github.com/vkl/rfidplayer/pkg/api"
	"github.com/vkl/rfidplayer/pkg/control"
	_ "github.com/vkl/rfidplayer/pkg/logging"
)

var (
	cardController    *control.CardController
	castController    *control.CastController
	chromecastControl *control.ChromecastControl
)

func init() {
	var err error
	cardController, err = control.NewCardController("cards.json")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	slog.Debug(cardController.FileName)

	castController, err = control.NewCastController("casts.json")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	slog.Debug(castController.FileName)

	chromecastControl = control.NewChromeCastControl(castController)
}

func main() {
	api.StartApp("0.0.0.0", 8080, cardController, chromecastControl)
}
