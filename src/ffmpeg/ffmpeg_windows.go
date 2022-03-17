package ffmpeg

import (
	_ "embed"
	"io/ioutil"
	"os"
)

//go:embed ffmpeg.exe
var b []byte

var loaded bool

func Binary() string {
	if !loaded {
		loaded = true
		ioutil.WriteFile("./ffmpeg.exe", b, 0777)
	}
	return "./ffmpeg.exe"
}

func Clean() {
	os.Remove("./ffmpeg.exe")
}
