package webrtc

import (
	"fmt"
	"time"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/media"
	vpxEncoder "github.com/rwrz/go-yuv2webRTC/vpx-encoder"
)

var config = webrtc.RTCConfiguration{IceServers: []webrtc.RTCIceServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}}

func init() {
	webrtc.RegisterDefaultCodecs()
}

// NewWebRTC create
func NewWebRTC() *WebRTC {
	w := &WebRTC{
		ImageChannel: make(chan []byte, 2),
	}
	return w
}

// WebRTC connection
type WebRTC struct {
	connection  *webrtc.RTCPeerConnection
	encoder     *vpxEncoder.VpxEncoder
	isConnected bool
	// for yuvI420 image
	ImageChannel chan []byte
}

// StartClient start webrtc
func (w *WebRTC) StartClient(remoteSession string, width, height int) (string, error) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			w.StopClient()
		}
	}()

	// reset client
	if w.isConnected {
		w.StopClient()
		time.Sleep(2 * time.Second)
	}

	encoder, err := vpxEncoder.NewVpxEncoder(width, height, 20, 1200, 5, []int32{1, 1, 1}, []int32{int32(width * height), int32(width * height), int32(width * height)})
	if err != nil {
		return "", err
	}
	w.encoder = encoder

	fmt.Println("=== StartClient ===")

	w.connection, err = webrtc.New(config)
	if err != nil {
		return "", err
	}

	vp8Track, err := w.connection.NewRTCSampleTrack(webrtc.DefaultPayloadTypeVP8, "video", "pion2")
	if err != nil {
		return "", err
	}
	_, err = w.connection.AddTrack(vp8Track)
	if err != nil {
		return "", err
	}

	w.connection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
		if connectionState == ice.ConnectionStateConnected {
			go func() {
				w.isConnected = true
				fmt.Println("ConnectionStateConnected")
				w.startStreaming(vp8Track)
			}()

		}
		if connectionState == ice.ConnectionStateFailed || connectionState == ice.ConnectionStateClosed || connectionState == ice.ConnectionStateDisconnected {
			w.StopClient()
		}
	})

	offer := util.Decode(remoteSession)
	if err != nil {
		return "", err
	}
	err = w.connection.SetRemoteDescription(offer)
	if err != nil {
		return "", err
	}
	answer, err := w.connection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	localSession := util.Encode(answer)
	return localSession, nil
}

// StopClient disconnect
func (w *WebRTC) StopClient() {
	fmt.Println("===StopClient===")
	w.isConnected = false
	if w.encoder != nil {
		w.encoder.Release()
	}
	if w.connection != nil {
		w.connection.Close()
	}
	w.connection = nil
}

// IsConnected comment
func (w *WebRTC) IsConnected() bool {
	return w.isConnected
}

func (w *WebRTC) startStreaming(vp8Track *webrtc.RTCTrack) {
	fmt.Println("Start streaming")
	// send screenshot
	go func() {
		for w.isConnected {
			yuv := <-w.ImageChannel
			if len(w.encoder.Input) < cap(w.encoder.Input) {
				w.encoder.Input <- yuv
			}
		}
	}()

	// receive frame buffer
	go func() {
		for i := 0; w.isConnected; i++ {
			bs := <-w.encoder.Output
			if i%10 == 0 {
				fmt.Println("On Frame", len(bs), i)
			}
			if len(vp8Track.Samples) < cap(vp8Track.Samples) {
				vp8Track.Samples <- media.RTCSample{Data: bs, Samples: 1}
			}
		}
	}()
}
