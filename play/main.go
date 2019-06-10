package main

import (
	"fmt"
	"net/http"
)

func main() {

	// "http://torrent.ubuntu.com:6969/announce?info_hash=%90%28%9F%D3M%FC%1C%F8%F3%16%A2h%AD%D85L%853DX&peer_id=-PC0001-706887310628&uploaded=0&downloaded=0&left=699400192&port=6889&compact=1"

	// params := map[string]string{

	// }

	tracker := "http://bt1.archive.org:6969/announce?compact=1&downloaded=0&infohash=%5E3tV%85%8DUj%14%D5%9Be%AE1%2CrH%E6%BA%C7&left=%F2%80%80%80&peerID=%EF%BF%BD&port=6889&uploaded=0"
	resp, err := http.Get(tracker)

	// conn, err := net.Dial("tcp", "[http://torrent.ubuntu.com]:6969/announce")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("resp:\n", resp)

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
