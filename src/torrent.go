package src

import (
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
