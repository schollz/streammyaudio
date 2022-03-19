serve:
	go build -v
	./streammyaudio --debug --server 

build-all: build-linux build-windows build-mac build-mac-arm

build-linux:
	go build -v -o streammyaudio
	zip streammyaudio_linux_amd64.zip streammyaudio LICENSE

build-windows: win32-x64
	cp win32-x64 src/ffmpeg/ffmpeg.exe
	GOOS=windows GOARCH=amd64 go build -v -o streammyaudio.exe
	zip streammyaudio_windows_amd64.zip streammyaudio.exe LICENSE
	rm -f src/ffmpeg/ffmpeg.exe

build-mac: darwin-x64
	cp darwin-x64 src/ffmpeg/ffmpegmac
	GOOS=darwin GOARCH=amd64 go build -v -o streammyaudio
	zip streammyaudio_macos_amd64.zip streammyaudio LICENSE
	rm -f src/ffmpeg/ffmpegmac

build-mac-arm: darwin-arm64
	cp darwin-arm64 src/ffmpeg/ffmpegmac
	GOOS=darwin GOARCH=arm64 go build -v -o streammyaudio
	zip streammyaudio_macos_m1.zip streammyaudio LICENSE
	rm -f src/ffmpeg/ffmpegmac

darwin-arm64:
	wget https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0/darwin-arm64

darwin-x64:
	wget https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0/darwin-x64

win32-x64:
	wget https://github.com/eugeneware/ffmpeg-static/releases/download/b5.0/win32-x64