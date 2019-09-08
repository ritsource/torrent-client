package src

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"

	"github.com/ritwik310/torrent-client/output"
)

// Constants corrosponding to `Status` enum value of of `Piece`
const (
	PieceStatusDefault    uint8 = 0 // default state, 0 when a piece is created
	PieceStatusRequested  uint8 = 1 // when the piece have been requested to a peer
	PieceStatusDownloaded uint8 = 2 // when the piece download has successfully been completed
	PieceStatusFailed     uint8 = 3 // when the piece download has not been successful (failed atleast once)
)

// Piece represents an individual piece of data,
type Piece struct {
	Index  uint32   // piece-index
	Hash   []byte   // 20-byte long SHA1-hash of the piece-data, extracted from `.torrent` file
	Length uint32   // size of piece (equal to piece-length of torrent)
	Blocks []*Block // pointer to blocks that the piece conatins
	Status uint8    // status of the piece default, requested, downloaded, failed
}

// GenBlocks calculates out blocks of data of a piece and
// appends pointer to all the `Block` on `Piece.Blocks`
func (p *Piece) GenBlocks() {
	// nubmer of blocks that the piece holds (for block-length = LengthOfBlock)
	n := int(math.Ceil(float64(p.Length / uint32(LengthOfBlock))))

	// calculating each possible block's "index", "start
	// -offset" and "length" and appending them to `p.Blocks`
	for i := 0; i < n; i++ {
		var ln int
		if i == n-1 && int(p.Length)%LengthOfBlock != 0 {
			ln = int(p.Length) % LengthOfBlock
		} else {
			ln = LengthOfBlock
		}

		p.Blocks = append(p.Blocks, &Block{
			PieceIndex: p.Index,
			Begin:      uint32(i * LengthOfBlock),
			Length:     uint32(ln),
		})
	}
}

// WriteToFiles .
func (p *Piece) WriteToFiles(data []byte) (int, error) {

	output.DevInfof("Piece-Index=%v\n", p.Index)

	// retrieving the files where the data has to be written, the method
	// `Torr.WhichFiles` returns pointer to all the files that a piece covers
	fs := Torr.WhichFiles(int(p.Index))

	// `psoff` and `peoff` are the "piece start offset" and
	// "piece end offset" in the full concatinated data
	psoff := int(p.Length * p.Index)
	peoff := psoff + int(p.Length)

	// calculating and writing the right chunk of for each file
	for _, f := range fs {
		// `ws` and `we` is the offset "in the chunk of data"
		// (`data`) where the file write needs to begin, and end
		var ws int
		var we int
		// `off `is the offset of file where the data needs to be written
		var off int

		// calculating the start of data write, wriet offset in file and chunk of data

		// if `f.Start` (start offset of file in full concatenated data)
		// is greater the `psoff` (start offset of piece in full data)
		if f.Start > psoff {
			// if `true`, then file write needs to begin (`off`) at the start of file and the
			// data will be starting (`ws`) at `f.Start - psoff` offset "in the chunk of data"
			ws = f.Start - psoff
			off = 0
		} else {
			// if `false`, then the file write needs to begin (`off`) where the piece begins, and
			// the data that needs to be written starts (`ws`) from the start of the "chunk of data"
			ws = 0
			off = psoff - f.Start
		}

		// calculating the start of data write, wriet offset in file and chunk of data

		// if end of file in the whole data (`f.Start+f.Length`)
		// is after end of piece in the whole data (`peoff`), then
		// the write needs to end when teh piece ends (`we = p.Length`)
		// else, it ends when the file ends (`f.Start + f.Length - psoff`)
		if f.Start+f.Length > peoff {
			we = int(p.Length)
		} else {
			we = f.Start + f.Length - psoff
		}

		// you can find a more detailed explaination of this method,
		// https://ritwiksaha.com/blog/write-a-torrent-client-in-go

		// so `data[ws:we]` needs to be written, at `off` offset of file
		_, err := f.WriteData(data[ws:we], off)
		if err != nil {
			return 0, err
		}
	}

	return int(p.Length), nil
}

/*
LengthOfBlock is the length of each block. While downloading pieces
from the peers, we request pieces in chunks. This is called a block.
Typically, each block happens to be 2^14 (16384) bytes in size
*/
var LengthOfBlock = int(math.Pow(2, 14))

// Block represents a block of data (a chunk of piece)
type Block struct {
	PieceIndex uint32 // piece-index of the piece that the block is a part of
	Begin      uint32 // offset where the block starts within the piece (that it's a part of)
	Length     uint32 // length of the block in bytes
}

// RequestBuff builds and returns a buffer for `Block` request message
func (b *Block) RequestBuff() (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	// length of message (1 (id) + 4 (piece-index) + 4 (begin) + 4 (length) = 13 bytes)
	err := binary.Write(buf, binary.BigEndian, uint32(13))
	err = binary.Write(buf, binary.BigEndian, uint8(6))     // message-id
	err = binary.Write(buf, binary.BigEndian, b.PieceIndex) // piece-index
	err = binary.Write(buf, binary.BigEndian, b.Begin)      // length
	err = binary.Write(buf, binary.BigEndian, b.Length)     // begin offset
	return buf, err
}

// File holds necessary info for each file
// to be constructed with the downloaded data
type File struct {
	Path   string // the path where the file needs to be written
	Length int    // file size (in bytes)
	Start  int    // start-offset of the file-data in the full/all data (all pieces concatenated)
}

// Create creates a file with the directories that it requires to be created
func (f *File) Create() error {
	err := os.MkdirAll(filepath.Dir(f.Path), os.ModePerm)
	if err != nil {
		return err
	}

	_, err = os.Create(f.Path)
	return err
}

// WriteData creates a file (if not already) and writes
// the provided data to a `File` at provided offset
func (f *File) WriteData(bs []byte, off int) (int, error) {
	var fl *os.File // os.File, to be written data on

	// checking if file exist or not
	_, err := os.Stat(f.Path)
	os.IsNotExist(err)
	if _, err := os.Stat(f.Path); err == nil {
		// pass
	} else if os.IsNotExist(err) {
		// create a file (with the folders) if doesn't exist
		err := f.Create()
		if err != nil {
			return 0, err
		}
	} else {
		return 0, err
	}

	// opening the file to write the data on it
	fl, err = os.OpenFile(f.Path, os.O_RDWR, os.ModeAppend)
	if err != nil {
		return 0, err
	}

	// closing the file at in after the method returns
	defer fl.Close()

	// also if any error while opening the file, throw error
	if err != nil {
		return 0, err
	}

	// writing the provided data on the right file offset (also provided)
	return fl.WriteAt(bs, int64(off))
}
