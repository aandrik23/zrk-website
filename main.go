package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	sessions = make(map[string]time.Time)
	mu       sync.Mutex
)

func main() {
	passwordHash := os.Getenv("ADMIN_PASSWORD_HASH")

	if passwordHash == "" {
		log.Fatal("ADMIN_PASSWORD_HASH environment variable is not set")
	}

	// Static files
	http.Handle("/css/",
		http.StripPrefix("/css/",
			http.FileServer(http.Dir("css")),
		),
	)

	http.Handle("/js/",
		http.StripPrefix("/js/",
			http.FileServer(http.Dir("js")),
		),
	)

	http.Handle("/assets/",
		http.StripPrefix("/assets/",
			http.FileServer(http.Dir("assets")),
		),
	)

	// Public website
	http.HandleFunc("/", homeHandler)

	// Authentication
	http.HandleFunc("/login", loginHandler(passwordHash))
	http.HandleFunc("/logout", logoutHandler)

	// Protected admin page
	http.HandleFunc("/admin", requireAuth(adminHandler))

	log.Println("Server running at http://localhost:8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, "index.html")
}

func loginHandler(passwordHash string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodGet {
			http.ServeFile(w, r, "templates/login.html")
			return
		}

		if r.Method != http.MethodPost {
			http.Error(
				w,
				"Method not allowed",
				http.StatusMethodNotAllowed,
			)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if username != "admin" {
			http.Error(w, "Λάθος στοιχεία σύνδεσης", http.StatusUnauthorized)
			return
		}

		err := bcrypt.CompareHashAndPassword(
			[]byte(passwordHash),
			[]byte(password),
		)

		if err != nil {
			http.Error(w, "Λάθος στοιχεία σύνδεσης", http.StatusUnauthorized)
			return
		}

		token, err := createSession()
		if err != nil {
			http.Error(
				w,
				"Server error",
				http.StatusInternalServerError,
			)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "admin_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   3600,
		})

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

func createSession() (string, error) {
	bytes := make([]byte, 32)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	token := hex.EncodeToString(bytes)

	mu.Lock()
	sessions[token] = time.Now().Add(time.Hour)
	mu.Unlock()

	return token, nil
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("admin_session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		mu.Lock()

		expiry, exists := sessions[cookie.Value]

		if exists && time.Now().After(expiry) {
			delete(sessions, cookie.Value)
			exists = false
		}

		mu.Unlock()

		if !exists {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin.html")
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {

	cookie, err := r.Cookie("admin_session")

	if err == nil {
		mu.Lock()
		delete(sessions, cookie.Value)
		mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
