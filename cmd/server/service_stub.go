//go:build !windows

package main

// Service stubs for non-Windows platforms

func runService(configPath string) error {
	return nil
}

func handleServiceCommand(args []string) bool {
	return false
}
