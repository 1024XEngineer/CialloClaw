package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "missing command")
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "foreground-window":
		info, err := queryForegroundWindow()
		if err != nil {
			_ = writeJSON(ForegroundWindow{})
			return
		}
		if err := writeJSON(info); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "layout-overlays":
		var request LayoutRequest
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		response, err := computeOverlayLayout(request)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if err := writeJSON(response); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, errors.New("unknown command"))
		os.Exit(1)
	}
}
