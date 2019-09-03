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

	f, err := os.Create(src.Torr.Files[0].Path)
	if err != nil {
		logrus.Errorf("%v\n", err)
		return
	}
	f.Close()

	logrus.Errorf("Downloading..\n")

	go func() {
		for {
			time.Sleep(5 * time.Second)
			fmt.Printf("\n\n")
			x := 0
			y := 0
			for _, s := range seeders {
				if s.State == src.PeerBitfieldReady {
					x++
				}
				y++
			}

			fmt.Printf("***** Requested: %v\n***** Recieved %v\n***** ActiveSds: %v\n***** AllSeeds: %v\n", src.Requested, src.Recieved, x, y)
		}
	}()

	pidx := 0
	bidx := 0
	sidx := 0
	pcover := 0
	in := 0

	err = seeders.Download(src.Torr, &pidx, &bidx, &sidx, &pcover, &in)

	fmt.Println("Fuck! Error", err)
	for {
	}
}

// func x(sds *Seeders) {
// 	pidx := 0
// 	bidx := 0
// 	sidx := 0
// 	pcover := 0
// 	i := 0

// 	err := sds.Download(src.Torr, &pidx, &bidx, &sidx, &pcover, &i)
// 	if err != nil && err.Error() == "not enough seeders" {
// 		for
// 		return
// 	}
// }

// type

// Seeders .
type Seeders []*src.Peer

// Len .
func (sds *Seeders) Len() int {
	return len(*sds)
}

// Active .
func (sds *Seeders) Active() int {
	x := 0
	for _, s := range *sds {
		if s.State == src.PeerBitfieldReady {
			x++
		}
	}

	return x
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
func (sds *Seeders) Download(torr *src.Torrent, pidx, bidx, sidx, pcover, i *int) error {
	// pidx := 0
	// bidx := 0
	// sidx := 0
	// pcover := 0
	// i := 0

	if sds.Active() <= int(sds.Len()/2) {
		return fmt.Errorf("not enough seeders")
	}

	*i++

	time.Sleep(100 * 10 * time.Millisecond)

	// if piece of `pidx` is not found on any of the
	// seeders, increment `pidx` and reset `pcover`
	if *pcover >= sds.Len() {
		*pidx++
		*pcover = 0
	}

	// if all pieces are covered (or, `pidx` is greater
	// than last piece index), reset `pidx` and `bidx`
	if *pidx >= len(torr.Pieces)-1 {
		// if *pidx >= len(torr.Pieces)-1 || len(seeder.Bitfield) - 1 {
		*bidx = 0
		*pidx = 0
	}

	// if all seeds are covered reset `sidx` to start/0
	if *sidx >= sds.Len() {
		*sidx = 0
	}

	// variables pointing to the data
	piece := torr.Pieces[*pidx]

	if *bidx == len(piece.Blocks) {
		*bidx = 0
		*pidx++
		*pcover = 0
		// continue
		return sds.Download(torr, pidx, bidx, sidx, pcover, i)
	}

	block := piece.Blocks[*bidx]
	seeder := (*sds)[*sidx]

	// if seeder is not free to request piece
	if !(*pidx >= len(seeder.Bitfield)) {
		fmt.Printf("State: %v, Wait: %v, vBitVal: %v\t\t%v:%v\n", seeder.State, seeder.Waiting, seeder.Bitfield[*pidx], seeder.IP, seeder.Port)
	}
	if seeder == nil || seeder.Waiting || *pidx >= len(seeder.Bitfield) || !seeder.Bitfield[*pidx] {
		*sidx++
		*pcover++
		// continue
		return sds.Download(torr, pidx, bidx, sidx, pcover, i)
	}

	if block.Status == src.BlockRequested || block.Status == src.BlockDownloaded {
		*bidx++
		// continue
		return sds.Download(torr, pidx, bidx, sidx, pcover, i)
	}

	err := seeder.RequestBlock(block)

	switch err {
	case nil:
		// padd
	case src.ErrDisconnected:
		logrus.Infof("reestablishing connection with - %v:%v\n", seeder.IP, seeder.Port)
		seeder.Reset()
		fmt.Println("xxxxxv")
		go func(sd *src.Peer) {
			fmt.Println("xxxxxf")
			sd.Ping()
			fmt.Printf("********************************Fuck\nFuck\nFuck\nFuck%v\n", *sd)
		}(seeder)
	default:
		logrus.Warnf("request couldn't be sent, %v", err)
	}

	fmt.Printf("REQ - PIDX: %v | BIDX: %v | SEED: %v:%v\n", *pidx, *bidx, seeder.IP, seeder.Port)
	*sidx++

	return sds.Download(torr, pidx, bidx, sidx, pcover, i)

}
