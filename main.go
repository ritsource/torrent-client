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
	for seeders.Len() < 1 {
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
			// for _, s := range seeders {
			// 	fmt.Printf("%v:%v\n", s.IP, s.Port)
			// }

			fmt.Printf("***** Requested: %v\n***** Recieved %v\n", src.Requested, src.Recieved)
		}
	}()

	seeders.Download(src.Torr)
}

// type

// Seeders .
type Seeders []*src.Peer

// Len .
func (sds *Seeders) Len() int {
	return len(*sds)
}

/*
Find is gonna disconnect with all the peers
and reestablish connection with all of them again
*/
func (sds *Seeders) Find(peers []*src.Peer) {
	for _, p := range peers {
		go func(p *src.Peer) {
			err := p.Ping()
			if err == nil {
				(*sds) = append(*sds, p)
			}
		}(p)
	}
}

// Download .
func (sds *Seeders) Download(torr *src.Torrent) {
	pidx := 0
	bidx := 0
	sidx := 0
	pcover := 0
	i := 0

	for {
		i++

		time.Sleep(400 * time.Microsecond)

		// if piece of `pidx` is not found on any of the
		// seeders, increment `pidx` and reset `pcover`
		if pcover >= sds.Len() {
			pidx++
			pcover = 0
		}

		// if all pieces are covered (or, `pidx` is greater
		// than last piece index), reset `pidx` and `bidx`
		if pidx == len(torr.Pieces)-1 {
			bidx = 0
			pidx = 0
		}

		// if all seeds are covered reset `sidx` to start/0
		if sidx >= sds.Len() {
			sidx = 0
		}

		// variables pointing to the data
		piece := torr.Pieces[pidx]

		if bidx == len(piece.Blocks) {
			bidx = 0
			pidx++
			pcover = 0
			continue
		}

		block := piece.Blocks[bidx]
		seeder := (*sds)[sidx]

		// fmt.Printf("sds - %v\n", seeder)

		// if seeder is not free to request piece
		if sds != nil && seeder.Waiting && !seeder.Bitfield[pidx] {
			sidx++
			pcover++
			continue
		}

		if block.Status == src.BlockRequested || block.Status == src.BlockDownloaded {
			bidx++
			continue
		}

		fmt.Println(i)
		seeder.RequestBlock(block)
		fmt.Printf("REQ - PIDX: %v | BIDX: %v | SEED: %v:%v\n", pidx, bidx, seeder.IP, seeder.Port)
		sidx++

		// Download(torr, pidx, )
	}
}
