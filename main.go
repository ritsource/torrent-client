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

	for _, piece := range src.Torr.Pieces {
		piece.GenBlocks()
	}

	seeders := forceStart(peers)
	if err != nil {
		logrus.Panicf("%v\n", err)
	}

	for _, s := range seeders {
		fmt.Printf("%v:%v\n", s.IP, s.Port)
	}

	return

	pieceidx := 0
	blockidx := 0
	peeridx := 0
	piececoverage := 0 // how many peers has been checked for a piece

	for {
		if piececoverage >= len(peers) {
			pieceidx++
		}

		if pieceidx == len(src.Torr.Pieces)-1 {
			blockidx = 0
			pieceidx = 0
		}

		if peeridx == len(peers) {
			peeridx = 0
		}

		piece := src.Torr.Pieces[pieceidx]
		block := piece.Blocks[blockidx]
		peer := peers[peeridx]

		if !peer.Waiting {
			peeridx++
		}

		if peer.Bitfield[pieceidx] || !peer.Waiting {

			if block.Status == src.BlockExist || block.Status == src.BlockFailed {
				peer.RequestBlock(block)
				// bRequested = append(bRequested, blockidx)
				// i++
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
}

/*
forceStart is gonna disconnect with all the peers
and reestablish connection with all of them again
*/
func forceStart(peers []*src.Peer) []*src.Peer {
	seeders := []*src.Peer{}

	var wg sync.WaitGroup

	for _, p := range peers {
		go p.Ping(&wg)
		wg.Add(1)
	}

	wg.Wait()

	for _, p := range peers {
		if p.UnChoked && p.State == src.PeerBitfieldReady {
			seeders = append(seeders, p)
		}
	}

	return seeders
}
