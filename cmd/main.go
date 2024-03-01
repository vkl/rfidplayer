package main

import (
	"fmt"
	"log"

	"github.com/vkl/rfidplayer/pkg/api"
	"github.com/vkl/rfidplayer/pkg/control"
	"github.com/vkl/rfidplayer/pkg/logging"
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
		log.Fatal(err)
	}
	logging.Log.Debug(cardController.FileName)

	castController, err = control.NewCastController("casts.json")
	if err != nil {
		log.Fatal(err)
	}
	logging.Log.Debug(castController.FileName)

	chromecastControl = control.NewChromeCastControl(castController)
}

func main() {
	fmt.Println("Starting app")
	api.StartApp("0.0.0.0", 8080, cardController, chromecastControl)
}
