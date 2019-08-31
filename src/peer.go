package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/ritwik310/torrent-client-xx/torrent"
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
	PeerDisconnected  = int8(3)
)

/*
Peer represents a single peer
*/
type Peer struct {
	IP            net.IP
	Port          uint16
	Conn          net.Conn
	Bitfield      []bool
	UnChoked      bool
	State         int8
	Waiting       bool
	LastRequested *Block
}

// IsReady .
func (p *Peer) IsReady() bool {
	return p.UnChoked && len(p.Bitfield) > 0 && p.State < PeerDisconnected
}

// IsAlive .
func (p *Peer) IsAlive() bool {
	return !(p.Conn == nil || p.State == PeerDisconnected)
}

// Disconnect .
func (p *Peer) Disconnect() {
	p.State = PeerDisconnected
	p.UnChoked = false
	if p.Conn != nil {
		p.Conn.Close()
	}
}

// Ping establishes a tcp connection with the peer and handshake request and reads response too to the peer and starts reqding
func (p *Peer) Ping() error {
	addr := p.IP.String() + ":" + strconv.Itoa(int(p.Port))

	var err error
	p.Conn, err = net.Dial("tcp", addr)
	if err != nil {
		logrus.Errorf("%+v\n", err)
		return err
	}

	hsbuf, err := hsMsgBuf()
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v\n", err)
		return err
	}

	_, err = p.Conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v\n", err)
		return err
	}

	err = p.readHsMsg(10)
	if err != nil {
		logrus.Warnf("unable to read handshake response, %v\n", err)
		p.Conn.Close()
		return err
	}

	// defer p.Conn.Close()

	p.State = PeerHandshaked

	go p.Read()

	i := 0
	for {
		time.Sleep(5 * time.Second)
		i++

		if p.IsReady() {
			return nil
		} else if !p.IsAlive() {
			return fmt.Errorf("peer disconnected")
		} else if i >= 10 {
			p.Disconnect()
			return fmt.Errorf("peer timeout")
		}
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

	for p.IsAlive() {
		d := make([]byte, 1024)
		nr, err := p.Conn.Read(d)
		switch err {
		case nil:
			// pass
		case io.EOF:
			p.Disconnect()
			break
		default:
			logrus.Warnf("%v\n", err)
			break
		}

		b := d[:nr]

		if nr < 4+1 {
			// invalid
			// logrus.Warnf("message recieved: invalid - %v - msg-length () < 4+1", conn.RemoteAddr())
			continue
		}

		// if msgbuf.Len() >  {

		// }

		if msgbuf.Len() == 0 {
			explen = int(binary.BigEndian.Uint32(b[:4])) + 4
			// fmt.Printf("%v - %v__ explen: %v, nr: %v\n", p.Conn.RemoteAddr(), i, explen, nr)
		}

		if explen > LengthOfBlock+500 {
			fmt.Printf("x Disconnect - %v _____________ Explen: %v Block: %v\n", p.Conn.RemoteAddr(), explen, LengthOfBlock)
			p.Disconnect()
			break
		}

		msgbuf.Write(d[:nr])

		if msgbuf.Len() >= 4 && msgbuf.Len() >= explen {
			// Handle-Msg
			// fmt.Printf("%v - %v__ Final", p.Conn.RemoteAddr(), i)
			// fmt.Printf("Fuck!!!!!\n %v\n", msgbuf.Bytes()[explen:msgbuf.Len()])
			// p.handleMessages(msgbuf.Bytes())
			p.handleMessages(msgbuf.Bytes())
			msgbuf.Reset()
			explen = 0
		}

		i++
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
		// logrus.Infof("%v - length: %v\n", p.Conn.RemoteAddr(), length)
		err := p.HandleBlock(payload)
		if err != nil {
			logrus.Errorf("lol - %v\n", err)
		}
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
	// p.Bitfield = make([]bool, len(Torr.Pieces))

	for i, b := range payload {
		for j := 0; j < 8; j++ {
			// pushing bool
			p.Bitfield[i*8+j] = int8(b>>uint(7-j)&0x01) == 1
		}
	}

	p.State = PeerBitfieldReady

}

var Requested = 0
var Recieved = 0

// HandleBlock .
func (p *Peer) HandleBlock(payload []byte) error {
	if len(payload) < 8+1 {
		return fmt.Errorf("invalid data, payload length is %v < 9", len(payload))
	}

	pieceidx := int(binary.BigEndian.Uint32(payload[:4])) // piece index
	begin := int(binary.BigEndian.Uint32(payload[4:8]))
	data := payload[8:]

	blkidx := begin / torrent.BlockLength

	if pieceidx >= len(Torr.Pieces) || blkidx >= len(Torr.Pieces[pieceidx].Blocks) {
		return fmt.Errorf("lol error 1")
	}

	block := Torr.Pieces[pieceidx].Blocks[blkidx]

	Recieved++

	if !p.Waiting || p.LastRequested != block {
		fmt.Printf("p.Wait: %v, p.LastReq %v, block:  %v\n", p.Waiting, p.LastRequested, block)
		return fmt.Errorf("lol error 2")
	}

	p.Waiting = false
	p.LastRequested = nil
	block.Status = BlockDownloaded

	logrus.Infof("Piece Index - %v\nBlock Index - %v\nBegin - %v\nData - bytes %v\n", pieceidx, blkidx, begin, len(data))

	return nil
}

// RequestBlock .
func (p *Peer) RequestBlock(blk *Block) error {
	p.Waiting = true
	p.LastRequested = blk
	blk.Status = BlockRequested

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(13))
	binary.Write(buf, binary.BigEndian, uint8(6))       // id - request message
	binary.Write(buf, binary.BigEndian, blk.PieceIndex) // piece index
	binary.Write(buf, binary.BigEndian, blk.Begin)      //
	binary.Write(buf, binary.BigEndian, blk.Length)

	Requested++

	_, err := p.Conn.Write(buf.Bytes())
	if err != nil {
		// logrus.Warnf("couldn't write request message, %v\n", err)
		p.Waiting = false
		p.LastRequested = nil
		blk.Status = BlockFailed
		Requested--
		return err
	}

	return nil
}
