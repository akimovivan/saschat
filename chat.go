package main

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/sessions"
)

var (
	templates = template.Must(template.ParseGlob("templates/*.html"))
	store     = sessions.NewCookieStore([]byte("super-secret-key"))
	validPath = regexp.MustCompile("^/(chat|ws)/([a-zA-Z0-9]+)$")
	Rooms     = make(map[string]*room)
	validName = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			log.Println("Page not found")
			return
		}
		fn(w, r, m[2])
	}
}

func homePageHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "cookie-name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
	}
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		if !validName.MatchString(username) {
			http.Error(w, "Invalid name", http.StatusUnprocessableEntity)
			return
		}
		session.Values["username"] = username

		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	log.Println(Rooms)
	for _, value := range Rooms {
		log.Println(value.clients)
	}
	data := make(map[string]interface{})
	if _, ok := session.Values["username"].(string); !ok {
		data["Authenticated"] = false
	} else {
		data["Authenticated"] = true
	}
	var chatNames []string
	for roomName := range Rooms {
		chatNames = append(chatNames, roomName)
	}
	data["Rooms"] = chatNames
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "cookie-name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Session load failed")
		return
	}
	delete(session.Values, "username")
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func chatHandler(w http.ResponseWriter, req *http.Request, name string) {
	session, err := store.Get(req, "cookie-name")
	data := make(map[string]interface{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
	}

	if _, ok := session.Values["username"].(string); !ok {
		data["Username"] = "AnonymousUser"
	} else {
		data["Username"] = session.Values["username"]
	}

	if _, ok := Rooms[name]; !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		// time.Sleep(3 * time.Second)
		// http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}

	data["History"] = chatHistory[name]

	log.Println("Entered as:", session.Values["username"])
	log.Println("Trying to access chat: ", name)

	data["ChatTitle"] = name

	if err := templates.ExecuteTemplate(w, "chat.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
	}
}

func roomCreationHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		name := req.FormValue("chatName")
		lifespanStr := req.FormValue("lifespan")
		if !validName.MatchString(name) {
			http.Error(w, "Invalid name", http.StatusUnprocessableEntity)
			return
		}
		lifespan, err := strconv.Atoi(lifespanStr)
		if err != nil || lifespan < 1 {
			http.Error(w, "Invalid integer", http.StatusBadRequest)
			return
		}

		r := newRoom(name)

		go func() {
			time.Sleep(time.Duration(lifespan) * time.Second)
			delete(Rooms, name)
			delete(chatHistory, name)
			r.done <- 1
		}()

		log.Println("Created new room:", name)
		Rooms[name] = r
		go r.run()
		http.Redirect(w, req, "/chat/"+name, http.StatusSeeOther)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	log.Println("Tried to access roomCreation with", req.Method)
}

func connectToRoom(w http.ResponseWriter, req *http.Request, name string) {
	log.Println(req.URL.Path)
	log.Println("sasik")
	r := Rooms[name]
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatal("ServeHTTP:", err)
	}
	client := &client{
		socket:  socket,
		receive: make(chan []byte, messageBufferSize),
		room:    r,
	}
	r.join <- client
	defer func() { r.leave <- client }()
	go client.write()
	client.read()
}

func main() {
	fs := http.FileServer(http.Dir("static/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	// http.HandleFunc("/ws", connectionHandler)
	// go handleMessages()
	// r := newRoom("sas")

	http.HandleFunc("/", homePageHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/chat/", makeHandler(chatHandler))
	http.HandleFunc("/ws/", makeHandler(connectToRoom))
	http.HandleFunc("/create", roomCreationHandler)
	// go r.run()

	log.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
