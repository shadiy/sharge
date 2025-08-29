package main

import (
	"html/template"
	"net/http"
	"strings"
)

// Login page handler
func loginHandler() http.HandlerFunc {
	tpl := template.Must(template.New("login").Parse(loginHTML))
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tpl.Execute(w, nil)
			return
		}

		if r.Method == http.MethodPost {
			userPassword := r.FormValue("password")

			if userPassword != *password {
				tpl.Execute(w, map[string]string{"Error": "Invalid password"})
				return
			}

			ipAddress := strings.Split(r.RemoteAddr, ":")[0]

			setSession(w, ipAddress)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Middleware to require login
func requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := getSession(r); ok && u != "" {
			next.ServeHTTP(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}
