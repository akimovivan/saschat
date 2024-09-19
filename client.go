package main

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

var chatHistory = make(map[string][]Message)

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
			return
		}
		var mesg Message
		json.Unmarshal(msg, &mesg)
		chatHistory[c.room.name] = append(chatHistory[c.room.name], mesg)
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
