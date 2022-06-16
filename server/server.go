package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

var (
	localPort  int
	remotePort int
)

func init() {
	flag.IntVar(&localPort, "l", 5200, "user link port")
	flag.IntVar(&remotePort, "r", 3333, "client listen port")
}

type client struct {
	conn   net.Conn
	read   chan []byte
	write  chan []byte
	exit   chan error
	reConn chan bool
}

func (c *client) Read(ctx context.Context) {
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, 10240)
			n, err := c.conn.Read(buf)
			if err != nil && err != io.EOF {
				if strings.Contains(err.Error(), "timeout") {
					c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
					c.conn.Write([]byte("ping"))
					continue
				}
				fmt.Println("读取出现错误...")
				c.exit <- err
				return
			}
			if n == 0 {
				continue
			}

			buf = buf[:n]
			if len(buf) == 4 && string(buf) == "ping" {
				fmt.Println("server收到心跳包")
				continue
			}
			c.read <- buf
		}
	}
}

func (c *client) Write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case buf := <-c.write:
			_, err := c.conn.Write(buf)
			if err != nil {
				c.exit <- err
				return
			}

		}
	}
}

type user struct {
	conn  net.Conn
	read  chan []byte
	write chan []byte
	exit  chan error
}

func (u *user) Read(ctx context.Context) {
	u.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, 10240)
			n, err := u.conn.Read(buf)
			if err != nil && err != io.EOF {
				if strings.Contains(err.Error(), "timeout") {
					u.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
					u.conn.Write([]byte("ping"))
					continue
				}
				fmt.Println("读取出现错误...")
				u.exit <- err
				return
			}
			if n == 0 {
				continue
			}
			buf = buf[:n]
			if len(buf) == 4 && string(buf) == "ping" {
				fmt.Println("server收到心跳包")
				continue
			}
			u.read <- buf
		}
	}
}

func (u *user) Write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case buf := <-u.write:
			_, err := u.conn.Write(buf)
			if err != nil {
				u.exit <- err
				return
			}

		}
	}
}

func main() {
	flag.Parse()

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	clientListener, err := net.Listen("tcp", fmt.Sprintf(":%d", remotePort))
	if err != nil {
		panic(err)
	}
	fmt.Printf("监听:%d端口, 等待client连接... \n", remotePort)
	//监听user来连接
	userListener, err := net.Listen("tcp", fmt.Sprintf(":%d", localPort))
	if err != nil {
		panic(err)
	}
	fmt.Printf("监听:%d端口, 等待user连接... \n", localPort)
	for {
		//有client连接
		clientConn, err := clientListener.Accept()
		if err != nil {
			panic(err)
		}
		fmt.Printf("有Client连接: %s \n", clientConn.RemoteAddr())
		client := &client{
			conn:   clientConn,
			read:   make(chan []byte),
			write:  make(chan []byte),
			exit:   make(chan error),
			reConn: make(chan bool),
		}
		userConnChan := make(chan net.Conn)

		go AcceptUserConn(userListener, userConnChan)
		go HandleClient(client, userConnChan)

		<-client.reConn
		fmt.Println("重新等待新的client连接..")
	}
}

func HandleClient(client *client, userConnChan chan net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	go client.Read(ctx)
	go client.Write(ctx)

	user := &user{
		read:  make(chan []byte),
		write: make(chan []byte),
		exit:  make(chan error),
	}

	defer func() {
		_ = client.conn.Close()
		_ = user.conn.Close()
		client.reConn <- true
	}()

	for {
		select {
		case userConn := <-userConnChan:
			user.conn = userConn
			go handle(ctx, client, user)
		case err := <-client.exit:
			fmt.Println("client出现错误, 关闭连接", err.Error())
			cancel()
			return
		case err := <-user.exit:
			fmt.Println("user出现错误，关闭连接", err.Error())
			cancel()
			return
		}
	}
}

func handle(ctx context.Context, client *client, user *user) {
	go user.Read(ctx)
	go user.Write(ctx)

	for {
		select {
		case userRecv := <-user.read:
			// 收到从user发来的信息
			client.write <- userRecv
		case clientRecv := <-client.read:
			// 收到从client发来的信息
			user.write <- clientRecv

		case <-ctx.Done():
			return
		}
	}
}

func AcceptUserConn(userListener net.Listener, connChan chan net.Conn) {
	userConn, err := userListener.Accept()
	if err != nil {
		panic(err)
	}

	fmt.Printf("user connect: %s \n", userConn.RemoteAddr())
	connChan <- userConn
}
