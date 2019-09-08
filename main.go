package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ritwik310/torrent-client/output"
	"github.com/ritwik310/torrent-client/src"
)

var torrFn string

func init() {
	// reading teh command-line flags
	devflag := flag.Bool("dev", false, "to print developer logs or not") // to determine in dev-mode or not
	flflag := flag.String("file", "", "path to the `.torrent` file")     // `.torrent file path`

	flag.Parse()

	torrFn = *flflag
	output.DevMode = *devflag
}

// variables to store data about download stats .
var (
	TotalPieceCount      int
	DownloadedPieceCount int
	TotalDataSize        int
	DownloadedDataSize   int
	DownloadStarted      bool
)

func main() {
	// if no `--file` value provided reading the `.torrent`
	// file path as the 2nd command-line arguements
	if torrFn == "" {
		if len(os.Args) < 2 {
			panic("no `.torrent` file provided")
		}
		torrFn = os.Args[1]
	}

	// print stats (different goroutine)
	iv := true
	go PrintStats(&iv)

	// reading the `.torrent` file
	err := src.ReadFile(torrFn)
	if err != nil {
		panic(fmt.Errorf("unable to read data from `.torrent` file, %v", err))
	}

	// set total piece count to what read on the `.torrent` file,
	TotalPieceCount = len(src.Torr.Pieces)

	ch1 := make(chan error) // holds tracker request error, when requested in a different goroutine
	var peers []*src.Peer   // holds pointers to `src.Peer` corresponding to each peer

	// retrieving information about peers from tracker server
	go func(prs *[]*src.Peer, c chan error) {
		*prs, err = src.GetPeers()
		c <- err
	}(&peers, ch1)

	// generating `src.Block` for each peers
	for _, piece := range src.Torr.Pieces {
		piece.GenBlocks()
	}

	// generating PFMap, mapping of piece and files, that determines
	// in which file/files teh piece data needs to be written
	src.Torr.GenPFMap()

	// handling tracker request response (if error)
	err = <-ch1
	if err != nil {
		panic(err)
	}

	// seeders holds the pointer to the peers from which data can be downloaded
	seeders := AllSeeders{}
	seeders.Find(peers)

	// wait as long as there's no peeris ready/available to share files
	for seeders.Len() < 1 {
		time.Sleep(1 * time.Second)
	}

	// DownloadStarted indicates file download has started or not
	DownloadStarted = true

	// `AllSeeders.Download` efficiently downloads pieces
	// of data from all the seeders concurrently
	seeders.Download(src.Torr)

	fmt.Println("\nDownload Complete!")

}

// PrintStats peints and updates stats about download process
// it requires a boolean as arguemnt for not so necessary reasons
func PrintStats(iv *bool) {
	ticker := time.Tick(time.Second)
	for {
		<-ticker
		perc := float64(DownloadedPieceCount) / float64(TotalPieceCount) * 100
		var status string
		if DownloadStarted {
			status = "Downloading.."
		} else {
			status = "Getting info.."
		}

		if *iv == true {
			*iv = false
			status += "."
		} else {
			status += " "
			*iv = true
		}

		fmt.Printf("\rDownloaded %.2f%%\tPieces %d/%d\t%v\t", float64(perc), DownloadedPieceCount, TotalPieceCount, status)
	}
}

// AllSeeders is a slice of pointers to peers from which pieces
// of data can be downloaded (peers that has unchoked the client)
type AllSeeders []*src.Peer

// Len returns the length of `Seeders`
func (as *AllSeeders) Len() int {
	return len(*as)
}

// Active checks and returns the number of
// seeders where peer connection is alive
func (as *AllSeeders) Active() int {
	x := 0
	for _, s := range *as {
		if s.IsAlive() {
			x++
		}
	}
	return x
}

/*
Find is gonna disconnect with all the peers
and reestablish connection with all of them again
*/
func (as *AllSeeders) Find(peers []*src.Peer) {
	for _, p := range peers {
		go func(p *src.Peer) {
			err := p.Ping()
			if err == nil {
				(*as) = append(*as, p)
			}
		}(p)
	}
}

// Download downloads each of `src.Torr.Pieces` from the seeder
func (as *AllSeeders) Download(torr *src.Torrent) {
	var done bool

	go func(done *bool) {
		for {
			time.Sleep(1 * time.Second)

			// if all the pieces are downloaded then break the loop
			dn, comp := isDownComplete(torr.Pieces)

			// print how many pieces has been downloaded yet, and reset `pidx = 0`
			output.DevInfof("[%v] out of [%v] pieces has been downloaded", dn, len(torr.Pieces))
			DownloadedPieceCount = dn

			if comp == true {
				output.DevInfof("all [%v] pieces has been downloaded **[DONE]**", dn)
				*done = true
				break
			}
		}
	}(&done)

	pidx := 0 // current piece index
	sidx := 0 // current seeder/peer index

	for !done {
		time.Sleep(100 * time.Millisecond)

		// when all the pieces are covered, reseting the `pidx = 0`
		if pidx >= len(torr.Pieces) {
			pidx = 0
		}

		// if all the seeder's are covered onc more time, reset `sidx = 0`
		if sidx >= len(*as) {
			sidx = 0
		}

		// pointer to the piece that has to be downloaded
		piece := src.Torr.Pieces[pidx]

		// if piece is currently downloading or has alreay been downloaded, then go to the next piece
		if piece.Status == src.PieceStatusDownloaded || piece.Status == src.PieceStatusRequested {
			pidx++
			continue
		}

		// pointer to the peer
		seeder := (*as)[sidx]

		// if peer is available for download and has the piece, then request the piece from it
		if seeder.IsFree() && seeder.HasPiece(pidx) {
			output.DevInfof("requesting piece of index %v | to %v:%v\n", piece.Index, seeder.IP, seeder.Port)

			// downloading teh piece in a different goroutine
			go func(s *src.Peer, p *src.Piece) {
				_, err := s.DownloadPiece(p)
				switch err {
				case nil:
					// pass
				case src.ErrPeerDisconnected:
					go s.Ping()
				default:
					output.DevErrorf("%v\n", err)
				}
			}(seeder, piece)

			// to go to the next-piece
			pidx++

		} else if !seeder.IsAlive() {
			// if peer connection is closed, then reestablish the connection
			go seeder.Ping()
		}

		// next-seeder
		sidx++
	}
}

// isDownComplete checks if all the pieces are downloaded
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
