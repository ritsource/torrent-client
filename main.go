package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/ritwik310/torrent-client/info"
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

	// que := queue.NewQueue(torr.Pieces)

	var wg sync.WaitGroup

	go func() {
		for {
			time.Sleep(3 * time.Second)
			PrintMemUsage()
		}
	}()

	// var activePeers []*Peers

	for i := 0; i < len(tracker.Peers); i++ {
		p := tracker.Peers[i]
		wg.Add(1)
		// go p.GetPieces(tracker.Torrent, &wg, que)
		go p.Start()
	}

	time.Sleep(time.Second * 15)

	for _, p := range tracker.Peers {
		if p.UnChoked {
			logrus.Infof("peer found %v\n", p.Conn.RemoteAddr())
		} else {
			p.Stop()
			p.Close()
		}

	}

	wg.Wait()

	// for {
	// }

}

// PrintMemUsage outputs the current, total and OS memory being used. As well as the number
// of garage collection cycles completed.
func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	if bToMb(m.TotalAlloc) >= 80000 {
		os.Exit(3)
	}

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
