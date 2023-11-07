package Distributed

import (
	"bufio"
	"flag"
	"fmt"
	"net/rpc"
	"os"
)

func main() {
	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()
	fmt.Println("Server: ", *server)
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()

	file, err := os.Open("wordlist")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		request := Request{Message: scanner.Text()}
		response := new(Response)
		client.Call(PremiumReverseHandler, request, response)
		fmt.Println("Responded: ", response.Message)
	}

}
