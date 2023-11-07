package Distributed

const LIVE = 255
const DEAD = 0

//This file is where we have the game of life algorithm

//Helper function to worker
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

//We add up the values of all neighbouring cells and then divide it by LIVE to determine
//living neighbours count
func calculateAdjacentAlive(inputWorld [][]byte, i, j, imageHeight, imageWidth int) int {
	adjacentAliveCells :=
		int(inputWorld[(i-1+imageHeight)%imageHeight][(j-1+imageWidth)%imageWidth]) +
			int(inputWorld[(i-1+imageHeight)%imageHeight][(j+imageWidth)%imageWidth]) +
			int(inputWorld[(i-1+imageHeight)%imageHeight][(j+1+imageWidth)%imageWidth]) +
			int(inputWorld[(i+imageHeight)%imageHeight][(j-1+imageWidth)%imageWidth]) +
			int(inputWorld[(i+imageHeight)%imageHeight][(j+1+imageWidth)%imageWidth]) +
			int(inputWorld[(i+1+imageHeight)%imageHeight][(j-1+imageWidth)%imageWidth]) +
			int(inputWorld[(i+1+imageHeight)%imageHeight][(j+imageWidth)%imageWidth]) +
			int(inputWorld[(i+1+imageHeight)%imageHeight][(j+1+imageWidth)%imageWidth])
	adjacentAliveCells = adjacentAliveCells / LIVE

	return adjacentAliveCells
}

//Perform the game of life algorithm
func worker(imageHeight int, imageWidth int, inputWorld [][]byte) [][]byte {
	//Create the result world
	updatedWorld := make([][]byte, imageHeight)
	for i := range updatedWorld {
		updatedWorld[i] = make([]byte, imageWidth)
	}

	//Go row by row
	for i, row := range inputWorld {
		for j, tile := range row {
			adjacentAliveCells := 0
			adjacentAliveCells = calculateAdjacentAlive(inputWorld, i, j, imageHeight, imageWidth)
			updatedWorld[i][j] = updateUpdatedWorldTile(tile, updatedWorld[i][j], adjacentAliveCells)
		}
	}
	return updatedWorld
}
