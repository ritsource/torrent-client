package peer3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"strconv"
	"sync"

	"github.com/ritwik310/torrent-client/queue"

	"github.com/ritwik310/torrent-client/info"
	"github.com/ritwik310/torrent-client/torrent"
	"github.com/sirupsen/logrus"
)

var (
	// PeerProtocolV1 - Peer protocol name, for bitorrent protocol version 1
	PeerProtocolV1 = []byte("BitTorrent protocol")
)

// Peer .
type Peer struct {
	IP       net.IP
	Port     uint16
	Conn     net.Conn
	Sharing  bool
	Bitfield []int
}

// Close .
func (p *Peer) Close() {
	p.Conn.Close()
}

// GetPieces .........
func (p *Peer) GetPieces(torr *torrent.Torrent, wg *sync.WaitGroup, que *queue.Queue) {
	var err error
	p.Conn, err = net.Dial("tcp", p.IP.String()+":"+strconv.Itoa(int(p.Port)))
	if err != nil {
		logrus.Warnf("%v\n", err)
		return
	}
	// defer p.Conn.Close()

	hsbuf, err := HSBuf(torr)
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v\n", err)
		return
	}

	_, err = p.Conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v\n", err)
		return
	}

	msgbuf := new(bytes.Buffer)
	handshake := true

	// messaging := true
	// iff := 0

	// go func(msg *bool) {
	// 	time.Sleep(5 * time.Second)
	// 	*msg = false
	// 	fmt.Println("Bang!")
	// 	p.Conn.Close()
	// }(&messaging)

	// for messaging {
	// 	fmt.Println("Reading ->>>>>>>", iff)
	// 	iff++

	for {
		b := make([]byte, 1024)
		nr, err := p.Conn.Read(b)
		if err != nil {
			if err != io.EOF {
				logrus.Warnf("%v\n", err)
				break
			}
			continue
		}
		data := b[:nr]

		if nr > 0 {
			var explen int

			if handshake {
				bf := new(bytes.Buffer)
				bf.Write(data)
				l, err := bf.ReadByte()
				if err != nil {
					logrus.Warnf("error on handshake response - %v\n", err)
				}

				explen = int(uint8(l)) + 49
			} else {
				explen = int(binary.BigEndian.Uint32(b[:4])) + 4
			}

			msgbuf.Write(data)

			if msgbuf.Len() >= 4 && msgbuf.Len() >= explen {
				if handshake && IsHS(msgbuf.Bytes()) {
					logrus.Infof("handshake successful, %v bytes from %v\n", explen, p.Conn.RemoteAddr())
				} else {
					logrus.Infof("message recieved, %v bytes from %v\n", explen, p.Conn.RemoteAddr())
					p.handleMessages(msgbuf, torr, wg)
				}

				msgbuf.Reset()
				handshake = false
			}
		}

	}
}

// HSBuf builds
func HSBuf(torr *torrent.Torrent) (*bytes.Buffer, error) {
	// building the buffer, for the details -
	// https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, uint8(len(PeerProtocolV1))) // length of protocol name string, as a single raw byte
	err = binary.Write(buf, binary.BigEndian, PeerProtocolV1)              // string identifier of the protocol ("BitTorrent protocol")
	err = binary.Write(buf, binary.BigEndian, uint64(0))                   // eight (8) reserved bytes. All current implementations use all zeroes.
	err = binary.Write(buf, binary.BigEndian, torr.InfoHash)               // 20-byte string used as a unique ID for the client.
	err = binary.Write(buf, binary.BigEndian, info.ClientID)               // 20-byte string used as a unique ID for client, the same peer_id that is transmitted in tracker requests

	return buf, err
}

// IsHS checks if a message data is handshake or not
func IsHS(b []byte) bool {
	return reflect.DeepEqual(b[1:20], PeerProtocolV1)
}

// Download .
func (p *Peer) Download(torr *torrent.Torrent, que *queue.Queue) {
	fmt.Println("downloading.......")
	// if p.Sharing {
	// 	piece.Status = torrent.PieceFound
	// }

	piece := que.Pop()
	i := piece.Index

	// if piece.Status == torrent.PieceNotFound || piece.Status == torrent.PieceFailed {

	if len(p.Bitfield) > i {
		if p.Bitfield[i] == 1 {

			fmt.Println("Requesting ->>>>>>>>>>>>>>>>>>>>>>", i)
			p.RequestPiece(piece, i, torr.PieceLen)
		}
		// piece.Status = torrent.PieceFound
	}

	// }
}

// RequestPiece .
func (p *Peer) RequestPiece(piece *torrent.Piece, pieceidx int, piecelen int) {
	nBlocks := int(math.Ceil(float64(piecelen) / float64(torrent.BlockLength)))

	for i := 0; i < nBlocks; i++ {
		var length int
		if i+1 == nBlocks {
			length = piecelen % torrent.BlockLength
		} else {
			length = torrent.BlockLength
		}

		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, uint32(13))
		binary.Write(buf, binary.BigEndian, uint8(6))
		binary.Write(buf, binary.BigEndian, uint32(pieceidx))
		binary.Write(buf, binary.BigEndian, uint32(i*torrent.BlockLength))
		binary.Write(buf, binary.BigEndian, uint32(length))

		_, err := p.Conn.Write(buf.Bytes())
		if err != nil {
			fmt.Println("Error:->>>>>>>>>>>>>><><><><>")
		}

		fmt.Println("request sent +++++++++++++")

	}
}

