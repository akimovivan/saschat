package main

import (
	"log"

	"github.com/gorilla/websocket"
)

type client struct {
	socket  *websocket.Conn
	receive chan []byte
	room    *room
}

func (c *client) read() {
	defer c.socket.Close()
	for {
		_, msg, err := c.socket.ReadMessage()
		if err != nil {
			log.Println(err)
		}
		c.room.forward <- msg
	}
}

func (c *client) write() {
	defer c.socket.Close()
	for msg := range c.receive {
		err := c.socket.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println(err)
		}
	}
}
