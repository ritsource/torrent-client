package client

import (
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/marksamman/bencode"
)

// NOTFOUND ...
var NOTFOUND = uint8(0)

// FOUND ...
var FOUND = uint8(1)

// REQUESTED ...
var REQUESTED = uint8(2)

// DOWNLOADED ...
var DOWNLOADED = uint8(3)

// var PIECE = 0
// var PIECE = 0

// Torr ...
type Torr struct {
	Data   map[string]interface{}
	Pieces []Piece
}

// Piece ...
type Piece struct {
	Status int8
	Hash   []byte
	Blocks []Block
}

// Block ...
type Block struct {
	Index  int
	Begin  int
	Length int
}

// Torr represents mets data from the torrent file
// type Torr map[string]interface{}

// ReadFile reads torrent file from the provided filepath,
// decodes bencode data and populates Torr properties (metainfo & info)
func (t *Torr) ReadFile(fp string) error {
	// reading a torrent file
	f, err := os.Open(fp)
	if err != nil {
		return err
	}

	// decoding torrent file data
	bd, err := bencode.Decode(f)
	if err != nil {
		return err
	}

	// torr := Torr(bd)
	(*t).Data = bd

	return nil
}

// ReadPieces ...
func (t *Torr) ReadPieces() error {
	if info, ok := (*t).Data["info"].(map[string]interface{}); ok {
		pieces := []byte(info["pieces"].(string))

		for i := 0; i+20 < len(pieces); i += 20 {
			fmt.Printf("%#x\n", pieces[i:i+20])

			t.Pieces = append(t.Pieces, Piece{Status: 0, Hash: pieces[i : i+20]})
		}

		return nil
	}

	return fmt.Errorf("unable to read pieces from torrent")
}

// Totalbytes calculates the total number of bytes to be
// downloaded at start, from torr["info"] values
func (t *Torr) Totalbytes() int {
	var totalbytes int // total number of bytes

	if info, ok := (*t).Data["info"].(map[string]interface{}); ok {
		var pl int // pl holds the value of `piece length`, length of each piece in bytes (its equal for all pieces)

		// iretating over info and reading necessary fields
		for k, v := range info {
			switch k {
			case "piece length":
				pl = int(v.(int64)) // each piece length
			case "pieces":
				// `pieces` contains of hashed values for all files, each hash is 20 bytes long,
				// so deviding the length of `pieces's value` will give us the number of pieces,
				// and multiplying it with `piece length` will be the total size of all pieces
				totalbytes = pl * (len([]byte(v.(string))) / 20)
			}
		}
	}

	return totalbytes
}

// Infohash gets value of info from metainfo map, and
// returns a 20 byte long sha1 hash of all the info contents
func (t *Torr) Infohash() []byte {
	enc := bencode.Encode((t).Data["info"])
	h := sha1.New()
	h.Write(enc)
	hash := h.Sum(nil)

	return hash
}
