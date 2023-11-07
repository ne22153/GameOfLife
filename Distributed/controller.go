package Distributed

import (
	"bufio"
	"flag"
	"fmt"
	"net/rpc"
	"os"
)

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool

	ioFilename chan<- string
	ioOutput   chan<- byte
	ioInput    <-chan byte
}

func controller(params Params, channels distributorChannels, keyPresses <-chan rune) {

}

func main() {
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	fmt.Println("Server: ", *server)
	client, _ := rpc.Dial("tcp", *server)
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			panic("AAAA")
		}
	}(client)

	file, err := os.Open("wordlist")
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic("AAAAA")
		}
	}(file)

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		request := Request{World: nil}
		response := new(Response)
		err := client.Call(GoLHandler, request, response)
		if err != nil {
			return
		}
		fmt.Println("Responded: ", response.World)
	}

}
