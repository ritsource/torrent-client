package src

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
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

/*
MaxMessageLength is the maximum p2p message length (happens to be a piece message)
if longer message recieved from a peer (ignore the message and disconnect the peer)
Msg-Length [4 bytes] + MsgID [1 byte] + Piece-Index [4 bytes] + Begin [4 bytes] + Length-of-Block [16384 bytes]
*/
var MaxMessageLength = 4 + 1 + 4 + 4 + LengthOfBlock

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

// Reset .
func (p *Peer) Reset() {
	p.State = PeerNone
	p.Bitfield = []bool{}
	p.UnChoked = false
	p.Waiting = false
	p.LastRequested = nil
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
		logrus.Warnf("couldn't establish TCP connection, %+v | %v:%v\n", err, p.IP, p.Port)
		return err
	}

	hsbuf, err := hsMsgBuf()
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v | %v:%v\n", err, p.IP, p.Port)
		return err
	}

	_, err = p.Conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v | %v:%v\n", err, p.IP, p.Port)
		return err
	}

	d := make([]byte, 1024)
	nr, err := p.Conn.Read(d)
	if err != nil {
		logrus.Warnf("couldn't to read handshake response, %v | %v:%v\n", err, p.IP, p.Port)
		p.Disconnect()
		return err
	}

	if !isHsMsg(d[:nr]) {
		logrus.Warnf("invalid handshake message, disconnecting.. | %v:%v\n", p.IP, p.Port)
		p.Disconnect()
	}

	logrus.Infof("handshake successfully established | %v:%v\n", p.IP, p.Port)
	p.State = PeerHandshaked

	go p.Read()

	i := 0
	for {
		time.Sleep(5 * time.Second)
		i++

		if p.IsReady() {
			return nil
		} else if !p.IsAlive() {
			logrus.Infof("peer has been disconnected | %v:%v\n", p.IP, p.Port)
			return fmt.Errorf("peer disconnected")
		} else if i >= 10 {
			logrus.Infof("peer handshake timeout, disconnecting.. | %v:%v\n", p.IP, p.Port)
			p.Disconnect()
			return fmt.Errorf("peer timeout")
		}
	}

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

// Read .
func (p *Peer) Read() {
	var length int
	msgbuf := new(bytes.Buffer)

	for p.IsAlive() {
		d := make([]byte, MaxMessageLength)
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
			continue
		}

		if msgbuf.Len() == 0 {
			length = int(binary.BigEndian.Uint32(b[:4]))
		}

		if length+4 > MaxMessageLength {
			logrus.Warnf("invalid message, msg-length = %v bytes, disconnecting.. | %v:%v\n", length, len(b), p.IP, p.Port)
			p.Disconnect()
			break
		}

		msgbuf.Write(d[:nr])

		if msgbuf.Len() == length+4 {
			p.handleMessages(msgbuf.Bytes(), length)
			msgbuf.Reset()
			length = 0
		}

	}
}

// handleMessages . identify
func (p *Peer) handleMessages(b []byte, length int) {
	id := b[4]

	var payload []byte
	if len(b) > 5 {
		payload = b[5:]
	}

	switch id {
	case uint8(0):
		logrus.Infof("CHOKE message, %v bytes | %v:%v\n", length, p.IP, p.Port)
		p.Disconnect()
	case uint8(1):
		logrus.Infof("UNCHOKE message, %v bytes | %v:%v\n", length, p.IP, p.Port)
		p.UnChoked = true
	case uint8(2):
		logrus.Infof("INTERESTED message, %v bytes | %v:%v\n", length, p.IP, p.Port)
	case uint8(3):
		logrus.Infof("NOT-INTERESTED message, %v bytes | %v:%v\n", length, p.IP, p.Port)
	case uint8(4):
		logrus.Infof("HAVE message, %v bytes | %v:%v\n", length, p.IP, p.Port)
	case uint8(5):
		logrus.Infof("BITFIELD message, %v bytes | %v:%v\n", length, p.IP, p.Port)
		p.HandleBitfield(payload)
	case uint8(6):
		logrus.Infof("REQUEST message, %v bytes | %v:%v\n", length, p.IP, p.Port)
	case uint8(7):
		logrus.Infof("PIECE message, %v bytes | %v:%v\n", length, p.IP, p.Port)
		err := p.HandleBlock(payload, int(length)-9)
		if err != nil {
			logrus.Errorf("lol - %v\n", err)
		}
	case uint8(8):
		logrus.Infof("CANCEL message, %v bytes | %v:%v\n", length, p.IP, p.Port)
	case uint8(9):
		logrus.Infof("PORT message, %v bytes | %v:%v\n", length, p.IP, p.Port)
	}

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
func (p *Peer) HandleBlock(payload []byte, blklen int) error {
	if len(payload) < 8+1 {
		return fmt.Errorf("invalid data, payload length is %v < 9", len(payload))
	}

	pieceidx := int(binary.BigEndian.Uint32(payload[:4])) // piece index
	begin := int(binary.BigEndian.Uint32(payload[4:8]))
	data := payload[8 : 8+blklen]

	fmt.Println("-----> data", len(data))

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

	logrus.Infof("PMSG (%v-bytes payload) - pidx=%v, bidx=%v, begin=%v, block=%v-bytes\n", len(payload), pieceidx, blkidx, begin, len(data))

	fname := Torr.Files[0].Path

	f, err := os.OpenFile(fname, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	// f, err := os.Open(fname)
	if err != nil {
		logrus.Errorf("%v\n", err)
	}
	defer f.Close()

	actbgn := pieceidx*int(Torr.PieceLen) + begin // actual file begin

	_, err = f.WriteAt(data, int64(actbgn))
	if err != nil {
		logrus.Errorf("%v\n", err)
	}

	return nil
}

var (
	// ErrDisconnected .
	ErrDisconnected = errors.New("peer connection has been closed")
)

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
		logrus.Warnf("boy! -- couldn't write request message, %v\n", err)
		p.Waiting = false
		p.LastRequested = nil
		blk.Status = BlockFailed
		Requested--

		if strings.Contains(err.Error(), "use of closed network connection") {
			fmt.Println("Holy-crap", err.Error(), "use of closed network connection")
			return ErrDisconnected
		}
	}

	fmt.Printf("HERE! \"%v\" %v:%v\n", err, p.IP, p.Port)

	return err
}
