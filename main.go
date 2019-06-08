package main

import (
	"fmt"
	"os"

	bencode "github.com/jackpal/bencode-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("no torrent file provided")
		return
	}

	readTorr(os.Args[1])
}

func readTorr(p string) {
	// reading a torrent file
	f, err := os.Open(p)
	if err != nil {
		fmt.Println("couldn't read the torrent file", err)
	}

	// decoding torrent file data
	torr, err := bencode.Decode(f)
	if err != nil {
		fmt.Println("couldn't decode the torrent file data", err)
	}

	// printing result
	fmt.Printf("%+v\n", torr)
}
