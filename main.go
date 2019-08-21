package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/ritwik310/torrent-client/src"
	"github.com/sirupsen/logrus"
)

func main() {
	// reading the `.torrent` file from the command-line arguements
	if len(os.Args) < 2 {
		logrus.Panicf("no `.torrent` file provided")
	}
	fn := os.Args[1]

	// reading the `.torrent` file
	err := src.ReadFile(fn)
	if err != nil {
		logrus.Panicf("%v\n", err)
	}

	peers, err := src.GetPeers()
	if err != nil {
		logrus.Panicf("%v\n", err)
	}

	err = forceStart(peers)
	if err != nil {
		logrus.Panicf("%v\n", err)
	}

	fmt.Printf("%+v\n", src.Status)
}

/*
forceStart is gonna disconnect with all the peers
and reestablish connection with all of them again
*/
func forceStart(peers []*src.Peer) error {
	var wg sync.WaitGroup

	for _, p := range peers {
		go p.Ping(&wg)
		wg.Add(1)
	}

	wg.Wait()
	return nil
}
