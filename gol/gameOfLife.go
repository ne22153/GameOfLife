package gol

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

//Perform the game of life algorithm
func worker(imageHeight int, imageWidth int, inputWorld [][]byte) [][]byte {

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

			updatedWorld[i][j] = updateUpdatedWorldTile(tile, updatedWorld[i][j], adjacentAliveCells)
		}
	}

	return updatedWorld
}
