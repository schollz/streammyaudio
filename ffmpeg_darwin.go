package main

import (
	_ "embed"
	"io/ioutil"
	"time"
)

//go:embed ffmpeg
var b []byte

var ffmpegBinary string

func init() {
	ioutil.WriteFile(ffmpegBinary, b, 0777)
	ffmpegBinary = "./ffmpeg"
}

func fileCreated(fname string) time.Time {
	return time.Now()
}
