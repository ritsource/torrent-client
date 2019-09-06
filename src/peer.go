package src

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// PeerProtocolName is version 1 peer protocol name, required in message construction
var PeerProtocolName = []byte("BitTorrent protocol")

/*
MaxMessageLength is the maximum p2p message length (happens to be a piece message)
if longer message recieved from a peer (ignore the message and disconnect the peer)
Msg-Length [4 bytes] + MsgID [1 byte] + Piece-Index [4 bytes] + Begin [4 bytes] + Length-of-Block [16384 bytes]
*/
var MaxMessageLength = 4 + 1 + 4 + 4 + LengthOfBlock

/*
Peer represents a single peer
*/
type Peer struct {
	IP          net.IP
	Port        uint16
	Conn        net.Conn
	Bitfield    []bool
	UnChoked    bool
	Connected   bool
	Downloading bool
}

/*
IsReady returns a boolean that indicates if the peer is ready to
request pieces or not. If the peer has been unchoked and holds a
valid bitfield it returns `true`, else it returns `false`
*/
func (p *Peer) IsReady() bool {
	return p.UnChoked && len(p.Bitfield) == len(Torr.Pieces) && p.Connected
}

/*
IsAlive checks if a peer connection has been disconnected or not.
If disconnected then it returns `false`, if not then returns `true`
*/
func (p *Peer) IsAlive() bool {
	return p.Conn != nil && p.Connected
}

// IsFree .
func (p *Peer) IsFree() bool {
	return !p.Downloading
}

// HasPiece .
func (p *Peer) HasPiece(pidx int) bool {
	return p.IsReady() && len(p.Bitfield) >= pidx && p.Bitfield[pidx]
}

/*
Disconnect closes the peer connection (TCP) and sets it's state to
`Unchoked` and `Disconnected`, though it doesn't resets the `Bitfield`
*/
func (p *Peer) Disconnect() {
	p.Connected = false
	if p.Conn != nil {
		p.Conn.Close()
	}
}

/*
Reset not only closes the peer connection and changes the state
to `Unchoked`, but it also resets the bitfield (to `[]bool{}`)
*/
func (p *Peer) Reset() {
	p.Disconnect()
	p.Bitfield = []bool{}
}

/*
Ping establishes a TCP connection with the peer and exchanges messages
to find what pieces the peer has, and if we are allowed to download
pieces of data from the peer. In detail, it exchange handshake messages
and waits for the peer to send `bitfield` and `unchoke` messages
(message read timeout 50 seconds)
*/
func (p *Peer) Ping() error {
	// if no response from the peer for 50 seconds disconnect the peer
	go func(p *Peer) {
		time.Sleep(50 * time.Second)
		if p.IsAlive() && !p.IsReady() {
			logrus.Infof("peer ping timeout, disconnecting.. | %v:%v\n", p.IP, p.Port)
			p.Disconnect()
		}
	}(p)

	// peer server address
	addr := p.IP.String() + ":" + strconv.Itoa(int(p.Port))

	// establishing a TCP connection with the peer
	var err error
	p.Conn, err = net.Dial("tcp", addr)
	if err != nil {
		logrus.Warnf("couldn't establish TCP connection, %+v | %v:%v\n", err, p.IP, p.Port)
		return err
	}
	p.Connected = true

	// building the handshake message buffer
	hsbuf, err := handshakeMsgBuf()
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v | %v:%v\n", err, p.IP, p.Port)
		return err
	}

	// writing the handshake message on peer connection
	_, err = p.Conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v | %v:%v\n", err, p.IP, p.Port)
		return err
	}

	// waiting for the peer to respond with a handshake message
	d := make([]byte, 1024)
	nr, err := p.Conn.Read(d)
	if err != nil {
		logrus.Warnf("couldn't to read handshake response, %v | %v:%v\n", err, p.IP, p.Port)
		p.Disconnect()
		return err
	}

	// checkign if the recieved messages is a valid handshake message or not
	if !isHsMsg(d[:nr]) {
		logrus.Warnf("invalid handshake message, disconnecting.. | %v:%v\n", p.IP, p.Port)
		p.Disconnect()
	}

	logrus.Infof("handshake-message, %v bytes | %v:%v\n", nr, p.IP, p.Port)

	// now, reading from the connection, waitign for peer to
	// write `bitfield` and `unchoke` message
	for p.IsAlive() && !p.IsReady() {
		// read reads messages sent over multiple writes and
		// concatinates and returns it as a single message
		msg, err := p.Read()
		if err != nil {
			logrus.Warnf("%v, disconnecting.. | %v:%v\n", err, p.IP, p.Port)
			p.Disconnect()
			return err
		}

		// extracting length, message-id and payload from the message
		lng, id, payld, err := extractMsg(msg)
		if err != nil {
			logrus.Warnf("%v, disconnecting.. | %v:%v\n", err, p.IP, p.Port)
			p.Disconnect()
			return err
		}

		// message id indicates the message type, i==0 -> "choke"; i==1 -> "unchoke" etc..
		// expecting the peer to write `bitfield` and `unchoke` message
		switch id {
		case uint8(0):
			logrus.Infof("choke-message, %v bytes | %v:%v\n", lng+4, p.IP, p.Port)
			p.Disconnect()
		case uint8(1):
			logrus.Infof("unchoke-message, %v bytes | %v:%v\n", lng+4, p.IP, p.Port)
			p.UnChoked = true
		case uint8(4):
			logrus.Infof("have-message, %v bytes | %v:%v\n", lng+4, p.IP, p.Port)
		case uint8(5):
			logrus.Infof("bitfield-message, %v bytes | %v:%v\n", lng+4, p.IP, p.Port)
			err := p.ReadBitfield(payld)
			if err != nil {
				logrus.Warnf("bitfield read error, %v, disconnecting.. | %v:%v\n", err, p.IP, p.Port)
				p.Disconnect()
				return err
			}
		}
	}

	return nil
}

