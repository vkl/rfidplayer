package control

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"sync"

	_ "github.com/vkl/rfidplayer/pkg/logging"
)

type Card struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	MediaLinks []struct {
		Link        string `json:"link"`
		ContentType string `json:"content_type"`
	} `json:"media_links"`
	Chromecast string  `json:"chromecast"`
	MaxVolume  float64 `json:"maxvolume"`
}

type CardController struct {
	FileName string
	Cards    map[string]Card
	mutex    sync.Mutex
}

func NewCardController(fname string) (*CardController, error) {
	cardController := &CardController{
		FileName: fname,
		Cards:    make(map[string]Card, 0),
	}
	if err := cardController.updateCardList(); err != nil {
		return &CardController{}, err
	}
	return cardController, nil
}

func (c *CardController) updateCardList() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	f, err := os.OpenFile(c.FileName, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&c.Cards); err != nil {
		if err != io.EOF {
			return err
		}
	}
	return nil
}

func (c *CardController) GetCards() map[string]Card {
	if err := c.updateCardList(); err != nil {
		slog.Error("", "error", err)
	}
	return c.Cards
}

func (c *CardController) AddCard(card Card) error {
	c.Cards[card.Id] = card
	return c.save()
}

func (c *CardController) DelCard(id string) bool {
	if _, ok := c.Cards[id]; !ok {
		return false
	}
	delete(c.Cards, id)
	c.save()
	return true
}

func (c *CardController) save() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	f, err := os.OpenFile(c.FileName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(c.Cards); err != nil {
		return err
	}
	return nil
}
