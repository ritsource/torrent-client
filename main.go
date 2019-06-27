package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/ritwik310/torrent-client/info"
	"github.com/ritwik310/torrent-client/queue"
	"github.com/ritwik310/torrent-client/torrent"
	"github.com/ritwik310/torrent-client/tracker"
	"github.com/sirupsen/logrus"
)

func main() {
	// reading command line arguements for torrent file path
	if len(os.Args) < 2 {
		logrus.Errorf("no torrent file provided")
		return
	}
	fn := os.Args[1]

	// // reading the torrent file
	torr, err := torrent.NewTorrent(fn)
	if err != nil {
		logrus.Panicf("%v\n", err)
	}

	// new tracker
	tracker := tracker.NewTracker(torr)

	// check protocol
	switch torr.AnnounceURL.Scheme {
	case "udp":
		// sending connection request to UDP server (the announce host) and reading responses
		tID, connID, err := tracker.ConnUDP(torr.AnnounceURL.Host, info.TransactionID)
		if err != nil {
			panic(err)
		}
		if tID != info.TransactionID {
			panic(fmt.Sprintf("transaction_id is the request and response did not match %v != %v \n", info.TransactionID, tID))
		}

		// once connection request is successfule, sending announce request
		// this will mainly get us a list of seeders for that torrent files
		interval, err := tracker.GetPeersUDP(torr.AnnounceURL.Host, tID, connID)
		if err != nil {
			panic(err)
		}

		fmt.Println("interval:", interval)

	case "http":
		// if the announce scheme is http then send a http tracker request,
		// this poputate tracker with peers
		interval, err := tracker.GetPeersHTTP()
		if err != nil {
			panic(err)
		}

		fmt.Println("interval:", interval)

	default:
		fmt.Printf("unsupported announce protocol, %v\n", torr.AnnounceURL.Scheme)
	}

	que := queue.NewQueue(torr.Pieces)

	var wg sync.WaitGroup

	// var activePeers []*Peers

	for i := 0; i < len(tracker.Peers); i++ {
		p := tracker.Peers[i]
		wg.Add(1)
		go p.GetPieces(tracker.Torrent, &wg, que)
	}

	// time.Sleep(time.Second * 15)

	wg.Wait()

	// for {
	// }

}
