package infrastructure

import "os"

func GetHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}