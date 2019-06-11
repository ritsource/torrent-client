package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	bencoding "github.com/marksamman/bencode"
	"github.com/ritwik310/torrent-client/bencode"

	"github.com/ritwik310/torrent-client/torrent"
)

func infohash(info interface{}) []byte {
	enc := bencoding.Encode(info)
	// dec, _ := bencoding.Decode(bytes.NewReader(enc))

	// fmt.Printf("%+v\n", dec)

	h := sha1.New()
	h.Write(enc)
	hash := h.Sum(nil)

	return hash
}

func getip() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func gethash(str string) []byte {
	h := sha1.New()
	io.WriteString(h, str)
	hash := h.Sum(nil)

	return hash
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("no torrent file provided")
		return
	}
	fn := os.Args[1]

	torrent.Run(fn)

	return

	torr, err := bencode.ReadTorrent(fn)
	if err != nil {
		fmt.Println("couldn't read/decode the torrent file data", err)
	}

	for k := range torr {
		fmt.Println("Key:", k)
	}

	fmt.Println(torr["announce"])
	// return

	// infohash := base64.URLEncoding.EncodeToString(infohash(torr["info"]))

	fmt.Println("LENGTH:", len(infohash(torr["info"])))

	// infohash := base64.StdEncoding.EncodeToString()
	infohash := infohash(torr["info"])

	// peerID := string(time.Now().Unix())

	peerID := "-qB3130-" + "381828934258"

	fmt.Println("********************", len(peerID))
	port := "6882"
	uploaded := "0"
	downloaded := "0"
	var left int
	compact := "1"

	// info := torr["info"]

	if info, ok := torr["info"].(map[string]interface{}); ok {
		var piecelength int
		var comblength int

		for key, val := range info {
			// fmt.Println(key, ":")
			// fmt.Printf("\n%+v\n\n", val)

			if key == "piece length" {
				piecelength = int(val.(int64))
				fmt.Println("piecelength:", piecelength)
			}

			if key == "pieces" {
				// fmt.Println("original type of info[\"pieces\"]:", reflect.TypeOf(val))

				fmt.Printf("%v\n", len([]byte(val.(string))))
				comblength = piecelength * (len([]byte(val.(string))) / 20)
				fmt.Println("comblength:", comblength)
			}
		}

		left = comblength
		// fmt.Printf("%s\n", "pl", pl)
	}

	fmt.Printf("info_hash: %#x\n", infohash)
	// fmt.Println(string(infohash))
	fmt.Printf("peer_id: %v\n", peerID)
	fmt.Printf("port: %v\n", port)
	fmt.Printf("uploaded: %v\n", uploaded)
	fmt.Printf("downloaded: %v\n", downloaded)
	fmt.Printf("left: %v\n", left)
	fmt.Printf("compact: %v\n", compact)
	// gethash
	var Url *url.URL
	Url, err = url.Parse(torr["announce"].(string))
	if err != nil {
		panic("boom")
	}

	// Url.Path += "/announce"
	parameters := url.Values{}
	parameters.Add("info_hash", string(infohash))
	parameters.Add("peer_id", peerID)
	parameters.Add("port", port)
	parameters.Add("uploaded", uploaded)
	parameters.Add("downloaded", downloaded)
	parameters.Add("left", strconv.Itoa(int(left)))
	parameters.Add("compact", compact)
	parameters.Add("event", "started")
	parameters.Add("ip", "10.13.48.176")
	Url.RawQuery = parameters.Encode()

	fmt.Printf("Encoded URL is %q\n", Url.String())

	resp, err := http.Get(Url.String())
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	fmt.Println("resp:\n", resp)

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		bodyString := string(bodyBytes)
		fmt.Printf("%+v\n", bodyString)
	}

}
