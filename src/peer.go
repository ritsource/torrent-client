package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var rs = rand.New(rand.NewSource(time.Now().UnixNano()))

// PeerProtocolNameV1 .
var PeerProtocolNameV1 = []byte("BitTorrent protocol")

// Peers .
var Peers = []*Peer{}

// Status .
var Status = PeersStatus{}

// PeersStatus .
type PeersStatus struct {
	Total     int
	Connected int
}

/*
Peer represents a single peer
*/
type Peer struct {
	IP       net.IP
	Port     uint16
	Bitfield []bool
	Unchoked bool
}

// Ping .
func (p *Peer) Ping(wg *sync.WaitGroup) {
	// defer wg.Done()

	addr := p.IP.String() + ":" + strconv.Itoa(int(p.Port))

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		logrus.Errorf("%+v\n", err)
		return
	}

	go p.Read(conn)

	hsbuf, err := handshakeBuf()
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v\n", err)
		return
	}

	_, err = conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v\n", err)
		return
	}

	Status.Total++
	if Status.Total%3 == 0 {
		Status.Connected++
	}

	fmt.Printf("peer: %+v\n", p.IP)

}

func (p *Peer) Read(conn net.Conn) {
	for conn != nil {
		time.Sleep(time.Duration(rs.Intn(1000)) * time.Microsecond)

		fmt.Printf("peer: %+v\n", p.IP)

		d := make([]byte, 1024)
		nr, err := conn.Read(d)
		if err != nil {
			logrus.Errorf("%v\n", err)
		}

		d = d[:nr]
		fmt.Printf("message: %+v bytes\n", len(d))

	}
}

// handshakeBuf builds a handshake message buffer
func handshakeBuf() (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, uint8(len(PeerProtocolNameV1)))
	err = binary.Write(buf, binary.BigEndian, PeerProtocolNameV1)
	err = binary.Write(buf, binary.BigEndian, uint64(0))
	err = binary.Write(buf, binary.BigEndian, Torr.InfoHash)
	err = binary.Write(buf, binary.BigEndian, []byte(PeerID))

	return buf, err
}
