package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/ritwik310/torrent-client/src"
)

var transactionID uint32

func init() {
	transactionID = uint32(time.Now().Unix()) // WARNING: not sure if a good idea
}

func main() {
	// reading command line arguements for torrent file path
	if len(os.Args) < 2 {
		fmt.Println("no torrent file provided")
		return
	}
	fn := os.Args[1] // path to the torrent file

	torr := src.Torr{}       // represents torrent metadata
	err := torr.ReadFile(fn) // populating torr by reading values from file
	if err != nil {
		panic(err)
	}

	// tracker
	tracker := src.NewTracker(&torr)

	// tracker.Torr.ReadPieces()
	// return

	// parsing announce url of tracker, could be udp or http
	ann, err := url.Parse((*tracker.Torr).Data["announce"].(string))
	if err != nil {
		fmt.Println("unable to parse announce url")
		panic(err)
	}

	// check protocol
	switch ann.Scheme {
	case "udp":
		// sending connection request to UDP server (the announce host) and reading responses
		tID, connID, err := tracker.ConnUDP(ann.Host, transactionID)
		if err != nil {
			panic(err)
		}
		if tID != transactionID {
			panic(fmt.Sprintf("transaction_id is the request and response did not match %v != %v \n", transactionID, tID))
		}

		// once connection request is successfule, sending announce request
		// this will mainly get us a list of seeders for that torrent files
		interval, err := tracker.GetPeersUDP(ann.Host, tID, connID)
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
		fmt.Printf("unsupported announce protocol, %v\n", ann.Scheme)
	}

	// return

	fmt.Printf("%+v\n", tracker.Peers)

	for i := 0; i < len(tracker.Peers); i++ {
		p := tracker.Peers[i]
		go p.HandleMessaging(tracker.Torr)
	}

	for {
	}

	// for {
	// }

}

// func handlePeer() {

// }

func handlePeerTemp(peer src.Peer, torr *src.Torr) {
	// err := peer.Handshake(torr)
	// if err != nil {
	// 	panic(err)
	// }

	conn, err := net.Dial("tcp", peer.IP.String()+":"+strconv.Itoa(int(peer.Port)))
	if err != nil {
		panic(err)
	}

	peer.KeepAlive(conn)

}