func (p *Peer) handleMessages(buf *bytes.Buffer, torr *torrent.Torrent, wg *sync.WaitGroup) {
	if IsHS(buf.Bytes()) {
		writeInterested(p.Conn)
	} else {
		var length uint32
		var id uint8
		binary.Read(buf, binary.BigEndian, &length)
		binary.Read(buf, binary.BigEndian, &id)

		payload := make([]byte, length-1)
		// var payload []byte
		binary.Read(buf, binary.BigEndian, &payload)

		switch id {
		case uint8(0):
			logrus.Info("choke")
			// p.Conn.Close()
			// wg.Done()
		case uint8(1):
			logrus.Info("unchoke")
			p.Sharing = true
			// writeInterested(p.Conn)
		case uint8(2):
			logrus.Info("interested")
		case uint8(3):
			logrus.Info("not interested")
		case uint8(4):
			logrus.Info("have")
			// p.Sharing = true

			// i := binary.BigEndian.Uint32(payload)
			// p.Pieces[i] = 1

		case uint8(5):
			logrus.Info("bitfield")
			p.Sharing = true

			bits := toBits(payload)

			if len(torr.Pieces) != len(bits) {
				logrus.Errorf("piece length mismatch in bitfield message")
				return
			}

			p.Bitfield = bits

			fmt.Println("Pice length ->", len(torr.Pieces))
			fmt.Println("bits length -> ", len(p.Bitfield))

		case uint8(6):
			logrus.Info("request")
		case uint8(7):
			logrus.Info("piece")
		case uint8(8):
			logrus.Info("cancel")
		case uint8(9):
			logrus.Info("port")
		}
	}
	// fmt.Printf("%s\n", buf.Bytes())
}

func toBits(bs []byte) []int {
	r := make([]int, len(bs)*8)
	for i, b := range bs {
		for j := 0; j < 8; j++ {
			r[i*8+j] = int(b >> uint(7-j) & 0x01)
		}
	}
	return r
}

// handleChoke ...
func handleChoke(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleUnchoke ...
func handleUnchoke(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleInterested ...
func handleInterested(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleChoke ...
func handleNotInterested(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleHave ...
func handleHave(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleBitfield ...
func handleBitfield(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleRequest ...
func handleRequest(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handlePiece ...
func handlePiece(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleCancel...
func handleCancel(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handlePort ...
func handlePort(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// the following methods are responsible for sending various event
// messages over the connection to the peer client, for more details
// https://wiki.theory.org/index.php/BitTorrentSpecification#Messages
// also, https://www.bittorrent.org/beps/bep_0003.html

// the message data simply looks something like <length prefix> <message ID> <payload> (defined by the protocol)
// length prefix -> is a uint32 that indicates the total length of the <message ID> and <payload>
// message ID -> defines the type is the message, an uint8
// payload -> is the payload requested if there's any (usually downloadable data)

// writeKeepAlive sends a keep-alive message to the peer-client it
// must be sent to maintain the connection alive if no command
// have been sent for a given amount of time (generally two minutes)
func writeKeepAlive(c net.Conn) error {
	// for keep-alive message there's (no id, no payload),
	// it's just 4 bytes of containing 0
	return writetoconn(c, make([]byte, 4))
}

// writeChoke message sender
func writeChoke(c net.Conn) error {
	// The choke message has an id of 0 and has no payload
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1)) // fixed-length just the id prop
	binary.Write(buf, binary.BigEndian, uint8(0))  // id => 0 (for choke message)

	return writetoconn(c, buf.Bytes())
}

// writeUnChoke message sender
func writeUnChoke(c net.Conn) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1))
	binary.Write(buf, binary.BigEndian, uint8(1))

	return writetoconn(c, buf.Bytes())
}

// writeInterested message sender
func writeInterested(c net.Conn) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1))
	binary.Write(buf, binary.BigEndian, uint8(2))

	return writetoconn(c, buf.Bytes())
}

// writeNotInterested message sender
func writeNotInterested(c net.Conn) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1))
	binary.Write(buf, binary.BigEndian, uint8(3))

	return writetoconn(c, buf.Bytes())
}

// writeHave message sender
func writeHave(c net.Conn, pi uint32) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(5))
	binary.Write(buf, binary.BigEndian, uint8(4))
	binary.Write(buf, binary.BigEndian, pi)

	return writetoconn(c, buf.Bytes())
}

// writetoconn writes data to a connection and returns
// error if there's an error
func writetoconn(c net.Conn, b []byte) error {
	_, err := c.Write(b)
	if err != nil {
		return err
	}
	return nil
}
