package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	"uk.ac.bris.cs/gameoflife/util"
)

type IoChannels struct {
	command <-chan IoCommand
	idle    chan<- bool

	filename <-chan string
	output   <-chan uint8
	input    chan<- uint8
}

// IoState is the internal ioState of the io goroutine.
type IoState struct {
	params   Shared.Params
	channels IoChannels
}

// IoCommand allows requesting behaviour from the io (pgm) goroutine.
type IoCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2
const (
	ioOutput IoCommand = iota
	ioInput
	ioCheckIdle
	ioTicker
)

// writePgmImage receives an array of bytes and writes it to a pgm file.
func (io *IoState) writePgmImage() {
	_ = os.Mkdir("out", os.ModePerm)

	// Request a filename from the distributor.
	filename := <-io.channels.filename

	file, ioError := os.Create("../../out/" + filename + ".pgm")
	util.Check(ioError)
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	_, _ = file.WriteString("P5\n")
	//_, _ = file.WriteString("# PGM file writer by modules (https://github.com/owainkenwayucl/pnmmodules).\n")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageWidth))
	_, _ = file.WriteString(" ")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageHeight))
	_, _ = file.WriteString("\n")
	_, _ = file.WriteString(strconv.Itoa(255))
	_, _ = file.WriteString("\n")

	world := make([][]byte, io.params.ImageHeight)
	for i := range world {
		world[i] = make([]byte, io.params.ImageWidth)
	}

	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {

			val := <-io.channels.output
			//if val != 0 {
			//	fmt.Println(x, y)
			//}
			world[y][x] = val
		}
	}

	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			_, ioError = file.Write([]byte{world[y][x]})
			util.Check(ioError)
		}
	}

	ioError = file.Sync()
	util.Check(ioError)

	fmt.Println("File", filename, "output done!")
}

// readPgmImage opens a pgm file and sends its data as an array of bytes.
func (io *IoState) readPgmImage() {

	// Request a filename from the distributor.
	filename := <-io.channels.filename
	fmt.Println(filename)

	data, ioError := ioutil.ReadFile("../../check/images/" + filename + ".pgm")
	util.Check(ioError)
	fmt.Println("File read")
	fields := strings.Fields(string(data))

	if fields[0] != "P5" {
		panic("Not a pgm file")
	}

	width, _ := strconv.Atoi(fields[1])
	if width != io.params.ImageWidth {
		panic("Incorrect width")
	}

	height, _ := strconv.Atoi(fields[2])
	if height != io.params.ImageHeight {
		panic("Incorrect height")
	}

	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic("Incorrect maxval/bit depth")
	}

	image := []byte(fields[4])

	for _, b := range image {
		//fmt.Println("Put in")
		io.channels.input <- b
		//fmt.Println("Taken out")
	}

	fmt.Println("File", filename, "input done!")
}

// startIo should be the entrypoint of the io goroutine.
func startIo(p Shared.Params, c IoChannels) {
	io := IoState{
		params:   p,
		channels: c,
	}

	for {
		select {
		// Block and wait for requests from the distributor
		case command := <-io.channels.command:
			switch command {
			case ioInput:
				fmt.Println("Input triggered")
				io.readPgmImage()
			case ioOutput:
				io.writePgmImage()
			case ioCheckIdle:
				io.channels.idle <- true
			case ioTicker:
				io.channels.idle <- false
			}

		}
	}
}
