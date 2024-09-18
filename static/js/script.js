"use strict";

window.onload = function() {
    const input = document.getElementById("message");
    const output = document.getElementById("chatWindow");
    const username = document.getElementById("username").innerHTML;
    const socket = new WebSocket("ws://192.168.10.151:8080/ws");

    socket.onopen = function () {
        output.innerHTML += "Status: Connected\n";
    };

    socket.onmessage = function (message) {
        //output.innerHTML += username +": " + message.data + "\n"

        const msg = JSON.parse(event.data);
        // console.log("Recieved message:", msg)
        output.innerHTML += `<p>${msg['username']}: ${msg["message"]}</p>`
    };

    document.getElementById('sendButton').addEventListener('click', function(e) {
        e.preventDefault();
        const msg = {
            username: username,
            message: input.value
        };
        socket.send(JSON.stringify(msg));   
        input.value = "";

    });
}

async function getMessage(ws) {
    const url = "ws://192.168.10.151:8080/ws";
    try {
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`Response status: ${response.status}`);
        }

        const json = await response.json();
        console.log(json);
    } catch (error) {
        console.error(error.message)
    }
}
