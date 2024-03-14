package control

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vkl/go-cast"
	"github.com/vkl/go-cast/api"
	"github.com/vkl/go-cast/controllers"
	"github.com/vkl/go-cast/discovery"
	_ "github.com/vkl/rfidplayer/pkg/logging"
)

type Action byte

const (
	PLAY Action = iota
	PAUSE
	STOP
	NEXT
	PREV
	SETVOLUME
	GETVOLUME
)

func (action Action) String() string {
	switch action {
	case PLAY:
		return "play"
	case PAUSE:
		return "pause"
	case STOP:
		return "stop"
	case NEXT:
		return "next"
	case PREV:
		return "prev"
	case SETVOLUME:
		return "setvolume"
	case GETVOLUME:
		return "getvolume"
	default:
		return "unknown"
	}
}

const DISCOVERY_DURATION = 30

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

type ChromecastControl struct {
	discoveryService  *discovery.Service
	castControl       *CastController
	isDiscovering     bool
	currentChromecast *cast.Client
}

func NewChromeCastControl(castControl *CastController) *ChromecastControl {
	chromecastControl := ChromecastControl{
		castControl:   castControl,
		isDiscovering: false,
	}
	chromecastControl.StartDiscovery(DISCOVERY_DURATION * time.Second)
	return &chromecastControl
}

func (cc *ChromecastControl) StartDiscovery(timeout time.Duration) {
	if cc.isDiscovering {
		slog.Warn("discovery already started early")
		return
	}
	cc.isDiscovering = true
	go func() {
		ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
		cc.discoveryService = discovery.NewService(context.Background())
		go func() {
			slog.Info("Discovery started")
			cc.discoveryService.Run(ctx, 4*time.Second)
		}()
		for {
			select {
			case client := <-cc.discoveryService.Found():
				err := cc.castControl.UpdateCast(Cast{
					Name:   client.Name(),
					IPAddr: client.IP(),
					Port:   client.Port(),
					Info:   client.GetInfo(),
				})
				if err != nil {
					slog.Error(err.Error())
				}
			case <-ctx.Done():
				slog.Info("Discovery complete")
				cancelFunc()
				goto COMPLETE
			}
		}
	COMPLETE:
		cc.isDiscovering = false
	}()
}

func (cc *ChromecastControl) GetClients() Casts {
	return cc.castControl.GetCasts()
}

func (cc *ChromecastControl) CastStatus() cast.DisplayStatus {
	if cc.currentChromecast == nil {
		slog.Debug("chromecast not used")
		return cast.DisplayStatus{}
	}
	return cc.currentChromecast.DisplayStatus()
}

func (cc *ChromecastControl) PlayCard(card Card) bool {
	var castInfo Cast
	var ok bool
	if castInfo, ok = cc.castControl.GetCastByName(card.Chromecast); !ok {
		cc.StartDiscovery(DISCOVERY_DURATION * time.Second)
		return false
	}
	if cc.currentChromecast != nil {
		cc.currentChromecast.Close()
	}
	cc.currentChromecast = cast.NewClient(castInfo.IPAddr, castInfo.Port)
	cc.currentChromecast.SetName(castInfo.Info["fn"])
	cc.currentChromecast.SetInfo(castInfo.Info)
	client := cc.currentChromecast
	ctx := context.Background()
	if !client.IsConnected() {
		if err := client.Connect(ctx); err != nil {
			cc.currentChromecast = nil
			slog.Error(err.Error())
			cc.StartDiscovery(DISCOVERY_DURATION * time.Second)
			return false
		}
	}
	media, err := client.Media(ctx, cast.AppMedia)
	if err != nil {
		slog.Error("play media", "error", err)
		return false
	}

	if len(card.MediaLinks) > 0 {
		mediaItem := controllers.MediaItem{
			ContentId:   card.MediaLinks[0].Link,
			StreamType:  "BUFFERED",
			ContentType: card.MediaLinks[0].ContentType,
			MetaData: controllers.MediaMetadata{
				MetadataType: controllers.MUSIC_TRACK,
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
		_, err := media.LoadMedia(ctx, mediaItem, 0, true, nil)
		if err != nil {
			slog.Error("play media", "error", err, "media", mediaItem.ContentId)
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
			_, err = media.QueueInsert(ctx, mediaItems, 0, true, nil)
			if err != nil {
				slog.Error("queue insert media items", "error", err)
			}
		}
	}
	return true
}

func (cc *ChromecastControl) Control(action Action) bool {
	payload := ClientAction{
		Action: action.String(),
	}
	return cc.ClientControl(payload)
}

func (cc *ChromecastControl) SetVolume(volume float64) bool {
	payload := ClientAction{
		Action: SETVOLUME.String(),
		Volume: volume,
	}
	return cc.ClientControl(payload)
}

func (cc *ChromecastControl) GetVolume() (float64, bool) {
	payload := ClientAction{
		Action: GETVOLUME.String(),
	}
	volume := &controllers.Volume{
		Level: new(float64),
		Muted: new(bool),
	}
	cc.ClientControl(payload, volume)
	slog.Debug("Volume", "level", *volume.Level)
	return *volume.Level, true
}

func (cc *ChromecastControl) ClientControl(payload ClientAction, args ...interface{}) bool {
	slog.Debug("client control", "current", cc.currentChromecast)
	if cc.currentChromecast == nil {
		slog.Debug("chromecast not used")
		return false
	}
	ctx := context.Background()
	client := cc.currentChromecast
	if !client.IsConnected() {
		err := client.Connect(ctx)
		if err != nil {
			slog.Error(err.Error())
			cc.StartDiscovery(DISCOVERY_DURATION * time.Second)
			return false
		}
	}
	receiver := client.Receiver()
	var media *controllers.MediaController
	var err error
	media, err = client.Media(ctx, cast.AppMedia)
	if err != nil {
		slog.Error("client control", "error", err)
		return false
	}
	var msg *api.CastMessage
	switch payload.Action {
	case "stop":
		if receiver.IsPlaying(ctx) {
			msg, err = media.Stop(ctx)
		}
	case "pause":
		if receiver.IsPlaying(ctx) {
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
		var volume *controllers.Volume
		volume, err = client.Receiver().GetVolume(ctx)
		slog.Debug("Volume", "level", *volume.Level)
		if len(args) > 0 {
			if value, ok := args[0].(*controllers.Volume); ok {
				*value.Level = *volume.Level
				*value.Muted = *volume.Muted
			}
		}
	default:
		err = fmt.Errorf("unknown command: %s", payload.Action)
	}
	if err != nil {
		slog.Error("media control", "error", err, "command", payload.Action)
		return false
	}
	slog.Debug(msg.String())
	return true
}
