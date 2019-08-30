package main

import (
	"fmt"
	"os"
	"time"

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

	seeders := Seeders{}
	seeders.Find(peers)

	i := 0
	for len(seeders) < 1 {
		time.Sleep(2 * time.Second)
		if i >= 10*5/2 {
			logrus.Panicf("Stalled, no seeders found!")
			break
		}
	}

	logrus.Errorf("Downloading..\n")

	go func() {
		for {
			time.Sleep(5 * time.Second)
			fmt.Printf("\n\n")
			for _, s := range seeders {
				fmt.Printf("%v:%v\n", s.IP, s.Port)
			}
		}
	}()

	// return

	pieceidx := 0
	blockidx := 0
	seederidx := 0
	piececoverage := 0 // how many seeders has been checked for a piece

	for {

		if piececoverage >= len(seeders) {
			pieceidx++
		}

		if pieceidx == len(src.Torr.Pieces)-1 {
			blockidx = 0
			pieceidx = 0
		}

		if seederidx >= len(seeders)-1 {
			seederidx = 0
		}

		piece := src.Torr.Pieces[pieceidx]
		block := piece.Blocks[blockidx]
		seeder := seeders[seederidx]

		if !seeder.Waiting {
			seederidx++
		}

		if seeder.Bitfield[pieceidx] || !seeder.Waiting {

			if block.Status == src.BlockExist || block.Status == src.BlockFailed {
				seeder.RequestBlock(block)
				// bRequested = append(bRequested, blockidx)
				// i++
			}

			blockidx++

			if blockidx == len(piece.Blocks) {
				pieceidx++
				piececoverage = 0
				blockidx = 0
			}

			seederidx++
		} else {
			piececoverage++
			seederidx++
		}
	}
}

// type

// Seeders .
type Seeders []*src.Peer

/*
Find is gonna disconnect with all the peers
and reestablish connection with all of them again
*/
func (sdrs *Seeders) Find(peers []*src.Peer) {
	for _, p := range peers {
		go func(p *src.Peer) {
			err := p.Ping()
			if err == nil {
				*sdrs = append(*sdrs, p)
			}
		}(p)
	}
}
