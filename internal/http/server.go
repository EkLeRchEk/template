package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	api "github.com/EkLeRchEk/template/internal/generated"
	"github.com/EkLeRchEk/template/internal/http/domainerror"
	createhandler "github.com/EkLeRchEk/template/internal/http/trip/create"
	finishhandler "github.com/EkLeRchEk/template/internal/http/trip/finish"
	gethandler "github.com/EkLeRchEk/template/internal/http/trip/get"
)

// DBPinger описывает зависимость, необходимую для проверки БД.
type DBPinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	router *chi.Mux
	create *createhandler.Handler
	get    *gethandler.Handler
	finish *finishhandler.Handler
	db     DBPinger
	logger *slog.Logger
}

// Проверяем на этапе компиляции реализацию OpenAPI-интерфейса.
var _ api.ServerInterface = (*Server)(nil)

func New(
	db DBPinger,
	logger *slog.Logger,
	create *createhandler.Handler,
	get *gethandler.Handler,
	finish *finishhandler.Handler,
) *Server {
	s := &Server{
		router: chi.NewRouter(),
		create: create,
		get:    get,
		finish: finish,
		db:     db,
		logger: logger,
	}

	api.HandlerWithOptions(s, api.ChiServerOptions{
		BaseRouter: s.router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			var invalidParam *api.InvalidParamFormatError

			if errors.As(err, &invalidParam) {
				s.logger.WarnContext(
					r.Context(),
					"invalid request parameter",
					"parameter", invalidParam.ParamName,
					"error", err,
				)
			}

			domainerror.WriteProblem(
				w,
				http.StatusBadRequest,
				"invalid_request",
				"Invalid request parameters",
			)
		},
	})

	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}

// POST /api/v1/trips
func (s *Server) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	_ api.CreateTripParams,
) {
	s.create.Handle(w, r)
}

// GET /api/v1/trips/{tripId}
func (s *Server) GetTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	s.get.Handle(w, r, tripID)
}

// POST /api/v1/trips/{tripId}/finish
func (s *Server) FinishTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	s.finish.Handle(w, r, tripID)
}

// GET /health
func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// GET /ready

// Ready проверяет доступность PostgreSQL.
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		s.logger.ErrorContext(
			r.Context(),
			"database readiness check failed",
			"error", err,
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
