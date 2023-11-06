package gol

import (
	"math"
	"os"
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

func manager(imageHeight int, imageWidth int, inputWorld [][]byte, out chan<- [][]byte, wg *sync.WaitGroup, j int) {
	gameSlice := worker(imageHeight, imageWidth, inputWorld)
	out <- gameSlice
	defer wg.Done()
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

//Manages the key press interrupts
func goPressTrack(inputWorld [][]byte, keyPresses <-chan rune, c distributorChannels, p Params, turn chan int, aliveCellsTicker *time.Ticker, pauseChannel chan bool) {
	var turns = 0
	var paused = false
	for {
		select {
		case key := <-keyPresses:
			if key == 's' {
				//When s is pressed, we need to generate a PGM file with the current state of the board
				c.ioCommand <- ioOutput
				var filename string = strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
				c.ioFilename <- filename
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						c.ioOutput <- inputWorld[i][j]
					}
				}
			} else if key == 'p' {
				//When p is pressed, pause the processing and print the current turn that is being processed
				//If p is pressed again resume the processing
				if paused {
					c.events <- StateChange{turns, Executing}
					paused = !paused
					pauseChannel <- true
				} else {
					c.events <- StateChange{turns, Paused}
					paused = !paused
				}

			} else if key == 'q' {
				//When q is pressed, generate a PGM file with the current state of the board then terminate
				c.ioCommand <- ioOutput
				var filename string = strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
				c.ioFilename <- filename
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						c.ioOutput <- inputWorld[i][j]
					}
				}

				c.ioCommand <- ioCheckIdle
				<-c.ioIdle

				c.events <- StateChange{turns, Quitting}
				aliveCellsTicker.Stop()
				close(c.events)
				os.Exit(0)
			}
		//When turn is incremented, we're informed of the change
		case t := <-turn:
			turns = t
			if !paused {
				pauseChannel <- true
			}
		}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

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

	turnChannel := make(chan int)
	pauseChannel := make(chan bool)
	go goPressTrack(inputWorld, keyPresses, c, p, turnChannel, aliveCellsTicker, pauseChannel)

	//We flip the cells
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if inputWorld[i][j] == LIVE {
				c.events <- CellFlipped{turn, util.Cell{j, i}}
			}
		}
	}

	//Run the GoL algorithm for specificed number of turns
	for i := 0; i < p.Turns; i++ {
		var newWorld [][]byte
		if p.Threads == 1 {
			newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
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
		turnChannel <- turn

		//fmt.Println(turn)
		aliveCells = getAliveCellsCount(newWorld)
		//fmt.Println(aliveCells)
		<-pauseChannel
		for i := 0; i < p.ImageHeight; i++ {
			for j := 0; j < p.ImageWidth; j++ {
				//If the cell has changed since the last iteration, we need to send an event to say so
				if inputWorld[i][j] != newWorld[i][j] {
					c.events <- CellFlipped{turn, util.Cell{j, i}}
				}
			}
		}
		c.events <- TurnComplete{turn}

		inputWorld = newWorld

	}

	c.events <- FinalTurnComplete{turn, calculateAliveCells(p, inputWorld)}
	aliveCellsTicker.Stop() //We need to stop the ticker

	c.ioCommand <- ioOutput
	var filename string = strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
	c.ioFilename <- filename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			//We populate the slice one input at a time.
			c.ioOutput <- inputWorld[i][j]
		}

	}

	//Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	aliveCellsTicker.Stop()
	//Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
