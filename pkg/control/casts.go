package control

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sort"
	"sync"

	_ "github.com/vkl/rfidplayer/pkg/logging"
)

type Cast struct {
	Name   string `json:"name"`
	IPAddr net.IP
	Port   int
	Info   map[string]string
}

type Casts []Cast

func (c Casts) Len() int {
	return len(c)
}

func (c Casts) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}

func (c Casts) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

type CastController struct {
	FileName string
	Casts    Casts
	mutex    sync.Mutex
}

func NewCastController(fname string) (*CastController, error) {
	castController := &CastController{
		FileName: fname,
		Casts:    make(Casts, 0),
	}
	if err := castController.updateCastList(); err != nil {
		return &CastController{}, err
	}
	return castController, nil
}

func (c *CastController) updateCastList() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	f, err := os.OpenFile(c.FileName, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&c.Casts); err != nil {
		if err != io.EOF {
			return err
		}
	}
	return nil
}

func (c *CastController) GetCastByName(name string) (Cast, bool) {
	for _, cast := range c.Casts {
		if cast.Name == name {
			return cast, true
		}
	}
	return Cast{}, false
}

func (c *CastController) GetCasts() Casts {
	if err := c.updateCastList(); err != nil {
		slog.Error("", "error", err)
	}
	sort.Sort(c.Casts)
	return c.Casts
}

func (c *CastController) AddCast(cast Cast) error {
	if _, ok := c.GetCastByName(cast.Name); ok {
		return fmt.Errorf("the cast %s already exists", cast.Name)
	}
	c.Casts = append(c.Casts, cast)
	return c.save()
}

func (c *CastController) UpdateCast(cast Cast) error {
	if cast.Name == "" || cast.IPAddr == nil || cast.Port == 0 {
		return fmt.Errorf("cast validation error: %v", cast)
	}
	for i, value := range c.Casts {
		if cast.Name == value.Name {
			c.Casts[i] = cast
			return c.save()
		}
	}
	c.Casts = append(c.Casts, cast)
	return c.save()
}

func (c *CastController) DelCast(name string) bool {
	for i, cast := range c.Casts {
		if cast.Name == name {
			c.Casts = append(c.Casts[:i], c.Casts[i+1:]...)
			c.save()
			return true
		}
	}
	return false
}

func (c *CastController) save() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	f, err := os.OpenFile(c.FileName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(c.Casts); err != nil {
		return err
	}
	return nil
}