/*
Read reads messages from peer connection. It reads the messages-length at
start and concatenates multiple writes and treats it as a whole message
*/
func (p *Peer) Read() ([]byte, error) {
	var lng int              // `lng` holds value of message-length (read from first 4 bytes)
	buf := new(bytes.Buffer) // `buf` holds message data over multiple writes
	defer buf.Reset()

	// as long as the connection is alive, read from the connection
	// and when a message is complete break continious read
	for p.IsAlive() {
		b := make([]byte, MaxMessageLength)
		nr, err := p.Conn.Read(b)
		switch err {
		case nil:
			// pass
		case io.EOF:
			return nil, fmt.Errorf("message read err, %v", err)
		default:
			logrus.Warnf("%v | %v:%v\n", err, p.IP, p.Port)
			return nil, err
		}

		// when the `buf` length is 0 (first read), reading total
		// message length encoded in first four bytes (uint32)
		if buf.Len() == 0 {
			lng = int(binary.BigEndian.Uint32(b[:4]))
		}

		// if message length excides teh maximum allowed message
		// length, disconnecting the peer and trowning an error
		if lng+4 > MaxMessageLength {
			return nil, fmt.Errorf("invalid message, msg-length = %v bytes", lng+4)
		}

		// saving corrent read to `buf`
		buf.Write(b[:nr])

		// when the message read is complete (buffer-length
		// == msg-length + first-4-bytes), break the read
		if buf.Len() >= lng+4 {
			break
		}
	}

	// return read message
	return buf.Bytes(), nil
}

/*
ReadBitfield reads a bitfield message payload and populates the
`peer.Bitfield` ([]bool) with booleans that indicates if a piece
of that index is available on the peer to be requested
*/
func (p *Peer) ReadBitfield(payld []byte) error {
	if len(payld)*8 != len(Torr.Pieces) {
		return fmt.Errorf("bitfield length (%v) != number of pieces (%v)", len(payld)*8, len(Torr.Pieces))
	}

	// the booleans directly cannot be appended to `peer.Bitfield` as
	// the `peer.IsReady()` method checks for len(peer.Bitfield) to be
	// requal to len(Torr.Pieces) is a concurrent goroutine
	bf := make([]bool, len(payld)*8)

	for i, b := range payld {
		for j := 0; j < 8; j++ {
			bf[i*8+j] = int8(b>>uint(7-j)&0x01) == 1 // pushing bool
		}
	}

	p.Bitfield = bf

	return nil
}

// ErrPeerDisconnected .
var ErrPeerDisconnected = errors.New("peer connection has been closed")

