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

	// Настраиваем обработку ошибок OpenAPI.
	options := api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(
			w http.ResponseWriter,
			r *http.Request,
			err error,
		) {
			writeProblem(
				w,
				http.StatusBadRequest,
				"invalid_request",
				"Invalid request parameters",
			)
		},
	}

	// Регистрируем маршруты из OpenAPI.
	api.HandlerWithOptions(handler, options)

	return &Server{
		router: router,
	}
}

func (s *Server) Handler() http.Handler {
	return s.router
}
