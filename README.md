# stream your audio

This program/server lets you livestream audio from your computer to a website *as easily as possible*. How easy? You should be able to just [download a release](https://github.com/schollz/streammyaudio/releases/latest), double-click it, and stream!

Use it to play demos for people you know, or make a live podcast, or stream your piano practice, or whatever you'd like.

live website: https://streammyaudio.com

blog (more info): https://schollz.com/blog/stream

## Usage

The easiest way to use this is to [download the latest release](https://github.com/schollz/streammyaudio/releases/latest). But you can build it yourself. This codebase includes both the server and the client. 

## Linux 

The basic build (Linux only) is:

```
git clone https://github.com/schollz/streammyaudio
cd streammyaudio
go build -v
```

which will build both the server and client, though you will also need `ffmpeg` installed.

You actually don't need to build this if just want to stream audio on Linux. You can directly just use `ffmpeg` and `curl` to send live audio:

```
ffmpeg -f alsa -i hw:0 -f mp3 - | \
    curl -s -k -H "Transfer-Encoding: chunked" -X POST -T - \
    "https://streammyaudio.com/YOURSTATIONNAME.mp3?stream=true&advertise=true"
```

Or similar. See the [website](https://streammyaudio.com) for more ideas.

## Windows

Windows basically is the same but it will automatically bundle a statically-compiled `ffmpeg` to self-contain the client. You can simply run

```
make build-windows
```

to build the client with `ffmpeg` so that it is a portale app.

## Mac OS

Mac OS basically is the same but it will automatically bundle a statically-compiled `ffmpeg` to self-contain the client. You can simply run

```
make build-mac
```

for most macs, or

```
make build-mac-arm`
```

for M1 macs. These will automatically bundle with the right version of `ffmpeg`.


## License

GPL
