package udp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type Client struct {
	Msg  []byte
	Addr string
}

func (c *Client) Send() ([]byte, error) {
	p := make([]byte, 16)

	conn, err := net.Dial("udp", c.Addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	fmt.Println("2")
	fmt.Fprintf(conn, string(c.Msg))
	fmt.Println("3")

	_, err = bufio.NewReader(conn).Read(p)
	if err != nil {
		fmt.Println("err")
		return nil, err
	}

	// fmt.Println("4")
	// fmt.Printf("action%+s\n", binary.BigEndian.Uint32(p[0:4]))
	// fmt.Printf("transaction_id%+s\n", binary.BigEndian.Uint32(p[4:8]))
	// fmt.Printf("connection_id%+s\n", binary.BigEndian.Uint64(p[8:16]))
	// fmt.Printf("action%+s\n", lib)
	return p, nil
}

func tempread32(b []byte, v int32) error {
	buf := bytes.NewBuffer(b)
	err := binary.Read(buf, binary.BigEndian, v)
	return err
}

func (c *Client) Send2() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cancel()
	}()

	reader := bytes.NewReader(c.Msg)

	fmt.Println("sending to " + c.Addr)
	err := client(ctx, c.Addr, reader)
	if err != nil && err != context.Canceled {
		panic(err)
	}

	var b []byte
	_, err = reader.Read(b)
	if err != nil {
		fmt.Println("::::::::")
		return err
	}

	fmt.Printf("%+v\n", b)

	return nil
}
