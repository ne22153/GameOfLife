package gol

import (
	"fmt"
	"math"
	"strconv"
	"sync"
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
	ioOutput   chan<- byte
	ioInput    <-chan byte
}

//Helper function to distributor to find the number of alive cells adjancent to the tile
func calculateAliveCells(p Params, world [][]byte) []util.Cell {
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

func updateUpdatedWorldTile(inputWorldTile, updatedWorldTile byte, adjacentAliveCells int) byte {
	// if the element is dead, then run through those checks
	if inputWorldTile == DEAD {
		if adjacentAliveCells > 0 {
			//fmt.Println(num, ", world", index2, index)
		}
		if adjacentAliveCells == 3 {
			updatedWorldTile = LIVE
		} else {
			updatedWorldTile = DEAD
		}

	} else {
		if adjacentAliveCells < 2 || adjacentAliveCells > 3 {
			updatedWorldTile = DEAD
		} else {
			updatedWorldTile = LIVE
		}

	}

	return updatedWorldTile
}

func populateNeighboursList(inputWorld [][]byte, prevRow, row, nextRow []byte, edgeIndex, tileIndex int) []byte {
	neighboursList := []byte{
		prevRow[tileIndex],
		nextRow[tileIndex]}

	//Boundary: check if the tile is to the very left of the screen, if so take left neighbours from the other side
	if tileIndex > 0 {
		neighboursList = append(
			neighboursList,
			row[(tileIndex-1)%len(inputWorld)],
			prevRow[(tileIndex-1)%len(row)],
			nextRow[(tileIndex-1)%len(row)])
	} else {
		neighboursList = append(
			neighboursList,
			row[edgeIndex],
			prevRow[edgeIndex],
			nextRow[edgeIndex])
	}

	//Boundary: check if the tile is to the very right of the screen, if so take right neighbours from the other side
	if tileIndex < edgeIndex {
		neighboursList = append(
			neighboursList,
			row[(tileIndex+1)%len(row)],
			prevRow[(tileIndex+1)%len(row)],
			nextRow[(tileIndex+1)%len(row)])
	} else {
		neighboursList = append(
			neighboursList,
			row[0],
			prevRow[0],
			nextRow[0])
	}

	return neighboursList
}

func manager(imageHeight int, imageWidth int, inputWorld [][]byte, out chan<- [][]byte, wg *sync.WaitGroup, j int) {
	gameSlice := worker(imageHeight, imageWidth, inputWorld, j)
	fmt.Println(j, ": Input game: ", inputWorld, "\n Output game: ", gameSlice)
	out <- gameSlice
	defer wg.Done()
}

//Perform the game of life algorithm
func worker(imageHeight int, imageWidth int, inputWorld [][]byte, j int) [][]byte {

	//Create the result world
	updatedWorld := make([][]byte, imageHeight)
	for i := range updatedWorld {
		updatedWorld[i] = make([]byte, imageWidth)
	}

	var edge int = imageHeight - 1

	//Go row by row
	for i, row := range inputWorld {

		//We find the rows above and below the current row the tile is at
		var prevRow []byte = inputWorld[((i + len(updatedWorld) - 1) % len(updatedWorld))]
		var nextRow []byte = inputWorld[((i + len(updatedWorld) + 1) % len(updatedWorld))]

		//Go through the elements in each row
		for j, tile := range row {

			//populate a list of neighbors of the tile
			var neighboursList []byte = populateNeighboursList(inputWorld, prevRow, row, nextRow, edge, j)

			//Find number of adjacent tiles that are alive
			var adjacentAliveCells int = 0
			for _, value := range neighboursList {
				if value > 0 {
					adjacentAliveCells++
				}
			}

			updatedWorld[i][j] = updateUpdatedWorldTile(tile, updatedWorld[i][j], adjacentAliveCells)

		}
	}

	return updatedWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	var turn int = 0

	//We create the worlds
	inputWorld := make([][]byte, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		inputWorld[i] = make([]byte, p.ImageWidth)
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

	//How to change this for concurrency:
	//Run the worker as goroutines for the number of threads
	//Break down the board into strips whose size is defined by the number of threads
	//Make the workers pass the finished strip of board back down a channel to remove its return value
	//Add mutex locks for accessing the overall board

	//Finds the appropriate size to be passed to each worker
	StripSize := math.Ceil(float64(p.ImageHeight) / float64(p.Threads))
	var StripArray = make([]float64, p.Threads)
	//16 div 5 = 3.2, ceil is 4
	//
	var stripSizeInt int = int(StripSize)
	//We reduce the strip size by one if the strips are one more than needed
	if (stripSizeInt*p.Threads)-p.ImageHeight == stripSizeInt {
		StripSize = StripSize - 1
		fmt.Println("Trig ", StripSize)
	}
	for i := 0; i < p.Threads; i++ {
		StripArray[i] = StripSize
	}
	fmt.Println(StripArray)
	//Run the game of life algorithm for specified number of turns
	for i := 0; i < p.Turns; i++ {
		var newWorld [][]byte
		//If there's only one worker, run as normal without the manager
		if p.Threads == 1 {
			newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld, 1)
		} else {
			//If there's more than one worker, set up a wait group and comms channels for each strip
			var wg sync.WaitGroup
			var genSlice []chan [][]byte
			for j := 0; j < p.Threads; j++ {
				newSlice := make(chan [][]byte, 2)
				genSlice = append(genSlice, newSlice)
			}
			//For each thread, split the input world and pass it to the manager goroutine
			for j := 0; j < p.Threads; j++ {
				wg.Add(1)
				var startIndex int
				var midIndex int
				var endIndex int
				var strip [][]byte
				//If it's the first thread, it should have the last line at [0]
				if j == 0 {
					//Set the start index as the last row on the board
					startIndex = p.ImageHeight - 1
					midIndex = 0
					endIndex = (j + 1) * int(StripArray[j])
					strip = append([][]byte{inputWorld[startIndex]}, inputWorld[midIndex:endIndex]...)
				} else if j == p.Threads-1 {
					startIndex = j*int(StripArray[j]) - 1
					midIndex = j * int(StripArray[j])
					endIndex = 0

					strip = append([][]byte{inputWorld[startIndex]}, inputWorld[midIndex:p.ImageHeight]...)
					//Fill out any remainder space on the last strip
					StripArray[j] = float64(p.ImageHeight - startIndex - 1)
				} else { //Middle of the strip
					startIndex = j*int(StripArray[j]) - 1
					midIndex = j * int(StripArray[j])
					endIndex = (j + 1) * int(StripArray[j])

					strip = append([][]byte{inputWorld[startIndex]}, inputWorld[midIndex:endIndex]...)
				}

				//Add on the last line
				strip = append(strip, inputWorld[endIndex])

				//fmt.Println("Bruh this is the size of the strip", len(strip), "adjusted: ", len(strip)-2)

				//fmt.Println(StripSize, strip)
				//Pass the strip to the manager goroutine to process
				go manager(int(StripArray[j])+2, p.ImageWidth, strip, genSlice[j], &wg, j)
			}
			//Wait until all strips are finished running
			wg.Wait()

			//Go through the channels and read the updated strips into the new world
			//fmt.Println(StripSize)
			var count = 0
			for _, element := range genSlice {
				var i = 0
				for _, line := range <-element {
					if i != 0 && i != int(StripArray[count]+1) {
						fmt.Println(count, line)
						newWorld = append(newWorld, line)
					} else {
						fmt.Println("Skipped")
					}
					i++
				}
				count++
			}
		}
		inputWorld = newWorld
		turn++
	}

	// TODO: Execute all turns of the Game of Life.
	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{turn, calculateAliveCells(p, inputWorld)}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
