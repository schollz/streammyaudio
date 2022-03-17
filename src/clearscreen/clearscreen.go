package clearscreen

import (
	"os"
	"os/exec"
	"runtime"
)

// clearScreen is a map from the operating system name to functions to execute
// the terminal commands to clear the screen on said OS
var clearScreen map[string]func()

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
