package main

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

	"github.com/sirupsen/logrus"
)

var (
	// PeerProtocolV1 - Peer protocol name, for bitorrent protocol version 1
	PeerProtocolV1 = []byte("BitTorrent protocol")
	BlockLength    = math.Pow(2, 14)
)

// Peer .
type Peer struct {
	IP      net.IP
	Port    uint16
	Conn    net.Conn
	Sharing bool
	Pieces  []int
}

// GetPieces handles the peer protocol messeging between with a peer. First,
// it creates a tcp connection with the peer and sends a handshake request.
// If the peer wants to share data, it responds with a similar message back
// to our client. This is called a handshake. Once the handshake is successful
// we request pieces from via peer to peer messaging, to learn more visit -
// https://wiki.theory.org/index.php/BitTorrentSpecification#Peer_wire_protocol_.28TCP.29
func (p *Peer) GetPieces(torr *Torrent, ch chan *Peer) {
	var err error
	// establishing TCP connection with the peer client
	p.Conn, err = net.Dial("tcp", p.IP.String()+":"+strconv.Itoa(int(p.Port)))
	if err != nil {
		logrus.Warnf("%v\n", err)
		return
	}
	defer p.Conn.Close()

	// First we want to let the peer know what files you want and also
	// some unique identifier for our client, aka a Handshake message.

	// Buffer to be sant as handshake request
	hsbuf, err := HSBuf(torr)
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v\n", err)
		return
	}

	// Writing the handshake data to the peer connection
	_, err = p.Conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v\n", err)
		return
	}

	// Now, if the peer doesn’t have the files you want (info sent via-
	// info_hash in handshake request), they will close the connection,
	// but if they do then they should send back a similar message as
	// confirmation. We need to wait for the client to write back

	// http://allenkim67.github.io/programming/2016/05/04/how-to-make-your-own-bittorrent-client.html#downloading-from-peers
	// this article helped me a lot to understand peer to peer messeging

	// Simply, the client might not always send the data in 1 event, it
	// might send the messages in chunks as multiple events, we need to
	// take care of making sense of that data, and that's where the
	// message length data is useful at the start of the message

	// msgbuf will be used to store the message data while reading multiple times
	msgbuf := new(bytes.Buffer)
	// the first message is expected to be handshake type and the length defination
	// for handshake is different, so handshake is true at the start of reading, once
	// the first message has totally been read we will decleare the handshake as `false`
	handshake := true

	// Continiously reading from the peer client
	for {
		// Reading from the connection
		b := make([]byte, 1024)
		nr, err := p.Conn.Read(b)
		if err != nil {
			if err != io.EOF {
				logrus.Warnf("%v\n", err)
				// logrus.Warnf("read error with %v -> %v\n", conn.RemoteAddr(), err)
			}
			continue // if error on read skip teh rest of this iretation
		}
		data := b[:nr] // useful part of `b`

		// Extracting message from the read data
		if nr > 0 {
			var explen int // expected length, indicates how far a messgae should be read a single message

			// for handshake message (generally the first message) length of message is different, so the
			// value of `explen` will be extracted differently, (49+len(x)); x = length of protocol-string
			// to learn more, https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake
			if handshake {
				// reading the protocol string ("BitTorrent protocol" for version 1) length
				bf := new(bytes.Buffer)
				bf.Write(data)
				l, err := bf.ReadByte() // l, length specified at the start of
				if err != nil {
					logrus.Warnf("error on handshake response - %v\n", err)
				}

				// explen = (49+len(x)); for handshake message
				explen = int(uint8(l)) + 49
			} else {
				// for all other message type, the first 4 bytes (uint32) defines
				// the length of message data. So, reading the data `explen` to that
				explen = int(binary.BigEndian.Uint32(b[:4])) + 4
			}

			// writing the crrently read data to `msgbuf` buffer (later will be reset once message read is complete)
			msgbuf.Write(data)

			// fmt.Println("explen -> ", explen)

			// once `msgbuf` lnegth is equal to (or greater than) `explen`,
			// the message read is complete, doing what to do _________
			if msgbuf.Len() >= 4 && msgbuf.Len() >= explen {
				// fmt.Printf("Okay %v %v %s\n", conn.RemoteAddr(), msgbuf.Len(), msgbuf.Bytes())

				if handshake && IsHS(msgbuf.Bytes()) {
					logrus.Infof("handshake successful, %v bytes from %v\n", explen, p.Conn.RemoteAddr())
				} else {
					logrus.Infof("message recieved, %v bytes from %v\n", explen, p.Conn.RemoteAddr())
					p.handleMessages(msgbuf, torr, ch)
				}

				msgbuf.Reset()    // reseting the `msgbuf` buffer, message read is complete
				handshake = false // handshake is settin g to false (can only be true for first message read)
			}
		}

	}

}

// HSBuf builds and returns data to be sent on a handshake
// request to the peer (first message transmitted by the
// client after the TCP connection is established)
func HSBuf(torr *Torrent) (*bytes.Buffer, error) {
	// building the buffer, for the details -
	// https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, uint8(len(PeerProtocolV1))) // length of protocol name string, as a single raw byte
	err = binary.Write(buf, binary.BigEndian, PeerProtocolV1)              // string identifier of the protocol ("BitTorrent protocol")
	err = binary.Write(buf, binary.BigEndian, uint64(0))                   // eight (8) reserved bytes. All current implementations use all zeroes.
	err = binary.Write(buf, binary.BigEndian, torr.InfoHash)               // 20-byte string used as a unique ID for the client.
	err = binary.Write(buf, binary.BigEndian, ClientID)                    // 20-byte string used as a unique ID for client, the same peer_id that is transmitted in tracker requests

	return buf, err
}

// IsHS checks if a message data is handshake or not
func IsHS(b []byte) bool {
	return reflect.DeepEqual(b[1:20], PeerProtocolV1)
}

// Download .
func (p *Peer) Download(piece *Piece, wg *sync.WaitGroup) {
	fmt.Println("downloading.......")
	if p.Sharing {
		piece.Status = PieceFound
	}
	wg.Done()
}

func (p *Peer) handleMessages(buf *bytes.Buffer, torr *Torrent, ch chan *Peer) {
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

			p.Pieces = bits

			fmt.Println("Pice length ->", len(torr.Pieces))
			fmt.Println("bits length -> ", len(p.Pieces))

			ch <- p

			return

			// for i, bit := range p.Pieces {
			// 	if bit == 1 && torr.Pieces[i].Status == PieceNotFound {
			// 		torr.Pieces[i].Status = PieceFound
			// 	}
			// }
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

// writeRequest .
// func writeRequest(c net.Conn, index, ) error {

// }

// writetoconn writes data to a connection and returns
// error if there's an error
func writetoconn(c net.Conn, b []byte) error {
	_, err := c.Write(b)
	if err != nil {
		return err
	}
	return nil
}
