package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/ritwik310/torrent-client/bencode"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("no torrent file provided")
		return
	}
	fn := os.Args[1]

	torr, err := bencode.ReadTorrent(fn)
	if err != nil {
		fmt.Println("couldn't read/decode the torrent file data", err)
	}

	var pieces string

	if info, ok := torr["info"].(map[string]interface{}); ok {
		for key, val := range info {
			if key == "pieces" {
				fmt.Println("original type of info[\"pieces\"]:", reflect.TypeOf(val))
				pieces = val.(string)
			}
		}

		// fmt.Printf("%s\n", y["pieces"])
	}

	fmt.Println("len:", len(pieces))
	fmt.Printf("%#x\n", pieces)

}
