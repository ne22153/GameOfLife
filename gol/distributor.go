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
	//fmt.Println(j, ": Input game: ", inputWorld, "\n Output game: ", gameSlice)
	out <- gameSlice
	defer wg.Done()
}

//Perform the game of life algorithm
func worker(imageHeight int, imageWidth int, inputWorld [][]byte, count int) [][]byte {

	//Create the result world
	updatedWorld := make([][]byte, imageHeight)
	for i := range updatedWorld {
		updatedWorld[i] = make([]byte, imageWidth)
	}
	
	//Go row by row
	for i, row := range inputWorld {

		//We find the rows above and below the current row the tile is at

		//Go through the elements in each row
		for j, tile := range row {

			adjacentAliveCells := 0
			//populate a list of neighbors of the tile
			for k := -1; k <= 1; k++ {
				for l := -1; l <= 1; l++ {

					//Since GOL wraps around the screen we need to ensure that the indexes remain in bounds
					//adding a +p.imagewidth/height incase i+k or j+l evaluates to -1
					adjustedIIndex := (i + k + imageHeight) % (imageHeight)
					adjustedJIndex := (j + l + imageWidth) % (imageWidth)

					if inputWorld[adjustedIIndex][adjustedJIndex] == LIVE {
						//don't count the cell itself (offset k , l are both zero)
						if (k == 0) && (l == 0) {
							continue
						} else {
							adjacentAliveCells++
						}

					}
				}
			}
			var placeHolder = updateUpdatedWorldTile(tile, updatedWorld[i][j], adjacentAliveCells)
			updatedWorld[i][j] = placeHolder
			//if placeHolder == LIVE {
			//	fmt.Println(count, ":", j, i)
			//}

		}
	}

	//for i := 1; i < len(updatedWorld)-1; i++ {
	//	fmt.Println(inputWorld[i], "         ", updatedWorld[i])
	//}
	//fmt.Println()
	//
	//fmt.Println("aaaa")
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

	//We need to find the strip sized passed to each worker
	stripSize := int(math.Ceil(float64(p.ImageHeight / p.Threads)))

	stripSizeList := make([]int, p.Threads) //Each index is the strip size for the specific worker
	//Check: if product of stripSize and threads is one more strip than needed
	if (stripSize*p.Threads)-p.ImageHeight == stripSize {
		stripSize--
	}
	sum := 0
	for i, _ := range stripSizeList {
		stripSizeList[i] = stripSize
		sum += stripSize
	}

	fmt.Println("sum: ", sum)

	//We adjust the final worker's slice size to fit to the pixels
	if sum > p.ImageHeight { //if sum is more than heigth
		difference := sum - p.ImageHeight
		stripSizeList[len(stripSizeList)-1] -= difference
	} else if p.ImageHeight > sum { //if sum is less the same as height
		difference := p.ImageHeight - sum
		fmt.Println("difference: ", difference)
		stripSizeList[len(stripSizeList)-1] += difference
	}

	//We increment this across workers

	//Run the GoL algorithm for specificed number of turns
	for i := 0; i < p.Turns; i++ {
		var newWorld [][]byte
		if p.Threads == 1 {
			newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld, 1)
		} else {
			//	We need to make a wait group and communication channels for each strip
			var waitGroup sync.WaitGroup
			var workerChannelList []chan [][]byte = make([]chan [][]byte, p.Threads)
			for j := 0; j < p.Threads; j++ {
				var workerChannel chan [][]byte = make(chan [][]byte, 2)
				workerChannelList[j] = workerChannel
			}

			currentHeight := 0
			//We now do split the input world for each thread accordingly
			for j := 0; j < p.Threads; j++ {
				waitGroup.Add(1)

				//topBuffer and endBuffer are just one extra top and bottom row for proper reading
				//in the GoL algorithm
				var topBuffer int
				var endBuffer int
				var startIndex int

				//We initialize the strip
				var strip [][]byte

				//fmt.Println("current height: ", currentHeight)
				if j == 0 { //starting worker
					topBuffer = p.ImageHeight - 1
					startIndex = 0
					endBuffer = currentHeight + stripSizeList[j]

					currentHeight += stripSizeList[j]
					strip = append(strip, inputWorld[topBuffer])
					strip = append(strip, inputWorld[startIndex:endBuffer]...)
					strip = append(strip, inputWorld[endBuffer])
				} else if j == p.Threads-1 { //final worker

					//fmt.Println("Entering final worker")
					topBuffer = currentHeight - 1
					startIndex = currentHeight
					endBuffer = 0

					strip = append(strip, inputWorld[topBuffer])
					strip = append(strip, inputWorld[startIndex:p.ImageHeight]...)
					strip = append(strip, inputWorld[0])
				} else { //middle workers
					topBuffer = currentHeight - 1
					startIndex = currentHeight
					endBuffer = currentHeight + stripSizeList[j]

					currentHeight += stripSizeList[j]
					strip = append(strip, inputWorld[topBuffer])
					strip = append(strip, inputWorld[startIndex:endBuffer]...)
					strip = append(strip, inputWorld[endBuffer])
				}

				//fmt.Println("current height after: ", currentHeight)
				//fmt.Println("strip size:", stripSizeList[j])
				//fmt.Println("worker: ", j)
				//for _, entry := range strip {
				//	fmt.Println(entry, "oof")
				//}
				//fmt.Println("bruh")

				go manager((stripSizeList[j])+2, p.ImageWidth, strip, workerChannelList[j],
					&waitGroup, j)

			}

			waitGroup.Wait()
			//Go through the channels and read the updated strips into the new world
			fmt.Println(stripSizeList)
			var count = 0
			for _, element := range workerChannelList {
				var i = 0
				for _, line := range <-element {
					if i != 0 && i != int(stripSizeList[count]+1) {
						//fmt.Println(count, line)
						newWorld = append(newWorld, line)
					} else {
						//fmt.Println("Skipped")
					}
					i++
				}
				count++
			}
		}
		inputWorld = newWorld
		turn++
	}

	fmt.Println(stripSizeList)
	//We make a stripSizeArray to

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
