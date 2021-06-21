package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"
	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

var peerConnection *webrtc.PeerConnection = nil;
var err error = nil;
var answer webrtc.SessionDescription;
var videoTrack *webrtc.Track = nil;
var audioTrack *webrtc.Track = nil;
var response []byte =nil;
var listener *net.TCPListener = nil;

func main() {


	go func ()  {

		tcpAddr, err := net.ResolveTCPAddr("tcp", ":1935")
		if err != nil {
			log.Panicf("Failed: %+v", err)
		}

		listener, err := net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			log.Panicf("Failed: %+v", err)
		}

		log.Println("Creating RTMP")
		srv := rtmp.NewServer(&rtmp.ServerConfig{
			OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
	
				return conn, &rtmp.ConnConfig{
					Handler: &Handler{
						peerConnection: peerConnection,
						videoTrack:     videoTrack,
						audioTrack:     audioTrack,
					},
	
					ControlState: rtmp.StreamControlStateConfig{
						DefaultBandwidthWindowSize: 6 * 1024 * 1024 / 8,
					},
				}
			},
		})
	
		if err := srv.Serve(listener); err != nil {
			log.Panicf("Failed: %+v", err)
		}
	
	}()



	rand.Seed(time.Now().UTC().UnixNano())

	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/createPeerConnection", createPeerConnection)

	panic(http.ListenAndServe(":8080", nil))




}


func createPeerConnection(w http.ResponseWriter, r *http.Request) {
	log.Println("Incoming HTTP Request")

	/* If the peer already exists lets close down*/
	if peerConnection != nil {
		peerConnection.Close();
	}

	peerConnection, err = webrtc.NewPeerConnection(webrtc.Configuration{});
	if err != nil {
		panic(err)
	}

	videoTrack, err = peerConnection.NewTrack(webrtc.DefaultPayloadTypeH264, rand.Uint32(), "video", "pion")
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}

	audioTrack, err = peerConnection.NewTrack(webrtc.DefaultPayloadTypePCMA, rand.Uint32(), "audio", "pion")
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(audioTrack); err != nil {
		panic(err)
	}

	var offer webrtc.SessionDescription
	if err = json.NewDecoder(r.Body).Decode(&offer); err != nil {
		panic(err)
	}

	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	answer, err = peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	response, err = json.Marshal(answer)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(response); err != nil {
		panic(err)
	}
}

type Handler struct {
	rtmp.DefaultHandler
	peerConnection         *webrtc.PeerConnection
	videoTrack, audioTrack *webrtc.Track
}

func (h *Handler) OnServe(conn *rtmp.Conn) {
}

func (h *Handler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Printf("OnConnect: %#v", cmd)
	return nil
}

func (h *Handler) OnCreateStream(timestamp uint32, cmd *rtmpmsg.NetConnectionCreateStream) error {
	log.Printf("OnCreateStream: %#v", cmd)
	return nil
}

func (h *Handler) OnPublish(timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
	log.Printf("OnPublish: %#v", cmd)

	if cmd.PublishingName == "" {
		return errors.New("PublishingName is empty")
	}
	return nil
}

func (h *Handler) OnAudio(timestamp uint32, payload io.Reader) error {

	if peerConnection == nil && audioTrack == nil {
		log.Println("No RTC Client");
		return err;
	}
	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
		return err
	}

	data := new(bytes.Buffer)
	if _, err := io.Copy(data, audio.Data); err != nil {
		return err
	}

	return h.audioTrack.WriteSample(media.Sample{
		Data:    data.Bytes(),
		Samples: media.NSamples(20*time.Millisecond, 48000),
	})
}

const headerLengthField = 4

func (h *Handler) OnVideo(timestamp uint32, payload io.Reader) error {


	if peerConnection == nil && videoTrack == nil {
		log.Println("No RTC Client");
		return err;
	}
	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(payload, &video); err != nil {
		return err
	}

	data := new(bytes.Buffer)
	if _, err := io.Copy(data, video.Data); err != nil {
		return err
	}

	outBuf := []byte{}
	videoBuffer := data.Bytes()
	for offset := 0; offset < len(videoBuffer); {
		bufferLength := int(binary.BigEndian.Uint32(videoBuffer[offset : offset+headerLengthField]))
		if offset+bufferLength >= len(videoBuffer) {
			break
		}

		offset += headerLengthField
		outBuf = append(outBuf, []byte{0x00, 0x00, 0x00, 0x01}...)
		outBuf = append(outBuf, videoBuffer[offset:offset+bufferLength]...)

		offset += int(bufferLength)
	}

	return h.videoTrack.WriteSample(media.Sample{
		Data:    outBuf,
		Samples: media.NSamples(time.Second/30, 90000),
	})
}

func (h *Handler) OnClose() {
	log.Printf("OnClose")
}
