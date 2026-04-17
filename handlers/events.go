package handlers

import (
	"net/http"

	eventComponents "go-crawler-notification/components/events"
)

func EventsIndex(w http.ResponseWriter, r *http.Request) {
	props := makeProps(w, r, "Events")
	events := EventLog.All()
	eventComponents.Index(props, events).Render(r.Context(), w)
}
