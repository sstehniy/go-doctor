package main

import "time"

func loop() {
	for {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-time.Tick(time.Second):
		}
	}
}
