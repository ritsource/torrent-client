package udp

import (
	"bufio"
	"bytes"
	"context"
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

func (c *Client) Send() error {
	p := make([]byte, 2048)

	conn, err := net.Dial("udp", c.Addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Println("2")
	fmt.Fprintf(conn, string(c.Msg))
	fmt.Println("3")

	_, err = bufio.NewReader(conn).Read(p)
	if err != nil {
		fmt.Println("err")
		return err
	}

	fmt.Println("4")

	fmt.Printf("%s\n", p)
	return nil
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

	return nil
}
