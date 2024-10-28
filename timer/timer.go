package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(180 * time.Second)

	seconds := 0

	for {
		select {
		case <-ticker.C:
			seconds++
			fmt.Printf("Seconds from start: %d\n", seconds)
		case <-timeout:
			fmt.Println("Exiting after 180 seconds")
			os.Exit(1)
		}
	}
}
