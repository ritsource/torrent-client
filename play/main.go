package main

import (
	"fmt"
	"net/http"
)

func main() {

	tracker := "http://bt1.archive.org:6969/announce"
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
