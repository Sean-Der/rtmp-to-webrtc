package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/Glimesh/go-fdkaac/fdkaac"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pkg/errors"
	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"

	// opus "gopkg.in/hraban/opus.v2"
	opus "github.com/sean-der/rtmp-to-webrtc/opus"
)

func startRTMPServer(peerConnection *webrtc.PeerConnection, videoTrack, audioTrack *webrtc.TrackLocalStaticSample) {
	log.Println("Starting RTMP Server")

	tcpAddr, err := net.ResolveTCPAddr("tcp", ":1935")
	if err != nil {
		log.Panicf("Failed: %+v", err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Panicf("Failed: %+v", err)
	}

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
}

type Handler struct {
	rtmp.DefaultHandler
	peerConnection         *webrtc.PeerConnection
	videoTrack, audioTrack *webrtc.TrackLocalStaticSample
	audioDecoder           *fdkaac.AacDecoder
	audioEncoder           *opus.Encoder
	audioBuffer            []byte
	audioClockRate         uint32
}

func (h *Handler) OnServe(conn *rtmp.Conn) {
}

func (h *Handler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Printf("OnConnect: %#v", cmd)
	h.audioClockRate = 48000
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

func (h *Handler) SetOpusCtl() {
	h.audioEncoder.SetMaxBandwidth(opus.Bandwidth(2))
	h.audioEncoder.SetComplexity(9)
	h.audioEncoder.SetBitrateToAuto()
	h.audioEncoder.SetInBandFEC(true)
}
func (h *Handler) initAudio(clockRate uint32) error {

	encoder, err := opus.NewEncoder(48000, 2, opus.AppAudio)
	if err != nil {
		println(err.Error())
		return err
	}
	h.audioEncoder = encoder
	h.SetOpusCtl()
	h.audioDecoder = fdkaac.NewAacDecoder()

	return nil
}
func (h *Handler) OnAudio(timestamp uint32, payload io.Reader) error {
	// Convert AAC to opus
	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
		return err
	}

	data := new(bytes.Buffer)
	if _, err := io.Copy(data, audio.Data); err != nil {
		return err
	}
	if data.Len() <= 0 {
		log.Println("no audio datas", timestamp, payload)
		return fmt.Errorf("no audio datas")
	}
	// log.Println("\r\ntimestamp->", timestamp, "\r\npayload->", payload, "\r\naudio data->", data.Bytes())
	datas := data.Bytes()
	// log.Println("\r\naudio data len:", len(datas), "->") // hex.EncodeToString(datas))

	if audio.AACPacketType == flvtag.AACPacketTypeSequenceHeader {
		log.Println("Created new codec ", hex.EncodeToString(datas))
		err := h.initAudio(h.audioClockRate)
		if err != nil {
			log.Println(err, "error initializing Audio")
			return fmt.Errorf("can't initialize codec with %s", err.Error())
		}
		err = h.audioDecoder.InitRaw(datas)

		if err != nil {
			log.Println(err, "error initializing stream")
			return fmt.Errorf("can't initialize codec with %s", hex.EncodeToString(datas))
		}

		return nil
	}

	pcm, err := h.audioDecoder.Decode(datas)
	if err != nil {
		log.Println("decode error: ", hex.EncodeToString(datas), err)
		return fmt.Errorf("decode error")
	}
	// log.Println("\r\npcm len ", len(pcm), " ->") //, pcm)
	blockSize := 960
	for h.audioBuffer = append(h.audioBuffer, pcm...); len(h.audioBuffer) >= blockSize*4; h.audioBuffer = h.audioBuffer[blockSize*4:] {
		pcm16 := make([]int16, blockSize*2)
		pcm16len := len(pcm16)
		for i := 0; i < pcm16len; i++ {
			pcm16[i] = int16(binary.LittleEndian.Uint16(h.audioBuffer[i*2:]))
		}
		bufferSize := 1024
		opusData := make([]byte, bufferSize)
		n, err := h.audioEncoder.Encode(pcm16, opusData)
		// n, err := h.audioEncoder.ReadEncode(pcm16, opusData)
		if err != nil {
			return err
		}
		opusOutput := opusData[:n]
		// log.Println(" pcm len:", pcm16len, " data->", " opusData len", n, " data->")
		if audioErr := h.audioTrack.WriteSample(media.Sample{
			Data:     opusOutput,
			Duration: 20 * time.Millisecond,
		}); audioErr != nil {
			log.Println("WriteSample err", audioErr)
		}

	}

	return nil
}

const headerLengthField = 4

func (h *Handler) OnVideo(timestamp uint32, payload io.Reader) error {
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
		Data:     outBuf,
		Duration: time.Second / 30,
	})
}

func (h *Handler) OnClose() {
	log.Printf("OnClose")
}
