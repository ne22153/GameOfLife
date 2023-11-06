package gol

import (
	"math"
	"strconv"
	"sync"
	"time"
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

func manager(imageHeight int, imageWidth int, inputWorld [][]byte, out chan<- [][]byte, wg *sync.WaitGroup, j int) {
	gameSlice := worker(imageHeight, imageWidth, inputWorld, j)
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

			//We add up the values of all neighbouring cells and then divide it by LIVE to determine
			//living neighbours count
			adjacentAliveCells =
				int(inputWorld[(i-1+imageHeight)%imageHeight][(j-1+imageWidth)%imageWidth]) +
					int(inputWorld[(i-1+imageHeight)%imageHeight][(j+imageWidth)%imageWidth]) +
					int(inputWorld[(i-1+imageHeight)%imageHeight][(j+1+imageWidth)%imageWidth]) +
					int(inputWorld[(i+imageHeight)%imageHeight][(j-1+imageWidth)%imageWidth]) +
					int(inputWorld[(i+imageHeight)%imageHeight][(j+1+imageWidth)%imageWidth]) +
					int(inputWorld[(i+1+imageHeight)%imageHeight][(j-1+imageWidth)%imageWidth]) +
					int(inputWorld[(i+1+imageHeight)%imageHeight][(j+imageWidth)%imageWidth]) +
					int(inputWorld[(i+1+imageHeight)%imageHeight][(j+1+imageWidth)%imageWidth])
			adjacentAliveCells = adjacentAliveCells / LIVE

			var placeHolder = updateUpdatedWorldTile(tile, updatedWorld[i][j], adjacentAliveCells)
			updatedWorld[i][j] = placeHolder
		}
	}

	return updatedWorld
}

//Determine how big the slice of the GoL board that the worker will work on.
//Return list of slice sizes
func distributeSliceSizes(stripSize, threads, imageHeight int) []int {
	stripSizeList := make([]int, threads) //Each index is the strip size for the specific worker

	if (stripSize*threads)-imageHeight == stripSize {
		stripSize--
	}

	sum := 0
	for i, _ := range stripSizeList {
		stripSizeList[i] = stripSize
		sum += stripSize
	}

	//We adjust the final worker's slice size to fit to the pixels
	if sum > imageHeight { //if sum is more than height
		difference := sum - imageHeight
		stripSizeList[len(stripSizeList)-1] -= difference
	} else if imageHeight > sum { //if sum is less the same as height
		difference := imageHeight - sum
		//fmt.Println("difference: ", difference)
		stripSizeList[len(stripSizeList)-1] += difference
	}

	return stripSizeList
}

// creates the strip that the worker will operate on
// currentHeight is pass by reference so that it will update for the next worker
func createStrip(world [][]byte, stripSizeList []int, workerNumber, imageHeight, threads int) [][]byte {
	var topBuffer int
	var endBuffer int
	var startIndex int

	//We initialize the strip
	var strip [][]byte

	//We exploit the fact that every strip size but the last one is the same so we can just precalcualte the currentY
	//cordinate locally
	var normalStripSize int = stripSizeList[0]
	currentY := (normalStripSize) * workerNumber

	if workerNumber == 0 { //starting worker
		topBuffer = imageHeight - 1
		startIndex = 0
		endBuffer = stripSizeList[0] //first worker

		strip = append(strip, world[topBuffer])
		strip = append(strip, world[startIndex:endBuffer+1]...)
	} else if workerNumber == threads-1 { //final worker

		topBuffer = currentY - 1
		startIndex = currentY
		endBuffer = 0

		strip = append(strip, world[topBuffer:imageHeight]...)
		strip = append(strip, world[0])
	} else { //middle workers
		topBuffer = currentY - 1
		startIndex = currentY
		endBuffer = currentY + normalStripSize

		strip = append(strip, world[topBuffer:endBuffer+1]...)
	}

	return strip
}

func getAliveCellsCount(inputWorld [][]byte) int {
	aliveCells := 0

	for _, row := range inputWorld {
		for _, tile := range row {
			if tile == LIVE {
				aliveCells++
			}
		}
	}

	return aliveCells
	//dummy comment
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	var turn int = 0
	var aliveCells int = 0

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
	stripSizeList := distributeSliceSizes(stripSize, p.Threads, p.ImageHeight)
	//Check: if product of stripSize and threads is one more strip than needed

	//We increment this across workers

	aliveCells = getAliveCellsCount(inputWorld)
	//We create a ticker
	aliveCellsTicker := time.NewTicker(2 * time.Second)

	//We make an anonymous goroutine for the ticker
	go func() {
		for {
			select {
			case <-aliveCellsTicker.C:
				c.events <- AliveCellsCount{turn, aliveCells}
			}
		}

	}()

	//We flip the cells
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if inputWorld[i][j] == LIVE {
				c.events <- CellFlipped{turn, util.Cell{i, j}}
			}
		}
	}
	c.events <- TurnComplete{turn}

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

			//We now do split the input world for each thread accordingly
			for j := 0; j < p.Threads; j++ {
				waitGroup.Add(1)

				var strip [][]byte = createStrip(inputWorld, stripSizeList,
					j, p.ImageHeight, p.Threads)
				go manager((stripSizeList[j])+2, p.ImageWidth, strip, workerChannelList[j],
					&waitGroup, j)

			}

			waitGroup.Wait()
			//Go through the channels and read the updated strips into the new world
			for i := 0; i < len(workerChannelList); i++ {
				//worldSection is just a gameslice from a specific worker
				worldSection := <-(workerChannelList[i])
				endBufferIndex := stripSizeList[i] + 1

				//We don't add the top and end buffers (that's what the inner loop's doing)
				newWorld = append(newWorld, worldSection[1:endBufferIndex]...)
			}
		}
		turn++

		for i := 0; i < p.ImageHeight; i++ {
			for j := 0; j < p.ImageWidth; j++ {
				if inputWorld[i][j] != newWorld[i][j] {
					c.events <- CellFlipped{turn, util.Cell{i, j}}
				}
			}
		}
		c.events <- TurnComplete{turn}

		inputWorld = newWorld
		aliveCells = getAliveCellsCount(inputWorld)

	}
	//We make a stripSizeArray to

	// TODO: Execute all turns of the Game of Life.
	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{turn, calculateAliveCells(p, inputWorld)}
	aliveCellsTicker.Stop() //We need to stop the ticker

	c.ioCommand <- ioOutput
	var filename string = strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
	c.ioFilename <- filename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			//WE populate the slice one input at a time.
			c.ioOutput <- inputWorld[i][j]
		}

	}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
