package main

import (
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

	ch1 := make(chan error)
	var peers []*src.Peer

	go func(prs *[]*src.Peer, c chan error) {
		*prs, err = src.GetPeers()
		c <- err
	}(&peers, ch1)

	for _, piece := range src.Torr.Pieces {
		piece.GenBlocks()
	}

	src.Torr.GenPFMap()

	// for pidx, fs := range src.Torr.PFMap {
	// 	fmt.Printf("%v | ", pidx)
	// 	for _, f := range fs {
	// 		fmt.Printf("\t%v", f)
	// 	}
	// 	fmt.Printf("\n")
	// }
	// return

	err = <-ch1
	if err != nil {
		logrus.Panicf("%v\n", err)
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

	for {
		time.Sleep(100 * time.Millisecond)

		if pidx >= len(src.Torr.Pieces) {
			dn, comp := isDownComplete(src.Torr.Pieces)
			if comp == true {
				logrus.Infof("all [%v] pieces has been downloaded **[DONE]**", dn)
				break
			}

			logrus.Infof("[%v] out of [%v] pieces has been downloaded", dn, len(src.Torr.Pieces))
			pidx = 0
		}

		if sidx >= len(*sdrs) {
			sidx = 0
		}

		piece := src.Torr.Pieces[pidx]

		// p2breq := piece.Status == src.PieceStatusDefault || piece.Status == src.PieceStatusFailed

		if piece.Status == src.PieceStatusDownloaded || piece.Status == src.PieceStatusRequested {
			pidx++
			continue
		}

		seeder := (*sdrs)[sidx]

		if seeder.IsFree() && seeder.HasPiece(pidx) {
			logrus.Infof("requesting piece of index %v | to %v:%v\n", piece.Index, seeder.IP, seeder.Port)
			go GetPiece(seeder, piece)

			pidx++

		} else if !seeder.IsAlive() {
			go seeder.Ping()
		}

		sidx++

	}
}

// isDownComplete .
func isDownComplete(pieces []*src.Piece) (int, bool) {
	boo := true // boo == true, all pieces are downloaded (none left)
	dn := 0     // number of pieces to be downloaded
	for _, p := range pieces {
		if p.Status == src.PieceStatusDownloaded {
			dn++
		} else {
			// if any piece is yet to be downloaded, set `boo = false`
			boo = false
		}
	}

	return dn, boo
}

// GetPiece .
func GetPiece(s *src.Peer, p *src.Piece) {
	_, err := s.DownloadPiece(p)
	switch err {
	case nil:
		// pass
	case src.ErrPeerDisconnected:
		go s.Ping()
	default:
		logrus.Errorf("%v\n", err)
	}
}
