package handler

import (
	"net/http"

	"ego/platform/jwt"
	"ego/services/ai/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
	Chat(w http.ResponseWriter, r *http.Request)
	ExplainWord(w http.ResponseWriter, r *http.Request)
	ExplainGrammar(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	mux.HandleFunc("POST /chat", mw.Handle(h.Chat))
	mux.HandleFunc("POST /words/explain", mw.Handle(h.ExplainWord))
	mux.HandleFunc("POST /grammar/explain", mw.Handle(h.ExplainGrammar))
}
