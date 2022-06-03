# rtmp-to-webrtc

This repo demonstrates a RTMP server that on every RTMP publish makes the audio/video available via WebRTC playback.

## How to use

* `go run *.go`
* Open [http://localhost:8080/](http://localhost:8080/)
* Publish an RTMP feed to `rtmp://localhost:1935/publish/foobar`. It must be H264 and alaw

#### GStreamer

`gst-launch-1.0 videotestsrc ! video/x-raw,format=I420 ! x264enc speed-preset=ultrafast tune=zerolatency key-int-max=20 ! flvmux name=flvmux ! rtmpsink location=rtmp://localhost:1935/publish/foobar audiotestsrc ! alawenc ! flvmux.`

## AAC convert to OPUS

Modify from source https://github.com/Glimesh/rtmp-ingest.git thanks Glimesh


### macOS Development

```
brew install opusfile fdk-aac
```

### Ubuntu / Linux Development

```
apt install -y pkg-config build-essential libopusfile-dev libfdk-aac-dev libavutil-dev libavcodec-dev libswscale-dev
```
