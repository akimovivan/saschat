package main

import (
	"log"

	"github.com/gorilla/websocket"
)

type room struct {
	name    string
	clients map[*client]bool
	join    chan *client
	leave   chan *client
	forward chan []byte
	done    chan int
}

func newRoom(name string) *room {
	return &room{
		name:    name,
		forward: make(chan []byte),
		join:    make(chan *client),
		leave:   make(chan *client),
		clients: make(map[*client]bool),
		done:    make(chan int),
	}
}

func (r *room) run() {
	for {
		select {
		case client := <-r.join:
			r.clients[client] = true

		case client := <-r.leave:
			delete(r.clients, client)
			close(client.receive)
		case msg := <-r.forward:
			for client := range r.clients {
				log.Println(string(msg))
				client.receive <- msg
			}
		case <-r.done:
			log.Printf("Closing room: %s", r.name)
			for client := range r.clients {
				client.receive <- []byte(`{"username":"SERVER","message":"The room is closing."}`)
				log.Println("Closes for user", client)
				client.socket.Close()
			}
			return
		}
	}
}

// func (r *room) close() {
// 	close(r.done)
// }

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

var upgrader = &websocket.Upgrader{ReadBufferSize: socketBufferSize, WriteBufferSize: socketBufferSize}
