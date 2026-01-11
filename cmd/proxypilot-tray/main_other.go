//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("proxypilot-tray is currently supported on Windows only.")
}
