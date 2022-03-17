package ffmpeg

import (
	_ "embed"
	"fmt"
	log "github.com/schollz/logger"
	"io/ioutil"
	"os"
)

//go:embed ffmpegmac
var b []byte

var loadedDarwin bool

func init() {
	fmt.Println("loaded ffmpeg ", len(b))
}

func Binary() string {
	log.Debugf("loaded ffmpeg: %d", len(b))
	if !loadedDarwin {
		loadedDarwin = true
		ioutil.WriteFile("./ffmpeg", b, 0777)
	}
	return "./ffmpeg"
}

func Clean() {
	os.Remove("./ffmpeg")
}
