package httpserver

import (
	"net/http"

	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/trip"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	router *chi.Mux
}

func New(db *pgxpool.Pool, service *trip.Service) *Server {
	router := chi.NewRouter()

	// Создаём HTTP-обработчик с доступом к сервису и БД.
	handler := NewHandler(service, db)

	// Регистрируем маршруты из OpenAPI.
	api.HandlerFromMux(handler, router)

	return &Server{
		router: router,
	}
}

func (s *Server) Handler() http.Handler {
	return s.router
}
