package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
)

// SwitchyOmega 新增test情景模式 添加本地代理服务器
func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Panicln(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panicln(err)
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	if conn == nil {
		return
	}
	defer conn.Close()

	log.Printf("remote addr: %v\n", conn.RemoteAddr())

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Println(err)
		return
	}

	var method, URL, address string
	fmt.Sscanf(string(buf[:bytes.IndexByte(buf[:], '\n')]), "%s%s", &method, &URL)
	hostPortURL, err := url.Parse(URL)
	if err != nil {
		log.Panicln(err)
		return
	}

	//如果方法是CONNECT，则为https协议
	if method == "CONNECT" {
		address = hostPortURL.Scheme + ":" + hostPortURL.Opaque
	} else {
		address = hostPortURL.Host
		if strings.Index(hostPortURL.Host, ":") == -1 { //host 不带端口， 默认 80
			address = hostPortURL.Host + ":80"
		}
	}

	log.Println("address  is ", address)

	server, err := net.Dial("tcp", address)
	if err != nil {
		log.Println(err)
		return
	}
	//如果使用 https 协议，需先向客户端表示连接建立完毕
	if method == "CONNECT" {
		fmt.Fprint(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
	} else { //如果使用 http 协议，需将从客户端得到的 http 请求转发给服务端
		server.Write(buf[:n])
	}
	//将客户端的请求转发至服务端，将服务端的响应转发给客户端。io.Copy 为阻塞函数，文件描述符不关闭就不停止
	go io.Copy(server, conn)
	io.Copy(conn, server)
}
