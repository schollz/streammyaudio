package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/h2non/filetype"
	"github.com/manifoldco/promptui"
	log "github.com/schollz/logger"
)

// clearScreen is a map from the operating system name to functions to execute
// the terminal commands to clear the screen on said OS
var clearScreen map[string]func()

//go:embed template.html
var templateData string

//go:embed static/*
var staticContent embed.FS

var streamName, streamAdvertise, streamArchive, streamDevice string

var flagDebug bool
var flagPort int
var flagFolder string
var flagServer bool

// init initializes the clearScreen variable for MacOS, Linux, & Windows
func init() {
	flag.StringVar(&flagFolder, "folder", "archived", "server folder to save archived")
	flag.IntVar(&flagPort, "port", 9222, "port for server")
	flag.BoolVar(&flagDebug, "debug", false, "debug mode")
	flag.BoolVar(&flagServer, "server", false, "server mode")
	flag.StringVar(&streamName, "cast-name", "", "cast stream name")
	flag.StringVar(&streamAdvertise, "cast-advertise", "", "cast stream advertise (yes/no)")
	flag.StringVar(&streamArchive, "cast-archive", "", "cast stream archive (yes/no)")

	clearScreen = make(map[string]func())

	clearScreen["darwin"] = func() {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}

	clearScreen["linux"] = func() {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}

	clearScreen["windows"] = func() {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func main() {
	flag.Parse()
	defer func() {
		if runtime.GOOS != "windows" {
			os.Remove(ffmpegBinary)
		}
	}()

	// use all of the processors
	runtime.GOMAXPROCS(runtime.NumCPU())
	if flagDebug {
		log.SetLevel("debug")
		log.Debug("debug mode")
	} else {
		log.SetLevel("info")
	}

	log.Debugf("ffmpeg binary: %s", ffmpegBinary)
	var err error
	if flagServer {
		os.MkdirAll(flagFolder, os.ModePerm)
		err = serve()
	} else {
		err = mainCast()
	}
	if err != nil {
		log.Debugf("err: %+v", err)
	}
}

// Clears the terminal window of the user if the operating system is supported
func ClearScreen() {
	function, exists := clearScreen[runtime.GOOS]
	if exists {
		function()
	}
}

// IsSupportedOS checks to see if the operating system that the user is running
// is able to have the terminal cleared
func IsSupportedOS() bool {
	_, exists := clearScreen[runtime.GOOS]
	return exists
}

func mainCast() (err error) {
	ClearScreen()
	fmt.Println("\n\n\n" + `            ____________________________
          /|............................|
         | |:                          :|
         | |:                          :|
         | |:     ,-.   _____   ,-.    :|
         | |:    ( ` + "`" + `)) [_____] ( ` + "`" + `))   :|
         |v|:     ` + "`" + `-` + "`" + `   ' ' '   ` + "`" + `-` + "`" + `    :|
         |||:     ,______________.     :|
         |||...../::::o::::::o::::\.....|
         |^|..../:::O::::::::::O:::\....|
         |/` + "`" + `---/--------------------` + "`" + `---|
         ` + "`" + `.___/ /====/ /=//=/ /====/____/
              ` + "`" + `--------------------'` + "\n\n\n")
	err = cast()
	if err != nil {
		fmt.Println("        no stream initiated, goodbye.")
		time.Sleep(1 * time.Second)
		err = nil
	}
	return
}

type stream struct {
	b    []byte
	done bool
}

// Serve will start the server
func serve() (err error) {
	tplmain, err := template.New("webpage").Parse(templateData)
	if err != nil {
		return
	}

	channels := make(map[string]map[float64]chan stream)
	archived := make(map[string]*os.File)
	advertisements := make(map[string]bool)
	mutex := &sync.Mutex{}

	serveMain := func(w http.ResponseWriter, r *http.Request, msg string) (err error) {
		// serve the README
		adverts := []string{}
		mutex.Lock()
		for advert := range advertisements {
			adverts = append(adverts, strings.TrimPrefix(advert, "/"))
		}
		mutex.Unlock()

		active := make(map[string]struct{})
		data := struct {
			Title    string
			Items    []string
			Rand     string
			Archived []ArchivedFile
			Message  string
		}{
			Title:    "Current broadcasts",
			Items:    adverts,
			Rand:     fmt.Sprintf("%d", rand.Int31()),
			Archived: listArchived(active),
			Message:  msg,
		}

		err = tplmain.Execute(w, data)
		return
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		log.Debugf("opened %s %s", r.Method, r.URL.Path)
		defer func() {
			log.Debugf("finished %s\n", r.URL.Path)
		}()

		if r.URL.Path == "/" {
			serveMain(w, r, "")
			return
		} else if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusOK)
			return
		} else if strings.HasPrefix(r.URL.Path, "/static/") {
			filename := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/static/"))
			// This extra join implicitly does a clean and thereby prevents directory traversal
			filename = path.Join("/", filename)
			filename = path.Join("static", filename)
			log.Debugf("serving %s", filename)
			p, err := staticContent.ReadFile(filename)
			if err != nil {
				log.Error(err)
			}
			w.Write(p)
			return
		} else if strings.HasPrefix(r.URL.Path, "/"+flagFolder+"/") {
			filename := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"+flagFolder+"/"))
			// This extra join implicitly does a clean and thereby prevents directory traversal
			filename = path.Join("/", filename)
			filename = path.Join(flagFolder, filename)
			v, ok := r.URL.Query()["remove"]
			if ok && v[0] == "true" {
				os.Remove(filename)
				filename = strings.TrimPrefix(filename, "archived/")
				serveMain(w, r, fmt.Sprintf("removed '%s'.", filename))
			} else {
				v, ok := r.URL.Query()["rename"]
				if ok && v[0] == "true" {
					newname_param, ok := r.URL.Query()["newname"]
					if !ok {
						w.Write([]byte(fmt.Sprintf("ERROR")))
						return
					}
					// This join with "/" prevents directory traversal with an implicit clean
					newname := newname_param[0]
					newname = path.Join("/", newname)
					newname = path.Join(flagFolder, newname)
					os.Rename(filename, newname)
					filename = strings.TrimPrefix(filename, "archived/")
					newname = strings.TrimPrefix(newname, "archived/")
					serveMain(w, r, fmt.Sprintf("renamed '%s' to '%s'.", filename, newname))
					// w.Write([]byte(fmt.Sprintf("renamed %s to %s", filename, newname)))
				} else {
					http.ServeFile(w, r, filename)
				}
			}
			return
		}

		v, ok := r.URL.Query()["stream"]
		doStream := ok && v[0] == "true"

		v, ok = r.URL.Query()["archive"]
		doArchive := ok && v[0] == "true"

		if doArchive && r.Method == "POST" {
			if _, ok := archived[r.URL.Path]; !ok {
				folderName := path.Join(flagFolder, time.Now().Format("200601021504"))
				os.MkdirAll(folderName, os.ModePerm)
				archived[r.URL.Path], err = os.Create(path.Join(folderName, strings.TrimPrefix(r.URL.Path, "/")))
				if err != nil {
					log.Error(err)
				}
			}
			defer func() {
				mutex.Lock()
				if _, ok := archived[r.URL.Path]; ok {
					log.Debugf("closed archive for %s", r.URL.Path)
					archived[r.URL.Path].Close()
					delete(archived, r.URL.Path)
				}
				mutex.Unlock()
			}()
		}

		v, ok = r.URL.Query()["advertise"]
		log.Debugf("advertise: %+v", v)
		if ok && v[0] == "true" && doStream {
			mutex.Lock()
			advertisements[r.URL.Path] = true
			mutex.Unlock()
			defer func() {
				mutex.Lock()
				delete(advertisements, r.URL.Path)
				mutex.Unlock()
			}()
		}

		mutex.Lock()
		if _, ok := channels[r.URL.Path]; !ok {
			channels[r.URL.Path] = make(map[float64]chan stream)
		}
		mutex.Unlock()

		if r.Method == "GET" {
			id := rand.Float64()
			mutex.Lock()
			channels[r.URL.Path][id] = make(chan stream, 30)
			channel := channels[r.URL.Path][id]
			log.Debugf("added listener %f", id)
			mutex.Unlock()

			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Cache-Control", "no-cache, no-store")

			mimetyped := false
			canceled := false
			for {
				select {
				case s := <-channel:
					if s.done {
						canceled = true
					} else {
						if !mimetyped {
							mimetyped = true
							mimetype := mimetype.Detect(s.b).String()
							if mimetype == "application/octet-stream" {
								ext := strings.TrimPrefix(filepath.Ext(r.URL.Path), ".")
								log.Debug("checking extension %s", ext)
								mimetype = filetype.GetType(ext).MIME.Value
							}
							w.Header().Set("Content-Type", mimetype)
							log.Debugf("serving as Content-Type: '%s'", mimetype)
						}
						w.Write(s.b)
						w.(http.Flusher).Flush()
					}
				case <-r.Context().Done():
					log.Debug("consumer canceled")
					canceled = true
				}
				if canceled {
					break
				}
			}

			mutex.Lock()
			delete(channels[r.URL.Path], id)
			log.Debugf("removed listener %f", id)
			mutex.Unlock()
			close(channel)
		} else if r.Method == "POST" {
			buffer := make([]byte, 2048)
			cancel := true
			isdone := false
			lifetime := 0
			for {
				if !doStream {
					select {
					case <-r.Context().Done():
						isdone = true
					default:
					}
					if isdone {
						log.Debug("is done")
						break
					}
					mutex.Lock()
					numListeners := len(channels[r.URL.Path])
					mutex.Unlock()
					if numListeners == 0 {
						time.Sleep(1 * time.Second)
						lifetime++
						if lifetime > 600 {
							isdone = true
						}
						continue
					}
				}
				n, err := r.Body.Read(buffer)
				if err != nil {
					log.Debugf("err: %s", err)
					if err == io.ErrUnexpectedEOF {
						cancel = false
					}
					break
				}
				if doArchive {
					mutex.Lock()
					archived[r.URL.Path].Write(buffer[:n])
					mutex.Unlock()
				}
				mutex.Lock()
				channels_current := channels[r.URL.Path]
				mutex.Unlock()
				for _, c := range channels_current {
					var b2 = make([]byte, n)
					copy(b2, buffer[:n])
					c <- stream{b: b2}
				}
			}
			if cancel {
				mutex.Lock()
				channels_current := channels[r.URL.Path]
				mutex.Unlock()
				for _, c := range channels_current {
					c <- stream{done: true}
				}
			}
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}

	log.Infof("running on port %d", flagPort)
	err = http.ListenAndServe(fmt.Sprintf(":%d", flagPort), http.HandlerFunc(handler))
	if err != nil {
		log.Error(err)
	}
	return
}

type ArchivedFile struct {
	Filename     string
	FullFilename string
	Created      time.Time
}

func listArchived(active map[string]struct{}) (afiles []ArchivedFile) {
	fnames := []string{}
	err := filepath.Walk(flagFolder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				fnames = append(fnames, path)
			}
			return nil
		})
	if err != nil {
		return
	}
	for _, fname := range fnames {
		_, onlyfname := path.Split(fname)
		created := fileCreated(fname)
		if _, ok := active[onlyfname]; !ok {
			afiles = append(afiles, ArchivedFile{
				Filename:     onlyfname,
				FullFilename: fname,
				Created:      created,
			})
		}
	}

	sort.Slice(afiles, func(i, j int) bool {
		return afiles[i].Created.After(afiles[j].Created)
	})

	return
}

