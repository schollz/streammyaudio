package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
)

func main() {
	var err error
	cmd := exec.Command("ffmpeg", "-f", "alsa", "-i", "hw:0", "-f", "mp3", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	tr := http.DefaultTransport
	client := &http.Client{
		Transport: tr,
		Timeout:   0,
	}
	r := stdout
	req := &http.Request{
		Method: "POST",
		URL: &url.URL{
			Scheme:   "http",
			Host:     "localhost:9222",
			Path:     "/test.mp3",
			RawQuery: "stream=true&advertise=true",
		},
		ProtoMajor:    1,
		ProtoMinor:    1,
		ContentLength: -1,
		Body:          r,
	}
	fmt.Printf("Doing request\n")
	go func() {
		_, err = client.Do(req)
		if err != nil {
			log.Print(err)
		}
	}()

	fmt.Println("starting command")
	err = cmd.Start()
	if err != nil {
		log.Print(err)
	}
	cmd.Wait()
}
