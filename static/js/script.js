"use strict";

const wsURL = "ws://your-ip-address:8080/ws"
window.onload = function() {
    const input = document.getElementById("message");
    const output = document.getElementById("chatWindow");
    const username = document.getElementById("username").innerHTML;
    const socket = new WebSocket(wsURL);

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
