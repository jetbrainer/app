package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	areAlive []Checker
}
type Checker func() (isAlive bool)

func NewHealthHandler(areAlive ...Checker) Handler {
	return Handler{
		areAlive: areAlive}
}

func (h Handler) Register(r chi.Router) {
	r.Get("/health/live", alive(h.areAlive))
}

func alive(areAlive []Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for _, isAlive := range areAlive {
			if !isAlive() {
				AnswerWithJSONError(w, http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

type errorResponse struct {
	ErrorCode    int
	ErrorDetails string
}

func AnswerWithJSONError(w http.ResponseWriter, code int) {
	jsonResponse, err := json.Marshal(errorResponse{
		ErrorCode:    code,
		ErrorDetails: "error during request processing",
	})
	if err != nil {
		http.Error(w, fmt.Errorf("failed to marshal error response").Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err = w.Write(jsonResponse); err != nil {
		err = fmt.Errorf("failed to write error response to writer")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    time.Duration     `json:"uptime"`
	Services  map[string]string `json:"services"`
}

func (s *Service) GetHealthStatus() HealthStatus {
	services := make(map[string]string)

	if s.DB != nil {
		if s.checkDBAlive() {
			services["database"] = "healthy"
		} else {
			services["database"] = "unhealthy"
		}
	}

	return HealthStatus{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    time.Since(s.startTime),
		Services:  services,
	}
}
