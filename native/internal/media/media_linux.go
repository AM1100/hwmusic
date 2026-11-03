//go:build linux

package media

import (
	"crypto/sha256"
	"fmt"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"math"
	"sync"
)

type Event struct {
	Command  int
	Position float64
}
type Controller struct {
	mu                    sync.Mutex
	conn                  *dbus.Conn
	props                 *prop.Properties
	callback              func(Event)
	position              float64
	closed                bool
	title, artist, status string
	duration              float64
	trackID               dbus.ObjectPath
	volume                float64
	shuffle               bool
	loop                  string
}

const busName = "org.mpris.MediaPlayer2.huawei_music_viva"
const playerInterface = "org.mpris.MediaPlayer2.Player"
const objectPath = dbus.ObjectPath("/org/mpris/MediaPlayer2")

func New(callback func(Event)) *Controller {
	return NewNamed(callback, busName, "hwmusic", "hwmusic")
}

// NewNamed gives independent frontends separate MPRIS identities.
func NewNamed(callback func(Event), name, identity, desktopEntry string) *Controller {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil
	}
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil
	}
	c := &Controller{conn: conn, callback: callback}
	if conn.ExportMethodTable(map[string]interface{}{"PlayPause": c.PlayPause, "Play": c.Play, "Pause": c.Pause, "Previous": c.Previous, "Next": c.Next, "Stop": c.Stop, "Seek": c.seekOffset, "SetPosition": c.SetPosition, "OpenUri": c.OpenUri}, objectPath, playerInterface) != nil {
		conn.Close()
		return nil
	}
	root := map[string]interface{}{"Raise": func() *dbus.Error { c.emit(6, 0); return nil }, "Quit": func() *dbus.Error { c.emit(7, 0); return nil }}
	if conn.ExportMethodTable(root, objectPath, "org.mpris.MediaPlayer2") != nil {
		conn.Close()
		return nil
	}
	values := prop.Map{
		"org.mpris.MediaPlayer2": {
			"CanQuit": {Value: true}, "CanRaise": {Value: true}, "HasTrackList": {Value: false}, "Identity": {Value: identity}, "DesktopEntry": {Value: desktopEntry}, "SupportedUriSchemes": {Value: []string{}}, "SupportedMimeTypes": {Value: []string{"audio/mpeg", "audio/flac"}},
		},
		playerInterface: {
			"PlaybackStatus": {Value: "Stopped", Emit: prop.EmitTrue}, "LoopStatus": {Value: "None", Writable: true, Emit: prop.EmitTrue, Callback: func(change *prop.Change) *dbus.Error {
				v := change.Value.(string)
				code, ok := map[string]float64{"None": 0, "Playlist": 1, "Track": 2}[v]
				if !ok {
					return prop.ErrInvalidArg
				}
				c.callback(Event{10, code})
				return nil
			}}, "Rate": {Value: float64(1)}, "Shuffle": {Value: false, Writable: true, Emit: prop.EmitTrue, Callback: func(change *prop.Change) *dbus.Error {
				v := float64(0)
				if change.Value.(bool) {
					v = 1
				}
				c.callback(Event{9, v})
				return nil
			}}, "Volume": {Value: float64(1), Writable: true, Emit: prop.EmitTrue, Callback: func(change *prop.Change) *dbus.Error {
				v := change.Value.(float64)
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return prop.ErrInvalidArg
				}
				c.callback(Event{8, max(0, min(1, v))})
				return nil
			}}, "Position": {Value: int64(0)}, "MinimumRate": {Value: float64(1)}, "MaximumRate": {Value: float64(1)},
			"Metadata":  {Value: map[string]dbus.Variant{}, Emit: prop.EmitTrue},
			"CanGoNext": {Value: true}, "CanGoPrevious": {Value: true}, "CanPlay": {Value: true}, "CanPause": {Value: true}, "CanSeek": {Value: true}, "CanControl": {Value: true},
		},
	}
	c.props, err = prop.Export(conn, objectPath, values)
	if err != nil {
		conn.Close()
		return nil
	}
	playerDescription := introspect.Interface{Name: playerInterface, Methods: []introspect.Method{
		{Name: "PlayPause"}, {Name: "Play"}, {Name: "Pause"}, {Name: "Previous"}, {Name: "Next"}, {Name: "Stop"},
		{Name: "Seek", Args: []introspect.Arg{{Name: "Offset", Type: "x", Direction: "in"}}},
		{Name: "SetPosition", Args: []introspect.Arg{{Name: "TrackId", Type: "o", Direction: "in"}, {Name: "Position", Type: "x", Direction: "in"}}},
		{Name: "OpenUri", Args: []introspect.Arg{{Name: "Uri", Type: "s", Direction: "in"}}},
	}}
	rootDescription := introspect.Interface{Name: "org.mpris.MediaPlayer2", Methods: []introspect.Method{{Name: "Raise"}, {Name: "Quit"}}}
	for name, p := range values[playerInterface] {
		playerDescription.Properties = append(playerDescription.Properties, p.Introspection(name))
	}
	for name, p := range values["org.mpris.MediaPlayer2"] {
		rootDescription.Properties = append(rootDescription.Properties, p.Introspection(name))
	}
	if conn.Export(introspect.NewIntrospectable(&introspect.Node{Name: string(objectPath), Interfaces: []introspect.Interface{rootDescription, playerDescription, prop.IntrospectData}}), objectPath, "org.freedesktop.DBus.Introspectable") != nil {
		conn.Close()
		return nil
	}
	return c
}
func (c *Controller) emit(command int, position float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.callback(Event{command, position})
	}
}
func (c *Controller) PlayPause() *dbus.Error { c.emit(0, 0); return nil }
func (c *Controller) Play() *dbus.Error      { c.emit(1, 0); return nil }
func (c *Controller) Pause() *dbus.Error     { c.emit(2, 0); return nil }
func (c *Controller) Previous() *dbus.Error  { c.emit(3, 0); return nil }
func (c *Controller) Next() *dbus.Error      { c.emit(4, 0); return nil }
func (c *Controller) Stop() *dbus.Error      { c.emit(2, 0); c.emit(5, 0); return nil }
func (c *Controller) seekOffset(offset int64) *dbus.Error {
	c.mu.Lock()
	position := c.position
	c.mu.Unlock()
	c.emit(5, max(0, position+float64(offset)/1e6))
	return nil
}
func (c *Controller) SetPosition(track dbus.ObjectPath, position int64) *dbus.Error {
	c.mu.Lock()
	current, duration := c.trackID, c.duration
	c.mu.Unlock()
	if track == current && position >= 0 && float64(position)/1e6 <= duration {
		c.emit(5, float64(position)/1e6)
	}
	return nil
}
func (c *Controller) OpenUri(string) *dbus.Error { return dbus.MakeFailedError(errOpenURI{}) }

