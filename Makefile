serve:
	go build -v
	./streammyaudio --debug --server 
	
build-all: build-linux build-windows build-mac

build-linux:
	go build -v -o cast_linux_amd64
	zip cast_linux_amd64.zip cast_linux_amd64

build-windows: ffmpeg.exe
	GOOS=windows GOARCH=amd64 go build -v -o cast_windows_amd64
	zip cast_windows_amd64.zip cast_windows_amd64

build-mac: ffmpeg
	GOOS=darwin GOARCH=amd64 go build -v -o cast_macos_amd64
	zip cast_macos_amd64.zip cast_macos_amd64

ffmpeg.exe: ffmpeg-release-essentials.zip
	unzip ffmpeg-release-essentials.zip
	mv ffmpeg-5.0-essentials_build/bin/ffmpeg.exe

ffmpeg: ffmpeg-5.0.zip
	unzip ffmpeg-5.0.zip

ffmpeg-release-essentials.zip:
	wget https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip

ffmpeg-5.0.zip:
	wget https://evermeet.cx/ffmpeg/ffmpeg-5.0.zip


