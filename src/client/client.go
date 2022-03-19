package client

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
	"github.com/schollz/streammyaudio/src/clearscreen"
	"github.com/schollz/streammyaudio/src/ffmpeg"
)

type Client struct {
	Name       string
	Advertise  string
	Archive    string
	DeviceName string
	Device     string
	Server     string
	Quality    int
}

func (c *Client) Run() (err error) {
	_ = ffmpeg.Binary()
	defer func() {
		ffmpeg.Clean()
	}()

	clearscreen.ClearScreen()
	fmt.Println("\n" + `     _______..___________..______       _______      ___      .___  ___.     
    /       ||           ||   _  \     |   ____|    /   \     |   \/   |     
   |   (----` + "`" + `` + "`" + `---|  |----` + "`" + `|  |_)  |    |  |__      /  ^  \    |  \  /  |     
    \   \        |  |     |      /     |   __|    /  /_\  \   |  |\/|  |     
.----)   |       |  |     |  |\  \----.|  |____  /  _____  \  |  |  |  |     
|_______/        |__|     | _| ` + "`" + `._____||_______|/__/     \__\ |__|  |__|     
.___  ___. ____    ____         ___      __    __   _______   __    ______   
|   \/   | \   \  /   /        /   \    |  |  |  | |       \ |  |  /  __  \  
|  \  /  |  \   \/   /        /  ^  \   |  |  |  | |  .--.  ||  | |  |  |  | 
|  |\/|  |   \_    _/        /  /_\  \  |  |  |  | |  |  |  ||  | |  |  |  | 
|  |  |  |     |  |         /  _____  \ |  ` + "`" + `--'  | |  '--'  ||  | |  ` + "`" + `--'  | 
|__|  |__|     |__|        /__/     \__\ \______/  |_______/ |__|  \______/  
                                                                             
                                                                            `)
	err = c.cast()
	if err != nil {
		fmt.Println("        no stream initiated, goodbye.")
		time.Sleep(1 * time.Second)
		err = nil
	}
	return
}

func (c *Client) windowsSelectAudioDevice() (cmd *exec.Cmd, err error) {
	cmd = exec.Command(ffmpeg.Binary(), "-list_devices", "true", "-f", "dshow", "-i", "dummy")
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
	i, c.DeviceName, err = prompt.Run()
	result := inputDeviceNames[i]

	if err != nil {
		return
	}
	cmd = exec.Command(ffmpeg.Binary(), "-f", "dshow", "-i", "audio="+result, "-f", "mp3", "-q:a", fmt.Sprint(c.Quality), "-")
	return
}

func (c *Client) linuxSelectAudioDevice() (cmd *exec.Cmd, err error) {
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
	i, c.DeviceName, err = prompt.Run()

	if err != nil {
		return
	}
	cmd = exec.Command("ffmpeg", "-f", "alsa", "-i", fmt.Sprintf("hw:%d", i), "-f", "mp3", "-q:a", fmt.Sprint(c.Quality), "-")
	return
}

func (c *Client) darwinSelectAudioDevice() (cmd *exec.Cmd, err error) {
	cmd = exec.Command(ffmpeg.Binary(), "-f", "avfoundation", "-list_devices", "true", "-i", "dummy")
	output, _ := cmd.CombinedOutput()
	inputDeviceNames := []string{}

	haveAudioDevices := false
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "audio devices") {
			haveAudioDevices = true
			continue
		}
		if strings.Contains(line, "AVFoundation") && haveAudioDevices {
			parts := strings.Split(line, "]")
			if len(parts) < 2 {
				continue
			}
			name := parts[len(parts)-1]
			inputDeviceNames = append(inputDeviceNames, strings.TrimSpace(name))
		}
	}

	prompt := promptui.Select{
		Label: "Select input device",
		Items: inputDeviceNames,
		Size:  len(inputDeviceNames),
	}

	var i int
	i, c.DeviceName, err = prompt.Run()
	if err != nil {
		return
	}
	cmd = exec.Command(ffmpeg.Binary(), "-f", "avfoundation", "-i", fmt.Sprintf(":%d", i), "-f", "mp3", "-q:a", fmt.Sprint(c.Quality), "-")
	return
}

func (c *Client) cast() (err error) {
	err = c.getStreamInfo()
	if err != nil {
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd, err = c.windowsSelectAudioDevice()
	case "darwin":
		cmd, err = c.darwinSelectAudioDevice()
	case "linux":
		cmd, err = c.linuxSelectAudioDevice()
	default:
		fmt.Printf("%s.\n", runtime.GOOS)
	}
	if err != nil {
		return
	}

	canceled := false
	cc := make(chan os.Signal, 1)
	signal.Notify(cc, os.Interrupt)
	go func() {
		for range cc {
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
			Scheme:   strings.Split(c.Server, "://")[0],
			Host:     strings.Split(c.Server, "://")[1],
			Path:     "/" + c.Name + ".mp3",
			RawQuery: "stream=true&advertise=" + c.Advertise + "&archive=" + c.Archive,
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

	fmt.Printf("\n\nnow streaming at\n")
	fmt.Printf("\n%s/%s\n\n", c.Server, c.Name)
	fmt.Printf("press Ctl+C to quit\n")

	cmd.Wait()
	fmt.Println("goodbye.")
	time.Sleep(1 * time.Second)
	return
}

func (c *Client) getStreamInfo() (err error) {
	validate := func(input string) error {
		if strings.TrimSpace(input) == "" {
			return fmt.Errorf("name cannot be empty")
		}
		return nil
	}

	if c.Name == "" {
		prompt := promptui.Prompt{
			Label:    "Enter stream name: ",
			Validate: validate,
		}

		c.Name, err = prompt.Run()
		if err != nil {
			return
		}

	}

	if c.Quality < 0 || c.Quality > 9 {
		prompt2 := promptui.Select{
			Label: "select quality",
			Items: []string{"best (260 kbps)", "good (165 kbps)", "poor (85 kbps)"},
		}
		var q int
		q, _, err = prompt2.Run()
		if err != nil {
			return
		}
		c.Quality = q
		if q == 1 {
			c.Quality = 4
		} else if q == 2 {
			c.Quality = 8
		}
	}

	if c.Advertise == "" {
		prompt2 := promptui.Select{
			Label: "advertise stream?",
			Items: []string{"no advertise", "yes advertise"},
		}
		_, c.Advertise, err = prompt2.Run()
		if err != nil {
			return
		}

	}
	if strings.Contains(c.Advertise, "yes") {
		c.Advertise = "true"
	} else {
		c.Advertise = "false"
	}

	if c.Archive == "" {
		prompt3 := promptui.Select{
			Label: "archive stream (keep after finished)?",
			Items: []string{"no archive", "yes archive"},
		}
		_, c.Archive, err = prompt3.Run()
		if err != nil {
			return
		}
	}
	if strings.Contains(c.Archive, "yes") {
		c.Archive = "true"
	} else {
		c.Archive = "false"
	}
	return
}