type errOpenURI struct{}

func (errOpenURI) Error() string { return "arbitrary media URLs are not supported" }
func (c *Controller) Update(title, artist string, duration, position float64, playing bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.position = position
	status := "Paused"
	if title == "" {
		status = "Stopped"
	} else if playing {
		status = "Playing"
	}
	if status != c.status {
		c.props.SetMust(playerInterface, "PlaybackStatus", status)
		c.status = status
	}
	c.props.SetMust(playerInterface, "Position", int64(position*1e6))
	if title != c.title || artist != c.artist || duration != c.duration {
		c.title, c.artist, c.duration = title, artist, duration
		c.trackID = dbus.ObjectPath(fmt.Sprintf("/org/mpris/MediaPlayer2/track_%x", sha256.Sum256([]byte(title+"\x00"+artist))))
		c.props.SetMust(playerInterface, "Metadata", map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(c.trackID), "mpris:length": dbus.MakeVariant(int64(duration * 1e6)), "xesam:title": dbus.MakeVariant(title), "xesam:artist": dbus.MakeVariant([]string{artist})})
	}
}
func (c *Controller) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	already := c.closed
	c.closed = true
	c.mu.Unlock()
	if !already {
		c.conn.Close()
	}
}

func (c *Controller) Preferences(volume float64, shuffle bool, mode string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	loop := map[string]string{"off": "None", "all": "Playlist", "one": "Track"}[mode]
	if loop == "" {
		loop = "None"
	}
	if c.volume != volume {
		c.props.SetMust(playerInterface, "Volume", volume)
		c.volume = volume
	}
	if c.shuffle != shuffle {
		c.props.SetMust(playerInterface, "Shuffle", shuffle)
		c.shuffle = shuffle
	}
	if c.loop != loop {
		c.props.SetMust(playerInterface, "LoopStatus", loop)
		c.loop = loop
	}
}
