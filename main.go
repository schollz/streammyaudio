package main

import (
	"flag"
	"os"
	"runtime"

	log "github.com/schollz/logger"
	"github.com/schollz/streamyouraudio/src/client"
	"github.com/schollz/streamyouraudio/src/server"
)

var streamName, streamAdvertise, streamArchive, streamServer string
var flagDebug bool
var flagPort int
var flagFolder string
var flagServer bool
var flagQuality int

// init initializes the clearScreen variable for MacOS, Linux, & Windows
func init() {
	flag.StringVar(&flagFolder, "server-folder", "archived", "server folder to save archived")
	flag.IntVar(&flagPort, "server-port", 9222, "port for server")
	flag.BoolVar(&flagDebug, "debug", false, "debug mode")
	flag.BoolVar(&flagServer, "server", false, "server mode")
	flag.StringVar(&streamName, "cast-name", "", "cast stream name")
	flag.StringVar(&streamAdvertise, "cast-advertise", "", "cast stream advertise (yes/no)")
	flag.StringVar(&streamArchive, "cast-archive", "", "cast stream archive (yes/no)")
	flag.StringVar(&streamServer, "cast-server", "https://streamyouraudio.com", "cast server address")
	flag.IntVar(&flagQuality, "cast-quality", -1, "cast audio quality (0 = best to 9 = worst)")
}

func main() {
	flag.Parse()

	// use all of the processors
	runtime.GOMAXPROCS(runtime.NumCPU())
	if flagDebug {
		log.SetLevel("debug")
		log.Debug("debug mode")
	} else {
		log.SetLevel("info")
	}

	var err error
	if flagServer {
		os.MkdirAll(flagFolder, os.ModePerm)
		s := &server.Server{
			Port:   flagPort,
			Folder: flagFolder,
		}
		err = s.Run()
	} else {
		c := &client.Client{
			Name:      streamName,
			Archive:   streamArchive,
			Advertise: streamAdvertise,
			Server:    streamServer,
			Quality: flagQuality,
		}
		err = c.Run()
	}
	if err != nil {
		log.Debugf("err: %+v", err)
	}
}
