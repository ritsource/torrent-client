package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

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

	err = tracker.GetPeers()
	if err != nil {
		logrus.Panicf("couldn't read peers, %v", err)
	}

	go func() {
		for {
			time.Sleep(3 * time.Second)
			PrintMemUsage()
		}
	}()

	var wg sync.WaitGroup

	for _, p := range tracker.Peers {
		fmt.Printf("%+v\n", p)
		wg.Add(1)
		go p.Start()
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

	if bToMb(m.TotalAlloc) >= 2000 {
		os.Exit(3)
	}

	fmt.Printf("NumGoroutine -> %v\n", runtime.NumGoroutine())

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
