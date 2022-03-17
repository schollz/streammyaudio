package main

import (
	_ "embed"
	"io/ioutil"
	"time"
)

//go:embed ffmpeg.exe
var b []byte

var ffmpegBinary string

func init() {
	ioutil.WriteFile(ffmpegBinary, b, 0777)
	ffmpegBinary = "./ffmpeg.exe"
}

func fileCreated(fname string) time.Time {
	return time.Now()
}
