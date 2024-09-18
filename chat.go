package main

import (
	"database/sql"
	"log"
	"net/http"
	"text/template"

	"github.com/gorilla/sessions"
	_ "github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// caching templates
var (
	templates = template.Must(template.ParseGlob("templates/*.html"))
	store     = sessions.NewCookieStore([]byte("super-secret-key"))
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan Message)
)

const (
	file     string = "users.db"
	createDb string = `
        CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT NOT NULL,
        password TEXT NOT NULL
        );`
)

type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type User struct {
	ID       int
	Username string
	Password string
}

func initDatabase(db *sql.DB) {
	res, err := db.Exec(createDb)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(res)
}

func (user User) add(db *sql.DB) {
	stmt, err := db.Prepare("INSERT INTO users(username, password) VALUES(?, ?);")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := stmt.Exec(user.Username, user.Password); err != nil {
		log.Fatal(err)
	}
}

func (user User) delete(db *sql.DB) {
	stmt, err := db.Prepare("DELETE FROM users WHERE id = ?;")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := stmt.Exec(user.ID); err != nil {
		log.Fatal(err)
	}
}

func (user *User) getById(id int, db *sql.DB) {
	err := db.QueryRow("SELECT id, username, password FROM users WHERE id = ?;", id).Scan(&user.ID, &user.Username, &user.Password)
	if err != nil {
		log.Fatal(err)
	}
}

func (user *User) get(username string, db *sql.DB) {
	err := db.QueryRow("SELECT id, username, password FROM users WHERE username = ?;", username).Scan(&user.ID, &user.Username, &user.Password)
	if err != nil {
		log.Fatal(err)
	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func loginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method == http.MethodPost {
		session, err := store.Get(r, "cookie-name")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Fatal(err)
		}
		user := &User{}
		username := r.FormValue("username")
		password := r.FormValue("password")
		user.get(username, db)
		ok := checkPasswordHash(password, user.Password)
		if !ok {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		session.Values["username"] = user.Username
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	err := templates.ExecuteTemplate(w, "login.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "cookie-name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
	}
	delete(session.Values, "username")
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func registerHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method == http.MethodPost {
		var err error
		user := &User{}
		username := r.FormValue("username")
		password1 := r.FormValue("password1")
		password2 := r.FormValue("password2")
		if password1 == password2 {
			user.Username = username
			user.Password, err = hashPassword(password1)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				log.Fatal(err)
			}
			user.add(db)

			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	if err := templates.ExecuteTemplate(w, "registration.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
	}
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "cookie-name")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatal(err)
	}
	username, ok := session.Values["username"].(string)
	var data User
	if ok {
		data.Username = username
	} else {
		data.Username = "anon"
	}
	log.Println("Entered as:", username)
	if err = templates.ExecuteTemplate(w, "chat.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
	}
}

func connectionHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	clients[conn] = true

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println(err)
			delete(clients, conn)
			return
		}
		broadcast <- msg

	}
}

func handleMessages() {
	for {
		msg := <-broadcast

		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Println(err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func main() {
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	initDatabase(db)

	fs := http.FileServer(http.Dir("static/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/ws", connectionHandler)
	go handleMessages()
	http.HandleFunc("/", chatHandler)
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		loginHandler(w, r, db)
	})

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		registerHandler(w, r, db)
	})

	http.HandleFunc("/logout", logoutHandler)

	log.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
