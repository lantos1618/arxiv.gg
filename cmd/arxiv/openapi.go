package main

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var researchOpenAPI []byte

func (s *server) handleAPIOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, 405, APIResponse{Error: "use GET"})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write(researchOpenAPI)
}
