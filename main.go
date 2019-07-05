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

	// fmt.Println("Getting peers")
	err = tracker.GetPeers()
	if err != nil {
		logrus.Panicf("couldn't read peers, %v", err)
	}

	// fmt.Println("len(torr.Pieces)", len(torr.Pieces))

	go func() {
		for {
			time.Sleep(3 * time.Second)
			fmt.Println("x-x-x-x")
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			fmt.Printf("\tGo routines -> %v\n", runtime.NumGoroutine())
			tot := m.TotalAlloc / 1024 / 1024
			fmt.Printf("\tMemory alloc -> %v MiB\n", tot)
			fmt.Println("x-x-x-x")

			if tot > 50 {
				os.Exit(3)
			}
		}
	}()

	ch := make(chan string)

	go func(t *torrent.Torrent, ch chan string) {
		for _, p := range t.Pieces {
			p.Blockize()
		}
		ch <- "done"
	}(torr, ch)

	for i, p := range tracker.Peers {
		if i > 9 {
			break
		}
		fmt.Printf("%+v\n", p)
		go p.Start()
	}

	time.Sleep(20 * time.Second)

	<-ch
	// for _, piece := range torr.Pieces {
	// 	fmt.Printf("%+v\n", *piece)
	// }
	// var wg sync.WaitGroup

	pieceidx := 0
	blockidx := 0
	peeridx := 0
	piececoverage := 0 // how many peers has been checked for a piece

	var bRequested []int

	go func(br *[]int) {
		for {
			time.Sleep(3 * time.Second)
			fmt.Println("------------------->", len(bRequested))
		}
	}(&bRequested)

	var wg sync.WaitGroup

	i := 0

	for {

		// if i > 100 {
		// 	break
		// }

		time.Sleep(time.Millisecond * 100)
		// if all peers has been checked and none of them contains
		// the piece skip that piece by pieceidx++, piececoverage
		// contains how many peers has been checked for a piece
		if piececoverage >= len(tracker.Peers) {
			pieceidx++
		}

		if pieceidx == len(torr.Pieces) {
			time.Sleep(5 * time.Second)
			blockidx = 0
			pieceidx = 0
		}

		if peeridx == len(tracker.Peers) {
			peeridx = 0
		}

		piece := torr.Pieces[pieceidx]
		block := piece.Blocks[blockidx]
		peer := tracker.Peers[peeridx]

		if !peer.UnChoked {
			peeridx++
			continue
		}

		if len(peer.Bitfield) >= pieceidx+1 && peer.Bitfield[pieceidx] == 1 {

			if block.Status == torrent.BlockExist || block.Status == torrent.BlockFailed {
				peer.RequestPiece(block)
				bRequested = append(bRequested, blockidx)
				i++
			}

			blockidx++

			if blockidx == len(piece.Blocks) {
				pieceidx++
				piececoverage = 0
				blockidx = 0
			}

			peeridx++
		} else {
			piececoverage++
			peeridx++
		}

	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			fmt.Println("Hohahahahhaha")
		}
	}()
	wg.Add(1)
	wg.Wait()

}
