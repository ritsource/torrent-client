package client

import (
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/marksamman/bencode"
)

// Torr ...
type Torr struct {
	Data map[string]interface{}
	// Pieces
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
func (t *Torr) ReadPieces() {
	if info, ok := (*t).Data["info"].(map[string]interface{}); ok {
		var piecelength int
		var comblength int

		// TODO: WTF, did I just use a loop! (-_-)
		for key, val := range info {
			// fmt.Println(key, ":")
			// fmt.Printf("\n%+v\n\n", val)
			fmt.Println("R:", key)

			if key == "piece length" {
				piecelength = int(val.(int64))
				fmt.Println("piecelength:", piecelength)
			}

			if key == "pieces" {
				// fmt.Println("original type of info[\"pieces\"]:", reflect.TypeOf(val))

				fmt.Printf("hhhhhhh %v\n", len([]byte(val.(string))))
				comblength = piecelength * (len([]byte(val.(string))) / 20)
				fmt.Println("comblength:", comblength)
			}
		}

	}
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
