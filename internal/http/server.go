package httpadapter

import (
	"net/http"

	api "github.com/EkLeRchEk/template/internal/generated"
	createhandler "github.com/EkLeRchEk/template/internal/http/trip/create"
	finishhandler "github.com/EkLeRchEk/template/internal/http/trip/finish"
	gethandler "github.com/EkLeRchEk/template/internal/http/trip/get"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	router *chi.Mux
	create *createhandler.Handler
	get    *gethandler.Handler
	finish *finishhandler.Handler
	db     *pgxpool.Pool
}

// Проверяем на этапе компиляции реализацию OpenAPI-интерфейса.
var _ api.ServerInterface = (*Server)(nil)

func New(
	db *pgxpool.Pool,
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
	}

	api.HandlerFromMux(s, s.router)

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
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"not_ready"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
