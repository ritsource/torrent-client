package main

import (
	"fmt"
	"os"
	"sync"
	"time"

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
	torr, err := NewTorrent(fn)
	if err != nil {
		logrus.Panicf("%v\n", err)
	}

	// new tracker
	tracker := NewTracker(torr)

	// check protocol
	switch torr.AnnounceURL.Scheme {
	case "udp":
		// sending connection request to UDP server (the announce host) and reading responses
		tID, connID, err := tracker.ConnUDP(torr.AnnounceURL.Host, TransactionID)
		if err != nil {
			panic(err)
		}
		if tID != TransactionID {
			panic(fmt.Sprintf("transaction_id is the request and response did not match %v != %v \n", TransactionID, tID))
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

	// return

	var activePeers []*Peer

	fmt.Printf("Number of peers: %v\n", len(tracker.Peers))

	for i := 0; i < len(tracker.Peers); i++ {
		p := tracker.Peers[i]

		ch := make(chan *Peer)
		go p.GetPieces(tracker.Torrent, ch)

		go func(c chan *Peer) {
			activePeers = append(activePeers, <-ch)
		}(ch)
	}

	go func() {
		for {
			time.Sleep(10 * time.Second)
			for _, v := range activePeers {
				fmt.Printf("Peer -> %v / %v\n", v.IP, v.Port)
			}
		}
	}()

	// return

	// go func() {
	// 	time.Sleep(20 * time.Second)
	// 	os.Exit(0)
	// }()

	fmt.Println("Starting Download....")

	pieceidx := 0
	peeridx := 0
	piececoverage := 0

	var wg sync.WaitGroup

	// go func(w *sync.WaitGroup) {
	// 	fmt.Printf("%v\n", w.)
	// }(&wg)

	i := 0

	for {
		time.Sleep(time.Millisecond * 400)
		fmt.Println("---------------------------------------------------------------->", i)
		i++

		if piececoverage >= len(tracker.Peers) {
			pieceidx++
		}

		if pieceidx == len(torr.Pieces) {
			// wg.Wait()
			pieceidx = 0
		}

		if peeridx == len(tracker.Peers) {
			peeridx = 0
		}

		piece := torr.Pieces[pieceidx]
		peer := tracker.Peers[peeridx]

		// fmt.Println("sharing ------------> ", peeridx, peer.Sharing)
		if !peer.Sharing {
			peeridx++
			continue
		}

		if piece.Status == PieceNotFound && peer.Pieces[pieceidx] == 1 {

			// if peer.Pieces[pieceidx] == 1 {
			// 	fmt.Println("AAAAAAAAAAAAAAAAAAAAAAAAAAAF")
			// } else {
			// 	fmt.Println("SSSSSSSSSSSSSSSSSSSSSSSSSSSF")
			// }

			fmt.Println("{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{------------>", pieceidx)

			wg.Add(1)
			peer.Download(piece, &wg)
			pieceidx++
			piececoverage = 0
			peeridx++
		} else {
			piececoverage++
			peeridx++
		}

	}

}
