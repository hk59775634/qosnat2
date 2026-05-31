package api

import (
	"log"
	"net/http"
)

// persistState saves state; when w is non-nil, writes a 500 JSON error on failure.
func (srv *Server) persistState(w http.ResponseWriter) bool {
	if err := srv.store.Save(); err != nil {
		if w != nil {
			writeSaveError(w, err)
		} else {
			log.Printf("persist state: %v", err)
		}
		return false
	}
	return true
}

// persistStateOrLog saves state for background/revert paths where no HTTP response exists.
func (srv *Server) persistStateOrLog(context string) bool {
	if err := srv.store.Save(); err != nil {
		log.Printf("%s: save failed: %v", context, err)
		return false
	}
	return true
}
