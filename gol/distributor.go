package gol

import (
	"fmt"
	"strconv"
	"uk.ac.bris.cs/gameoflife/util"
)

//We make consts as stand ins for alive or dead
const LIVE = 255
const DEAD = 0

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool

	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func findAliveCells(p Params, world [][]uint8) []util.Cell {
	aliveCells := make([]util.Cell, p.ImageWidth*p.ImageHeight)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[x][y] == 255 {
				newCell := util.Cell{x, y}
				aliveCells = append(aliveCells, newCell)
			}
		}
	}
	return aliveCells
}

func calculateAliveCells(p Params, world [][]uint8) []util.Cell {
	coordinates := []util.Cell{}
	for index, row := range world {
		for index2, _ := range row {
			if world[index][index2] > 0 {
				coordinates = append(coordinates, util.Cell{index2, index})
			}
		}
	}
	return coordinates
}

func worker(p Params, inputWorld [][]uint8) [][]uint8 {
	updatedWorld := make([][]uint8, p.ImageHeight)

	for i := range updatedWorld {
		updatedWorld[i] = make([]byte, p.ImageWidth)
	}

	for index, element := range inputWorld {
		var prevElement, nextElement []byte

		prevElement = inputWorld[((index + len(updatedWorld) - 1) % len(updatedWorld))]
		nextElement = inputWorld[(index+len(updatedWorld)+1)%len(updatedWorld)]

		var edge = p.ImageHeight - 1

		for index2, element2 := range element {
			// makes a list of the neighbours of the element2
			values := []byte{prevElement[index2], nextElement[index2]}
			if index2 > 0 {
				values = append(values, element[(index2-1)%len(inputWorld)], prevElement[(index2-1)%len(element)], nextElement[(index2-1)%len(element)])
			} else {
				values = append(values, element[edge], prevElement[edge], nextElement[edge])
			}
			if index2 < edge {
				values = append(values, element[(index2+1)%len(element)], prevElement[(index2+1)%len(element)], nextElement[(index2+1)%len(element)])
			} else {
				values = append(values, element[0], prevElement[0], nextElement[0])
			}
			num := 0
			for _, value := range values {
				if value > 0 {
					num += 1
				}
			}
			// if the element is dead, then run through those checks
			if element2 == 0 {
				if num > 0 {
					//fmt.Println(num, ", world", index2, index)
				}
				if num == 3 {
					updatedWorld[index][index2] = 255
				} else {
					updatedWorld[index][index2] = 0
				}

			} else {
				if num < 2 || num > 3 {
					updatedWorld[index][index2] = 0
				} else {
					updatedWorld[index][index2] = 255
				}

			}
		}
	}
	return updatedWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.

	//We create the worlds
	inputWorld := make([][]uint8, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		inputWorld[i] = make([]uint8, p.ImageWidth)
	}

	// contact the io
	c.ioFilename <- strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioCommand <- 1

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			//WE populate the slice one input at a time.
			inputWorld[i][j] = <-(c.ioInput)
		}
	}
	turn := 0

	fmt.Println(calculateAliveCells(p, inputWorld))

	//updatedWorld = worker(p, inputWorld)
	for i := 0; i < p.Turns; i++ {
		inputWorld = worker(p, inputWorld)
		turn++
		//fmt.Println("Passed turn")
	}

	// TODO: Execute all turns of the Game of Life.
	fmt.Println("Finished")
	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{turn, calculateAliveCells(p, inputWorld)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
