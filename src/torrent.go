package src

import (
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

// func (t *Torr) DecodePieces() {

// }
