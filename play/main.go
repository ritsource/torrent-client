package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {

	// "http://torrent.ubuntu.com:6969/announce?info_hash=%90%28%9F%D3M%FC%1C%F8%F3%16%A2h%AD%D85L%853DX&peer_id=-PC0001-706887310628&uploaded=0&downloaded=0&left=699400192&port=6889&compact=1"

	// params := map[string]string{

	// }

	tracker := "http://torrent.ubuntu.com:6969/announce?info_hash=%90%28%9F%D3M%FC%1C%F8%F3%16%A2h%AD%D85L%853DX&peer_id=-PC0001-381828927258&port=6889&uploaded=0&downloaded=0&left=699400192&compact=1"

	resp, err := http.Get(tracker)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	fmt.Println("resp:\n", resp.Body)

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		bodyString := string(bodyBytes)
		fmt.Printf("%+v\n", bodyString)
	}

	// // raddr, err := net.ResolveUDPAddr("udp", "[http://torrent.ubuntu.com]:6969/announce")
	// // if err != nil {
	// // 	fmt.Println("Error:", err)
	// // 	return
	// // }

	// // conn, err := net.DialUDP("udp", nil, raddr)
	// // if err != nil {
	// // 	fmt.Println("Error:", err)
	// // 	return
	// // }

	// var b []byte
	// _, err = conn.Read(b)
	// fmt.Printf("Data:\n%+v\n", b)
}