func getStreamInfo() (err error) {
	validate := func(input string) error {
		if strings.TrimSpace(input) == "" {
			return fmt.Errorf("name cannot be empty")
		}
		return nil
	}

	if streamName == "" {
		prompt := promptui.Prompt{
			Label:    "Enter stream name: ",
			Validate: validate,
		}

		streamName, err = prompt.Run()
		if err != nil {
			return
		}

	}

	if streamAdvertise == "" {
		prompt2 := promptui.Select{
			Label: "advertise stream?",
			Items: []string{"no advertise", "yes advertise"},
		}
		_, streamAdvertise, err = prompt2.Run()
		if err != nil {
			return
		}

	}
	if strings.Contains(streamAdvertise, "yes") {
		streamAdvertise = "true"
	} else {
		streamAdvertise = "false"
	}

	if streamArchive == "" {
		prompt3 := promptui.Select{
			Label: "archive stream (keep after finished)?",
			Items: []string{"no archive", "yes archive"},
		}
		_, streamArchive, err = prompt3.Run()
		if err != nil {
			return
		}
	}
	if strings.Contains(streamArchive, "yes") {
		streamArchive = "true"
	} else {
		streamArchive = "false"
	}
	return
}

// GetStringInBetween returns empty string if no start or end string found
func GetStringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	s += len(start)
	e := strings.Index(str[s:], end)
	if e == -1 {
		return
	}
	return str[s : s+e]
}

