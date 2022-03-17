package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

// clearScreen is a map from the operating system name to functions to execute
// the terminal commands to clear the screen on said OS
var clearScreen map[string]func()

// init initializes the clearScreen variable for MacOS, Linux, & Windows
func init() {
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

var streamName, streamAdvertise, streamArchive, streamDevice string

func main() {
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
	err := cast()
	if err != nil {
		fmt.Println("        no stream initiated, goodbye.")
		time.Sleep(1 * time.Second)
	}
}

func getStreamInfo() (err error) {
	validate := func(input string) error {
		if strings.TrimSpace(input) == "" {
			return fmt.Errorf("name cannot be empty")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Enter stream name: ",
		Validate: validate,
	}

	streamName, err = prompt.Run()
	if err != nil {
		return
	}

	prompt2 := promptui.Select{
		Label: "advertise stream?",
		Items: []string{"no advertise", "yes advertise"},
	}

	var result string
	_, result, err = prompt2.Run()
	if err != nil {
		return
	}
	if strings.Contains(result, "yes") {
		streamAdvertise = "true"
	} else {
		streamAdvertise = "false"
	}

	prompt3 := promptui.Select{
		Label: "archive stream (keep after finished)?",
		Items: []string{"no archive", "yes archive"},
	}

	_, result, err = prompt3.Run()
	if err != nil {
		return
	}
	if strings.Contains(result, "yes") {
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
