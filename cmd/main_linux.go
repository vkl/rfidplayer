//go:build linux
// +build linux

package main

import (
	"log"

	"github.com/vkl/rfidplayer/pkg/control"
)

func init() {
	_, err := control.NewPlayerController(chromecastControl, cardController)
	if err != nil {
		log.Fatal(err)
	}
}
