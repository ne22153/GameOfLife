package SharedSDL

import (
	"fmt"
	"github.com/veandco/go-sdl2/sdl"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

func Run(p Shared.Params, events <-chan Shared.Event, keyPresses chan<- rune) {
	w := NewWindow(int32(p.ImageWidth), int32(p.ImageHeight))
sdlLoop:
	for {
		event := w.PollEvent()
		if event != nil {
			switch e := event.(type) {
			case *sdl.KeyboardEvent:
				switch e.Keysym.Sym {
				case sdl.K_p:
					//When p is pressed, pause the processing and print the current turn that is being processed
					//If p is pressed again resume the processing
					keyPresses <- 'p'
					fmt.Println("P pressed")
				case sdl.K_s:
					//When s is pressed, we need to generate a PGM file with the current state of the board
					keyPresses <- 's'
				case sdl.K_q:
					//When q is pressed, generate a PGM file with the current state of the board then terminate
					keyPresses <- 'q'
					fmt.Println("q pressed")
				case sdl.K_k:
					keyPresses <- 'k'
				}
			}
		}
		select {
		case event, ok := <-events:
			if !ok {
				w.Destroy()
				break sdlLoop
			}
			switch e := event.(type) {
			case Shared.CellFlipped:
				w.FlipPixel(e.Cell.X, e.Cell.Y)
			case Shared.TurnComplete:
				w.RenderFrame()
			case Shared.FinalTurnComplete:
				w.Destroy()
				break sdlLoop
			default:
				if len(event.String()) > 0 {
					fmt.Printf("Completed Turns %-8v%v\n", event.GetCompletedTurns(), event)
				}
			}
		default:
			break
		}
	}

}