func windowsSelectAudioDevice() (cmd *exec.Cmd, err error) {
	cmd = exec.Command("./ffmpeg.exe", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	output, err := cmd.CombinedOutput()
	// if err != nil {
	// 	panic(err)
	// }
	inputDevices := []string{}
	inputDeviceNames := []string{}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "[dshow") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			foo := strings.Split(line, ` "`)
			if len(foo) < 2 {
				continue
			}
			name := foo[1]
			name = name[:len(name)-2]
			if strings.HasPrefix(foo[1], "@device") {
				inputDeviceNames = append(inputDeviceNames, strings.TrimSpace(name))
			} else {
				inputDevices = append(inputDevices, strings.TrimSpace(name))
			}
		}
	}
	if len(inputDevices) != len(inputDeviceNames) {
		err = fmt.Errorf("devices names do not match %d!=%d", len(inputDevices), len(inputDeviceNames))
	}

	prompt := promptui.Select{
		Label: "Select input device",
		Items: inputDevices,
		Size:  len(inputDevices),
	}

	var i int
	i, streamDevice, err = prompt.Run()
	result := inputDeviceNames[i]

	if err != nil {
		return
	}
	cmd = exec.Command("./ffmpeg.exe", "-f", "dshow", "-i", "audio="+result, "-f", "mp3", "-")
	return
}

