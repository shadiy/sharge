package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Session management (simple in-memory, for demo)
var (
	sessions   = map[string]string{} // sessionID -> username
	sessionsMu sync.Mutex
)

func setSession(w http.ResponseWriter, username string) string {
	sid := fmt.Sprintf("sess_%d_%d", time.Now().UnixNano(), rand.Int())
	sessionsMu.Lock()
	sessions[sid] = username
	sessionsMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "session", Value: sid, Path: "/", HttpOnly: true})
	return sid
}

func getSession(r *http.Request) (string, bool) {
	c, err := r.Cookie("session")
	if err != nil {
		return "", false
	}

	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	u, ok := sessions[c.Value]
	return u, ok
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session")
	if err == nil {
		sessionsMu.Lock()
		delete(sessions, c.Value)
		sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
}
