package main

import (
	"crypto/sha1"
	"fmt"
	"net/url"
	"os"
	"time"

	bencoding "github.com/marksamman/bencode"
	"github.com/ritwik310/torrent-client/bencode"
)

func infohash(info interface{}) []byte {
	enc := bencoding.Encode(info)
	h := sha1.New()
	h.Write(enc)
	hash := h.Sum(nil)

	return hash
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("no torrent file provided")
		return
	}
	fn := os.Args[1]

	torr, err := bencode.ReadTorrent(fn)
	if err != nil {
		fmt.Println("couldn't read/decode the torrent file data", err)
	}

	for k := range torr {
		fmt.Println("Key:", k)
	}

	infohash := infohash(torr["info"])
	peerID := string(time.Now().Unix())
	port := "6889"
	uploaded := "0"
	downloaded := "0"
	var left int64
	compact := "1"

	// info := torr["info"]

	if info, ok := torr["info"].(map[string]interface{}); ok {
		for key, val := range info {
			// fmt.Println(key, ":")
			// fmt.Printf("\n%+v\n\n", val)

			if key == "piece length" {
				left = val.(int64)
			}

			// if key == "pieces" && false {
			// 	fmt.Println("original type of info[\"pieces\"]:", reflect.TypeOf(val))
			// 	// pieces = val.(string)
			// }
		}

		// fmt.Printf("%s\n", y["pieces"])
	}

	fmt.Printf("info_hash: %#x\n", infohash)
	fmt.Printf("peer_id: %v\n", peerID)
	fmt.Printf("port: %v\n", port)
	fmt.Printf("uploaded: %v\n", uploaded)
	fmt.Printf("downloaded: %v\n", downloaded)
	fmt.Printf("left: %v\n", left)
	fmt.Printf("left: %v\n", compact)

	var Url *url.URL
	Url, err = url.Parse("http://bt1.archive.org:6969")
	if err != nil {
		panic("boom")
	}

	Url.Path += "/announce"
	parameters := url.Values{}
	parameters.Add("infohash", string(infohash))
	parameters.Add("peerID", peerID)
	parameters.Add("port", port)
	parameters.Add("uploaded", uploaded)
	parameters.Add("downloaded", downloaded)
	parameters.Add("left", string(left))
	parameters.Add("compact", compact)
	Url.RawQuery = parameters.Encode()

	fmt.Printf("Encoded URL is %q\n", Url.String())

	return

	// params := map[string]string{
	// 	"info_hash": string(h.Sum(nil)),
	// }

	// fmt.Println("params:", params)

	// return

	// fmt.Printf("%+v\n", torr["info"])

	// return

	// var pieces string

	// if info, ok := torr["info"].(map[string]interface{}); ok {
	// 	for key, val := range info {
	// 		fmt.Println(key, ":")
	// 		fmt.Printf("\n%+v\n\n", val)

	// 		if key == "pieces" && false {
	// 			fmt.Println("original type of info[\"pieces\"]:", reflect.TypeOf(val))
	// 			pieces = val.(string)
	// 		}
	// 	}

	// 	// fmt.Printf("%s\n", y["pieces"])
	// }

	// fmt.Println("len:", len(pieces))
	// fmt.Printf("%#x\n", pieces)

}
