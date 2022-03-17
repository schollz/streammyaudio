package main

import (
	_ "embed"
	"os"
	"syscall"
	"time"
)

//go:embed ffmpeg
var b []byte

var ffmpegBinary string

func init() {
	ffmpegBinary = "ffmpeg"
}

func fileCreated(fname string) time.Time {
	finfo, _ := os.Stat(fname)
	stat_t := finfo.Sys().(*syscall.Stat_t)
	return timespecToTime(stat_t.Ctim)
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(int64(ts.Sec), int64(ts.Nsec))
}
