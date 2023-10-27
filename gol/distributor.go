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

//Helper function to distributor to find the number of alive cells adjancent to the tile
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

func updateUpdatedWorldTile(inputWorldTile, updatedWorldTile uint8, adjacentAliveCells int) uint8 {
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

func populateNeighboursList(inputWorld [][]uint8, prevRow, row, nextRow []uint8, edgeIndex, tileIndex int) []uint8 {
	neighboursList := []uint8{
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

//Perform the game of life algorithm
func worker(p Params, inputWorld [][]uint8) [][]uint8 {

	//Create the result world
	updatedWorld := make([][]uint8, p.ImageHeight)
	for i := range updatedWorld {
		updatedWorld[i] = make([]uint8, p.ImageWidth)
	}

	var edge int = p.ImageHeight - 1

	//Go row by row
	for i, row := range inputWorld {

		//We find the rows above and below the current row the tile is at
		var prevRow []uint8 = inputWorld[((i + len(updatedWorld) - 1) % len(updatedWorld))]
		var nextRow []uint8 = inputWorld[((i + len(updatedWorld) + 1) % len(updatedWorld))]

		//Go through the elements in each row
		for j, tile := range row {

			//populate a list of neighbors of the tile
			var neighboursList []uint8 = populateNeighboursList(inputWorld, prevRow, row, nextRow, edge, j)

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

	// TODO: Create a 2D slice to store the world.

	var turn int = 0

	//We create the worlds
	inputWorld := make([][]uint8, p.ImageHeight)
	for i := 0; i < p.ImageHeight; i++ {
		inputWorld[i] = make([]uint8, p.ImageWidth)
	}

	// contact the io
	c.ioFilename <- (strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight))
	c.ioCommand <- 1

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			//WE populate the slice one input at a time.
			inputWorld[i][j] = <-(c.ioInput)
		}
	}

	//Run the game of life algorithm for specified number of turns
	for i := 0; i < p.Turns; i++ {
		inputWorld = worker(p, inputWorld)
		turn++
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
