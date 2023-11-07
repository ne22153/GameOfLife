package gol

import "strconv"

//Helper function of distributor. We use this to create an initial world map from a file name
func writeFromFileIO(imageHeight, imageWidth int, c distributorChannels) [][]byte {

	//We create the worlds
	var world [][]byte = make([][]byte, imageHeight)
	for i := 0; i < imageHeight; i++ {
		world[i] = make([]byte, imageWidth)
	}

	//We set the command to input to be able to read from the file
	c.ioFilename <- strconv.Itoa(imageWidth) + "x" + strconv.Itoa(imageHeight)
	c.ioCommand <- ioInput

	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			//We populate the slice one input at a time.
			world[i][j] = <-(c.ioInput)
		}
	}

	return world
}

//Helper function of distributor. We use this to create a .pgm file from a given world map
func writeToFileIO(world [][]byte, p Params, filename string,
	c distributorChannels) {
	c.ioCommand <- ioOutput
	c.ioFilename <- filename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			c.ioOutput <- world[i][j]
		}
	}
}
