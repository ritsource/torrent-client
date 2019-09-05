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

	// fmt.Println()
	logrus.Errorln("Downloadijng.....................................")
	seeders.Download()

}

// Seeders .
type Seeders []*src.Peer

// Len .
func (sdrs *Seeders) Len() int {
	return len(*sdrs)
}

// Active .
func (sdrs *Seeders) Active() int {
	x := 0
	for _, s := range *sdrs {
		if s.IsReady() {
			x++
		}
	}
	return x
}

/*
Find is gonna disconnect with all the peers
and reestablish connection with all of them again
*/
func (sdrs *Seeders) Find(peers []*src.Peer) {
	for _, p := range peers {
		go func(p *src.Peer) {
			err := p.Ping()
			if err == nil {
				(*sdrs) = append(*sdrs, p)
			}
		}(p)
	}
}

// Download .
func (sdrs *Seeders) Download() {
	pidx := 0
	sidx := 0
	// pcovered := 0

	downloaded := 0

	go func(d *int) {
		for {
			time.Sleep(30 * time.Second)
			fmt.Printf("*******************\n\n Downloaded = %v \n\n*****\n", *d)
			if *d >= len(src.Torr.Pieces) {
				break
			}
		}
	}(&downloaded)

	for {
		time.Sleep(100 * time.Millisecond)

		if pidx >= len(src.Torr.Pieces) {
			x := false
			d := 0
			for _, p := range src.Torr.Pieces {
				if p.Status == src.BlockDownloaded {
					d++
				} else {
					x = true
				}
			}

			if x == false {
				break
			}

			downloaded = d
			pidx = 0
		}

		if sidx >= len(*sdrs) {
			sidx = 0
		}

		piece := src.Torr.Pieces[pidx]
		piecefree := piece.Status == src.BlockExist || piece.Status == src.BlockFailed

		// fmt.Println(pidx, sidx)
		if s := (*sdrs)[sidx]; s.IsFree() && s.HasPiece(pidx) && piecefree {

			go func(s *src.Peer, p *src.Piece) {
				fmt.Printf("requesting %v to %v:%v\n", p.Index, s.IP, s.Port)
				p.Status = src.BlockRequested

				_, err := s.DownloadPiece(p)
				if err != nil {
					p.Status = src.BlockFailed
					logrus.Errorf("%v\n", err)
					return
				}

				p.Status = src.BlockDownloaded
			}(s, piece)

			pidx++

		}

		sidx++

	}
}
