# rtmp-to-webrtc

This repo demonstrates a RTMP server that on every RTMP publish makes the audio/video available via WebRTC playback.

## How to use

* `go run *.go`
* Open [http://localhost:8080/](http://localhost:8080/)
* Publish an RTMP feed to `rtmp://localhost:1935/publish/foobar`. It must be H264 and alaw

#### AAC convert to OPUS

Modify from source https://github.com/Glimesh/rtmp-ingest.git thanks Glimesh 

Modify from source [mediadevices/pkg/codec/opus at master · pion/mediadevices (github.com)](https://github.com/pion/mediadevices/tree/master/pkg/codec/opus)

Opus lib ref  [xiph/opus: Modern audio compression for the internet. (github.com)](https://github.com/xiph/opus)

Opus Lib : static build(please add your lib to path ./opus name like :libopus-linux-x64.a), pkgconfig dynamic

    please build your lib or install your opus lib dev env

### macOS Development

```
brew install opusfile fdk-aac
```

### Ubuntu / Linux Development

```
apt install -y pkg-config build-essential libopusfile-dev libfdk-aac-dev libavutil-dev libavcodec-dev libswscale-dev
```
