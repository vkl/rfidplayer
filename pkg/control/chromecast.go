package control

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/vkl/go-cast"
	"github.com/vkl/go-cast/api"
	"github.com/vkl/go-cast/controllers"
	"github.com/vkl/go-cast/discovery"
	"github.com/vkl/go-cast/events"
	"github.com/vkl/rfidplayer/pkg/logging"
)

const DISCOVERY_TIMEOUT = 10

type ClientAction struct {
	Action string  `json:"action"`
	Volume float64 `json:"volume"`
}

type ChromecastClient struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	MediaStatus string  `json:"media_status"`
	MediaData   string  `json:"media_data"`
	Volume      float64 `json:"volume"`
}

type ChromecastClients []*ChromecastClient

func (c ChromecastClients) Len() int {
	return len(c)
}

func (c ChromecastClients) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}

func (c ChromecastClients) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

type ChromecastControl struct {
	discoveryService  *discovery.Service
	clients           map[string]*cast.Client
	chromecastClients ChromecastClients
	isDiscovering     bool
	mutex             sync.Mutex
}

func NewChromeCastControl() *ChromecastControl {
	chromecastControl := ChromecastControl{
		clients:           make(map[string]*cast.Client, 0),
		chromecastClients: make(ChromecastClients, 0),
		isDiscovering:     false,
	}
	return &chromecastControl
}

func (cc *ChromecastControl) StartDiscovery(timeout time.Duration) {
	if cc.isDiscovering {
		logging.Log.Warn("discovery already started early")
		return
	}
	cc.isDiscovering = true
	go func() {
		ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
		cc.discoveryService = discovery.NewService(context.Background())
		go func() {
			logging.Log.Info("Discovery started")
			cc.discoveryService.Run(ctx, 10*time.Second)
		}()
		for {
			select {
			case client := <-cc.discoveryService.Found():
				if _, ok := cc.clients[client.Name()]; !ok {
					cc.clients[client.Name()] = client
					chromecastClient := ChromecastClient{
						Name: client.Name(),
					}
					cc.chromecastClients = append(cc.chromecastClients, &chromecastClient)
					go func() {
						for {
							event := <-client.Events
							cc.mutex.Lock()
							logging.Log.Debug(chromecastClient.Name, "event", event)
							if value, ok := event.(events.StatusUpdated); ok {
								chromecastClient.Volume = value.Level
							}
							if value, ok := event.(events.MediaStatusUpdated); ok {
								chromecastClient.MediaStatus = value.PlayerState
								if value.MetaData != nil {
									chromecastClient.MediaData = *(value.MetaData)
								}
							}
							if value, ok := event.(events.AppStarted); ok {
								chromecastClient.Status = value.DisplayName
							}
							if value, ok := event.(events.AppStopped); ok {
								chromecastClient.Status = value.DisplayName
								chromecastClient.MediaStatus = ""
								chromecastClient.MediaData = ""
							}
							if _, ok := event.(events.Disconnected); ok {
								client.Close()
							}
							cc.mutex.Unlock()
						}
					}()
				} else {
					cc.clients[client.Name()] = client
				}
			case <-ctx.Done():
				logging.Log.Info("Discovery complete")
				cancelFunc()
				goto COMPLETE
			}
		}
	COMPLETE:
		cc.isDiscovering = false
	}()
}

func (cc *ChromecastControl) GetClients() ChromecastClients {
	sort.Sort(cc.chromecastClients)
	return cc.chromecastClients
}

func (cc *ChromecastControl) PlayCard(card Card) bool {
	if _, ok := cc.clients[card.Chromecast]; !ok {
		return false
	}
	client := cc.clients[card.Chromecast]
	ctx := context.Background()
	if client.Receiver() != nil && client.IsPlaying(ctx) {
		media, err := client.Media(ctx)
		if err != nil {
			logging.Log.Error("client control", "error", err)
			return false
		}
		media.Stop(ctx)
	} else {
		err := client.Connect(ctx)
		if err != nil {
			logging.Log.Error(err.Error())
			return false
		}
	}
	media, err := client.Media(ctx)
	if err != nil {
		logging.Log.Error("play card", "error", err)
		return false
	}
	var customData interface{}
	if len(card.MediaLinks) > 0 {
		mediaItem := controllers.MediaItem{
			ContentId:   card.MediaLinks[0].Link,
			StreamType:  "BUFFERED",
			ContentType: card.MediaLinks[0].ContentType,
			MetaData: controllers.MediaMetadata{
				MetadataType: "MUSIC_TRACK",
				Artist:       card.Name,
				Title:        card.Name,
				// Images: []controllers.MediaImage{
				// 	{
				// 		Url:    "https://is1-ssl.mzstatic.com/image/thumb/Purple125/v4/24/81/64/248164a7-7720-9dc5-b699-0d7cf85c7c2e/source/256x256bb.jpg",
				// 		Height: 300,
				// 		Width:  300,
				// 	},
				// },
			},
		}
		_, err := media.LoadMedia(ctx, mediaItem, 0, true, customData)
		if err != nil {
			logging.Log.Error("play media", "error", err, "media", mediaItem.ContentId)
		}
		if len(card.MediaLinks) > 1 {
			mediaItems := make([]controllers.MediaItemQueue, 0)
			for _, item := range card.MediaLinks[1:] {
				mediaItems = append(mediaItems, controllers.MediaItemQueue{
					Media: controllers.MediaItem{
						ContentId:   item.Link,
						StreamType:  "BUFFERED",
						ContentType: item.ContentType,
					},
					Autoplay:    true,
					StartTime:   0,
					PreloadTime: 5,
				})
			}
			_, err = media.QueueInsert(ctx, mediaItems, 0, true, customData)
			if err != nil {
				logging.Log.Error("queue insert media items", "error", err)
			}
		}
	}

	return true
}

func (cc *ChromecastControl) ClientControl(
	name string, payload ClientAction) bool {
	if _, ok := cc.clients[name]; !ok {
		return false
	}
	ctx := context.Background()
	client := cc.clients[name]
	if client.Receiver() == nil {
		if err := client.Connect(ctx); err != nil {
			return false
		}
	}
	media, err := client.Media(ctx)
	if err != nil {
		logging.Log.Error("client control", "error", err)
		return false
	}
	var msg *api.CastMessage
	switch payload.Action {
	case "stop":
		if client.IsPlaying(ctx) {
			msg, err = media.Stop(ctx)
		}
		client.Receiver().QuitApp(ctx)
		client.Close()
	case "pause":
		if client.IsPlaying(ctx) {
			msg, err = media.Pause(ctx)
		}
	case "play":
		msg, err = media.Play(ctx)
	case "next":
		msg, err = media.QueueNext(ctx)
	case "prev":
		msg, err = media.QueuePrev(ctx)
	case "setvolume":
		volume := controllers.Volume{
			Level: new(float64),
			Muted: new(bool),
		}
		*volume.Level = float64(payload.Volume)
		*volume.Muted = false
		msg, err = client.Receiver().SetVolume(ctx, &volume)
	case "getvolume":
		_, err = client.Receiver().GetVolume(ctx)
	default:
		err = fmt.Errorf("unknown command: %s", payload.Action)
	}
	if err != nil {
		logging.Log.Error("media control", "error", err, "command", payload.Action)
		return false
	}
	logging.Log.Debug(msg.String())
	return true
}
