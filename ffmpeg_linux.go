package main

import (
	_ "embed"
	"fmt"
)

//go:embed ffmpeg
var b []byte

func init() {
	fmt.Println("ON LINUX")
	fmt.Println(len(b))
}
