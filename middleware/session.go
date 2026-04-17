package middleware

import (
	"net/http"

	"github.com/gorilla/sessions"

	"go-crawler-notification/models"
)

var store = sessions.NewCookieStore([]byte("crawler-secret-key-change-in-production"))

func init() {
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
	}
}

// GetSession returns the gorilla session
func GetSession(r *http.Request) *sessions.Session {
	session, _ := store.Get(r, "crawler-session")
	return session
}

// SetFlash stores a flash message in session
func SetFlash(w http.ResponseWriter, r *http.Request, msgType, message string) {
	session := GetSession(r)
	session.AddFlash(message, msgType)
	session.Save(r, w)
}

// GetFlashes retrieves and clears flash messages
func GetFlashes(w http.ResponseWriter, r *http.Request) []models.FlashMessage {
	session := GetSession(r)
	var flashes []models.FlashMessage
	for _, t := range []string{"notice", "alert"} {
		for _, f := range session.Flashes(t) {
			if msg, ok := f.(string); ok {
				flashes = append(flashes, models.FlashMessage{Type: t, Message: msg})
			}
		}
	}
	session.Save(r, w)
	return flashes
}
