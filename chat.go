package main

import (
	"database/sql"
	"log"
	"net/http"
	"text/template"

	"github.com/gorilla/sessions"
	_ "github.com/gorilla/sessions"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	_ "golang.org/x/crypto/bcrypt"
)

// caching templates
var templates = template.Must(template.ParseGlob("templates/*.html"))

var store = sessions.NewCookieStore([]byte("super-secret-key"))

const (
	file     string = "users.db"
	createDb string = `
        CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT NOT NULL,
        password TEXT NOT NULL
        );`
)

type User struct {
	ID       int
	username string
	password string
}

type UsersDbRow struct {
	ID int
	User
}

func initDatabase(db *sql.DB) {
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
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
	if _, err := stmt.Exec(user.username, user.password); err != nil {
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
	err := db.QueryRow("SELECT id, username, password FROM users WHERE id = ?;", id).Scan(&user.ID, &user.username, &user.password)
	if err != nil {
		log.Fatal(err)
	}
}

func (user *User) get(username string, db *sql.DB) {
	err := db.QueryRow("SELECT id, username, password FROM users WHERE username = ?;", username).Scan(&user.ID, &user.username, &user.password)
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
		ok := checkPasswordHash(password, user.password)
		if !ok {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		session.Values["username"] = user.username
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
			user.username = username
			user.password, err = hashPassword(password1)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				log.Fatal(err)
			}
			user.add(db)

			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	err := templates.ExecuteTemplate(w, "registration.html", nil)
	if err != nil {
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
	if !ok {
		username = "anon"
	}
	log.Println(username)
	err = templates.ExecuteTemplate(w, "chat.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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
