package main

import (
	"fmt"
	"os"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
)

// From median filter in concurrency lab 1
//Modified for the purposes of showing 1-16 threads instead of powers of 2
// Benchmark applies the filter to the ship.png b.N times.
// The time taken is carefully measured by go.
// The b.N  repetition is needed because benchmark results are not always constant.
func BenchmarkFilter(b *testing.B) {
	// Disable all program output apart from benchmark results
	os.Stdout = nil

	// Use a for-loop to run
	for threads := 1; threads <= 16; threads++ {
		b.Run(fmt.Sprintf("%d_workers", threads), func(b *testing.B) {
			for i := 0; i < b.N; i++ {

				params := gol.Params{
					Turns:       1000,
					Threads:     threads,
					ImageWidth:  512,
					ImageHeight: 512,
				}

				keyPresses := make(chan rune, 10)
				events := make(chan gol.Event, 1000)

				go gol.Run(params, events, keyPresses)
				complete := false
				for !complete {
					event := <-events
					switch event.(type) {
					case gol.FinalTurnComplete:
						complete = true
					}
				}
			}
		})
	}
}