/*
DownloadPiece downloads all the `Block`s of data of a given `Piece` from the `Peer`,
and writes the data to the appropriate files.

If `Peer` gets disconnected then the method returns a `ErrPeerDisconnected` error,
so that the client can reestablish connection with the `Peer`
*/
func (p *Peer) DownloadPiece(piece *Piece) (int, error) {
	bidx := 0                              // index of the block to be requested
	downs := make([]byte, 0, piece.Length) // holds all the downloaded data

	// errcnt counts the number download error for a single block of data,
	// so if it excides teh limit the method can throw an error
	errcnt := 0

	// managing the states of `Peer` and `Piece` over the course of download
	piece.Status = PieceStatusRequested
	p.Downloading = true
	defer func(p *Peer) {
		// this method `peer.DownloadPiece()` doesn't set the `piece.Status` value
		// to `PieceStatusDownloaded`, it's to be down after the file write, so by
		// the `piece.WriteToFiles()` method. So, `peer.DownloadPiece()` only checks
		// that `piece.Status` is equal to `PieceStatusDownloaded` or not, if not
		// then it sets the value of `piece.Status` to `PieceStatusFailed`
		if piece.Status != PieceStatusDownloaded {
			piece.Status = PieceStatusFailed
		}
		p.Downloading = false
	}(p)

	for {
		if !p.IsAlive() {
			// returning `ErrPeerDisconnected` error if `Peer` connection is not up
			return 0, ErrPeerDisconnected
		}

		if errcnt > 3 {
			// returning error when download failures for a single block exceeds the limit (3)
			return 0, fmt.Errorf("download error limit exceeded (%v), for block-index=%v", errcnt, bidx)
		}

		if bidx >= len(piece.Blocks) {
			break
		}

		block := piece.Blocks[bidx]

		b, err := p.RequestBlock(block)
		if err != nil {
			errcnt++
			continue
		}
		errcnt = 0

		downs = append(downs, b...)

		bidx++
	}

	hash, err := GetSHA1(downs)
	if err != nil {
		return 0, fmt.Errorf("couldn't generate sha1 hash of downloaded data")
	}

	if !reflect.DeepEqual(hash, piece.Hash) {
		fmt.Printf("Hash doesn't match, piece-index = %v, %x != %x | %v:%v\n", piece.Index, hash, piece.Hash, p.IP, p.Port)
		return 0, fmt.Errorf("hash doesn't match")
	}

	return piece.WriteToFiles(downs)
}

// RequestBlock .
func (p *Peer) RequestBlock(block *Block) ([]byte, error) {
	buf, err := block.requestMsgBuf()
	if err != nil {
		return nil, fmt.Errorf("buffer build error, %v", err)
	}

	_, err = p.Conn.Write(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("message write error, %v", err)
	}

	msg, err := p.Read()
	if err != nil {
		logrus.Warnf("%v, disconnecting.. | %v:%v\n", err, p.IP, p.Port)
		p.Disconnect()
		return nil, err
	}

	// extracting length, message-id and payload from the message
	lng, id, payld, err := extractMsg(msg)
	if err != nil {
		logrus.Warnf("%v, disconnecting.. | %v:%v\n", err, p.IP, p.Port)
		p.Disconnect()
		return nil, err
	}

	// expecting "piece" message, (id==7)
	if id == uint8(7) && len(payld) > 8 {
		logrus.Infof("piece-message, %v bytes | %v:%v\n", lng, p.IP, p.Port)

		pidx := binary.BigEndian.Uint32(payld[:4]) // piece index
		beg := binary.BigEndian.Uint32(payld[4:8])
		data := payld[8:]
		// bidx := beg / uint32(LengthOfBlock)

		if pidx != block.PieceIndex || beg != block.Begin || len(data) != int(block.Length) {
			return nil, fmt.Errorf("recieved a unrequested block, pidx=%v,beg=%v,lng=%v", pidx, beg, len(data))
		}

		return data, nil
	}

	return nil, fmt.Errorf("unexpected message read, %v bytes", lng)
}

// GetSHA1 returns a `sha1` hash of a given []byte
func GetSHA1(b []byte) ([]byte, error) {
	h := sha1.New()
	_, err := h.Write(b)
	return h.Sum(nil), err
}

// extractMsg returns (length, id, and payload)
func extractMsg(b []byte) (uint32, uint8, []byte, error) {
	if len(b) < 5 {
		return 0, 0, nil, fmt.Errorf("message not long enough, %v bytes", len(b))
	}

	lng := binary.BigEndian.Uint32(b[:4])
	id := b[4]

	if len(b) > 5 {
		return lng, id, b[5:], nil
	}

	return lng, id, []byte{}, nil
}

// handshakeMsgBuf builds a handshake message buffer
func handshakeMsgBuf() (*bytes.Buffer, error) {
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
