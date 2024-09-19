package main

import (
	"log"
	"net/http"
	"regexp"
	"text/template"

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
		if !validName.MatchString(name) {
			http.Error(w, "Invalid name", http.StatusUnprocessableEntity)
			return
		}
		r := newRoom()
		log.Println("Created new room")
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
