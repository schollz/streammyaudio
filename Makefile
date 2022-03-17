serve:
	go build -v
	./streamyouraudio --debug --server 

build-all: build-linux build-windows build-mac

build-linux:
	go build -v -o streamyouraudio
	zip streamyouraudio_linux_amd64.zip streamyouraudio

build-windows: src/ffmpeg/ffmpeg.exe
	GOOS=windows GOARCH=amd64 go build -v -o streamyouraudio.exe
	zip streamyouraudio_windows_amd64.zip streamyouraudio.exe

build-mac: src/ffmpeg/ffmpeg
	GOOS=darwin GOARCH=amd64 go build -v -o streamyouraudio
	zip streamyouraudio_macos_amd64.zip streamyouraudio

src/ffmpeg/ffmpeg.exe: ffmpeg-release-essentials.zip
	unzip -o ffmpeg-release-essentials.zip
	cp ffmpeg-5.0-essentials_build/bin/ffmpeg.exe src/ffmpeg/ffmpeg.exe

src/ffmpeg/ffmpeg: ffmpeg-5.0.zip
	unzip -o ffmpeg-5.0.zip
	cp ffmpeg src/ffmpeg/ffmpeg

ffmpeg-release-essentials.zip:
	wget https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip

ffmpeg-5.0.zip:
	wget https://evermeet.cx/ffmpeg/ffmpeg-5.0.zip


