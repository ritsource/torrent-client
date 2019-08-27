package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var rs = rand.New(rand.NewSource(time.Now().UnixNano()))

// PeerProtocolName .
var PeerProtocolName = []byte("BitTorrent protocol")

// Peers .
var Peers = []*Peer{}

// Status .
var Status = PeersStatus{}

// PeersStatus .
type PeersStatus struct {
	Total     int
	Connected int
}

/*Enum values for Peer state
PeerNone - Is the default state (at start)
PeerHandshaked - After handshake the state changes to PeerHandshaked
PeerBitfieldReady - After recieving bitfield message the state changes to PeerBitfieldReady
*/
const (
	PeerNone          = int8(0) // default/zero state
	PeerHandshaked    = int8(1)
	PeerBitfieldReady = int8(2)
)

/*
Peer represents a single peer
*/
type Peer struct {
	IP       net.IP
	Port     uint16
	Conn     net.Conn
	Bitfield []bool
	UnChoked bool
	State    int8
}

// Ping establishes a tcp connection with the peer and handshake request and reads response too to the peer and starts reqding
func (p *Peer) Ping(wg *sync.WaitGroup) {
	// defer wg.Done()

	addr := p.IP.String() + ":" + strconv.Itoa(int(p.Port))

	var err error
	p.Conn, err = net.Dial("tcp", addr)
	if err != nil {
		logrus.Errorf("%+v\n", err)
		return
	}

	hsbuf, err := hsMsgBuf()
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v\n", err)
		return
	}

	_, err = p.Conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v\n", err)
		return
	}

	err = p.readHsMsg(10)
	if err != nil {
		logrus.Warnf("unable to read handshake response, %v\n", err)
		p.Conn.Close()
		return
	}

	// defer p.Conn.Close()

	p.State = PeerHandshaked

	go p.Read()

	// checking for timeout
	go func(p *Peer) {
		time.Sleep(40 * time.Second)
		if p.State == PeerNone && p.Conn != nil {
			logrus.Warnf("closing peer connection: response read timeout from %v", p.Conn.RemoteAddr())
			p.Disconnect()
		}
	}(p)

	Status.Total++
	if Status.Total%3 == 0 {
		Status.Connected++
	}

	// fmt.Printf("peer: %+v\n", p.IP)
}

// Disconnect .
func (p *Peer) Disconnect() {
	p.State = PeerNone
	p.UnChoked = false
	if p.Conn != nil {
		p.Conn.Close()
	}
}

// readHsMsg .
func (p *Peer) readHsMsg(lim int) error {
	if lim == 0 {
		return fmt.Errorf("message read timeout")
	}

	if lim < 10 {
		fmt.Printf("\n\n\n\n\n\n*******haha - it was a good option*******\n\n\n\n\n\n")
	}

	d := make([]byte, 1024)
	nr, err := p.Conn.Read(d)
	if err != nil {
		logrus.Warnf("%+v\n", err)
		return err
	}

	if isHsMsg(d[:nr]) {
		logrus.Infof("message recieved: handshake - %v\n", p.Conn.RemoteAddr())
		return nil
	}

	return p.readHsMsg(lim - 1)
}

func (p *Peer) Read() {
	i := 0
	msgbuf := new(bytes.Buffer)
	var explen int

	for {
		if p.Conn == nil {
			break
		}

		d := make([]byte, 1024)
		nr, err := p.Conn.Read(d)
		if err != nil {
			logrus.Errorf("%v\n", err)
			break
		}

		b := d[:nr]

		if nr < 4+1 {
			// invalid
			// logrus.Warnf("message recieved: invalid - %v - msg-length () < 4+1", conn.RemoteAddr())
			continue
		}

		if msgbuf.Len() == 0 {
			explen = int(binary.BigEndian.Uint32(b[:4])) + 4
		}

		msgbuf.Write(d[:nr])

		if msgbuf.Len() >= 4 && msgbuf.Len() >= explen {
			// Handle-Msg
			p.handleMessages(msgbuf.Bytes())
			msgbuf.Reset()
			i++
		}

	}

}

// handleMessages . identify
func (p *Peer) handleMessages(b []byte) {

	if len(b) < 4+1 {
		logrus.Warnf("invalid message: %v bytes from %v", len(b), p.Conn.RemoteAddr())
		return
	}
	length := binary.BigEndian.Uint32(b[:4])

	if len(b) < int(length)+4 || length == 0 {
		logrus.Warnf("invalid message: %v bytes from %v", len(b), p.Conn.RemoteAddr())
		return
	}
	id := b[4]

	var payload []byte
	if len(b) > 5 {
		payload = b[5:]
	}

	logrus.Infof("message received: %v bytes from %v", len(b), p.Conn.RemoteAddr())

	switch id {
	case uint8(0):
		logrus.Infof("choke\n")
		p.Disconnect()
	case uint8(1):
		logrus.Infof("unchoke\n")
		p.UnChoked = true
	case uint8(2):
		logrus.Infof("interested\n")
	case uint8(3):
		logrus.Infof("not interested\n")
	case uint8(4):
		logrus.Infof("have\n")
	case uint8(5):
		logrus.Infof("-> bitfield\n")
		p.HandleBitfield(payload)
	case uint8(6):
		logrus.Infof("request\n")
	case uint8(7):
		logrus.Infof("piece\n")
		// p.handlePiece(payload)
	case uint8(8):
		logrus.Infof("cancel\n")
	case uint8(9):
		logrus.Infof("port\n")
	}

}

func (p *Peer) handleMsg(buf *bytes.Buffer) {
	b := buf.Bytes()

	if isHsMsg(b) {
		logrus.Infof("message recieved: handshake - %v\n", p.Conn.RemoteAddr())
		return
	}

	// if len(b) < 4+1 {
	// 	logrus.Errorf("message recieved: invalid - %v - msg-length < 4+1", conn.RemoteAddr())
	// 	return
	// }

	logrus.Infof("message recieved: %v - %v\n", len(b), p.Conn.RemoteAddr())

}

// handshakeBuf builds a handshake message buffer
func hsMsgBuf() (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, uint8(len(PeerProtocolName)))
	err = binary.Write(buf, binary.BigEndian, PeerProtocolName)
	err = binary.Write(buf, binary.BigEndian, uint64(0))
	err = binary.Write(buf, binary.BigEndian, Torr.InfoHash)
	err = binary.Write(buf, binary.BigEndian, []byte(PeerID))

	return buf, err
}

func isHsMsg(b []byte) bool {
	return len(b) >= 20 && reflect.DeepEqual(b[1:20], PeerProtocolName)
}

// HandleBitfield ..
func (p *Peer) HandleBitfield(payload []byte) {
	if len(Torr.Pieces) != len(payload)*8 {
		logrus.Warnf("piece length mismatch in bitfield message")
		return
	}

	p.Bitfield = make([]bool, len(payload)*8)

	for i, b := range payload {
		for j := 0; j < 8; j++ {
			// pushing bool
			p.Bitfield[i*8+j] = int8(b>>uint(7-j)&0x01) == 1
		}
	}

	p.State = PeerBitfieldReady

}

// RequestPiece .
func RequestPiece() {}