func linuxSelectAudioDevice() (cmd *exec.Cmd, err error) {
	cmd = exec.Command("cat", "/proc/asound/cards")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	inputDeviceNames := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "[") {
			inputDeviceNames = append(inputDeviceNames, strings.TrimSpace(line))
		}
	}

	prompt := promptui.Select{
		Label: "Select input device",
		Items: inputDeviceNames,
		Size:  len(inputDeviceNames),
	}

	var i int
	i, streamDevice, err = prompt.Run()

	if err != nil {
		return
	}
	cmd = exec.Command("ffmpeg", "-f", "alsa", "-i", fmt.Sprintf("hw:%d", i), "-f", "mp3", "-")
	return
}

func cast() (err error) {
	err = getStreamInfo()
	if err != nil {
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd, err = windowsSelectAudioDevice()
	case "darwin":
		fmt.Println("MAC operating system")
	case "linux":
		cmd, err = linuxSelectAudioDevice()
	default:
		fmt.Printf("%s.\n", runtime.GOOS)
	}
	if err != nil {
		return
	}

	canceled := false
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			canceled = true
			// sig is a ^C, handle it
			cmd.Process.Kill()
		}
	}()
	// output, err := cmd.CombinedOutput()
	// fmt.Println(string(output))
	// fmt.Println(err)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	err = cmd.Start()
	if err != nil {
		return
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
			Scheme: "http",
			Host:   "localhost:9222",
			// Scheme:   "https",
			// Host:     "broadcast.schollz.com",
			Path:     "/" + streamName + ".mp3",
			RawQuery: "stream=true&advertise=" + streamAdvertise + "&archive=" + streamArchive,
		},
		ProtoMajor:    1,
		ProtoMinor:    1,
		ContentLength: -1,
		Body:          r,
	}

	go func() {
		_, err = client.Do(req)
		if err != nil && !canceled {
			fmt.Println("problem connecting: %s", err.Error())
			cmd.Process.Kill()
		}
	}()

	fmt.Printf("\n\n\n        now streaming '%s' at\n", streamDevice)
	fmt.Printf("\t        https://broadcast.schollz.com/" + streamName + ".mp3\n\n\n")
	cmd.Wait()
	fmt.Println("goodbye.")
	time.Sleep(1 * time.Second)
	return
}
