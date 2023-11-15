package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"
	"unicode/utf8"
)

const (
	port        = "6969"
	safeMode    = true
	messageRate = 1.0
	BanLimit    = 10 * 60.0
	StrikeLimit = 10
)

func sensitive(message string) string {
	if safeMode {
		return "[REDACTED]"
	} else {
		return message
	}
}

type messageType int

const (
	clientConnected messageType = iota + 1
	newMessage
	ClientDisconected
)

type Message struct {
	TypeMsg messageType
	Conn    net.Conn
	Text    string
}

type Clients struct {
	Conn        net.Conn
	LastMessage time.Time
	StrikeCount int
}

func server(message chan Message) {
	clients := map[string]*Clients{}
	bannedMfs := map[string]time.Time{}

	for {
		msg := <-message
		switch msg.TypeMsg {
		case clientConnected:
			addr := msg.Conn.RemoteAddr().(*net.TCPAddr)

			bannedAt, banned := bannedMfs[addr.IP.String()]
			now := time.Now()
			if banned {
				if time.Now().Sub(bannedAt).Seconds() >= BanLimit {
					delete(bannedMfs, addr.IP.String())
					banned = false
				}
			}
			if !banned {
				log.Printf("client %s connected", sensitive(addr.String()))
				clients[msg.Conn.RemoteAddr().String()] = &Clients{
					Conn:        msg.Conn,
					LastMessage: time.Now(),
				}
			} else {
				msg.Conn.Write([]byte(fmt.Sprintf("you are banned at %f secs left\n", now.Sub(bannedAt).Seconds())))
				msg.Conn.Close()
			}

		case ClientDisconected:
			addr := msg.Conn.RemoteAddr().(*net.TCPAddr)
			log.Printf("client %s disconnected", sensitive(addr.String()))
			delete(clients, addr.String())

		case newMessage:
			authorAddr := msg.Conn.RemoteAddr().(*net.TCPAddr)
			now := time.Now()
			author := clients[authorAddr.String()]
			// check the author's map condition, when he has been banished is it nil / not
			if author != nil {
				if now.Sub(clients[authorAddr.String()].LastMessage).Seconds() >= 1.0 {
					// supplies only utf8 characters that are allowed to be sent
					if utf8.Valid([]byte(msg.Text)) {
						author.LastMessage = now
						author.StrikeCount = 0

						log.Printf("client %s sent message %s", sensitive(authorAddr.String()), msg.Text)
						for _, client := range clients {
							if client.Conn.RemoteAddr().String() != authorAddr.String() {
								client.Conn.Write([]byte(msg.Text))
								// if err != nil {
								// 	log.Printf("Could not send data to %s %s", sensitive(authorAddr.String()), sensitive(err.Error()))
								// }
							}

						}
					} else {
						author.StrikeCount += 1
						if author.StrikeCount >= StrikeLimit {
							bannedMfs[authorAddr.IP.String()] = now
							author.Conn.Close()
							author.Conn.Write([]byte("you are banned"))
						}

					}
				} else {
					author.StrikeCount += 1
					if author.StrikeCount >= StrikeLimit {
						bannedMfs[authorAddr.IP.String()] = now
						author.Conn.Close()
						author.Conn.Write([]byte("you are banned"))
					}

				}
			} else {
				msg.Conn.Close()
			}

		}
	}

}

// create TCP
func client(conn net.Conn, messages chan Message) {
	buffer := make([]byte, 512)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Could not read from %s: %s", sensitive(conn.RemoteAddr().String()), sensitive(err.Error()))
			defer conn.Close()
			messages <- Message{
				TypeMsg: ClientDisconected,
				Conn:    conn,
			}
			return
		}

		text := string(buffer[0:n])
		if text == ":quit" {

		}
		messages <- Message{
			TypeMsg: newMessage,
			Text:    text,
			Conn:    conn,
		}
	}

}

func main() {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Printf("ERROR: Could not listen to epic port")
		os.Exit(1)
	}
	log.Printf("Listning TCP server on port %s.....", port)

	// initialisasi channel message
	messages := make(chan Message)
	go server(messages)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("ERROR: Could not Accept connection:%s\n", err)
			os.Exit(1)
		}
		log.Printf("Accepted connection from %s", sensitive(conn.RemoteAddr().String()))
		messages <- Message{
			TypeMsg: clientConnected,
			Conn:    conn,
		}
		go client(conn, messages)
	}
}
