package app

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"google.golang.org/grpc"
)

type GRPCServerOption struct {
	address string
}

func (w GRPCServerOption) Apply(s *Service) error {
	grpcSrv := grpc.NewServer()

	s.GRPCServers = append(s.GRPCServers, &GRPCServer{
		server: grpcSrv, address: w.address,
	})
	return nil
}
func WithGRPCServer(address string) Option {
	return GRPCServerOption{address: address}
}

type TechHTTPServerOption struct {
	address string
}

func (w TechHTTPServerOption) Apply(s *Service) error {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)

	// adding pprof routes
	r.Mount("/debug/pprof", pprofRoutes())

	// adding gometrics
	prometheusRegistry := prometheus.NewRegistry()
	prometheusRegistry.MustRegister(collectors.NewGoCollector())
	prometheusRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	NewTelemtryHandler(prometheusRegistry).Register(r)
	NewReadinessHandler(s.isReady).Register(r)
	NewHealthHandler(s.IsAlive).Register(r)

	s.HTTPServers = append(s.HTTPServers, &http.Server{
		Addr:           w.address,
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	})

	return nil
}

func pprofRoutes() http.Handler {
	router := chi.NewRouter()
	router.HandleFunc("/", pprof.Index)
	router.HandleFunc("/cmdline", pprof.Cmdline)
	router.HandleFunc("/symbol", pprof.Symbol)
	router.HandleFunc("/trace", pprof.Trace)
	router.HandleFunc("/profile", pprof.Profile)
	// Manually add support for paths not easily linked as above
	router.Handle("/goroutine", pprof.Handler("goroutine"))
	router.Handle("/allocs", pprof.Handler("allocs"))
	router.Handle("/mutex", pprof.Handler("mutex"))
	router.Handle("/heap", pprof.Handler("heap"))
	router.Handle("/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/block", pprof.Handler("block"))
	router.Handle("/vars", http.DefaultServeMux) // this might be tricky, test thoroughly

	return router
}

func WithTechHTTPServerOption(address string) Option {
	return TechHTTPServerOption{address: address}
}

type DBOption struct {
	cfg pgxpool.Config
}

func (w DBOption) Apply(s *Service) error {
	poolConfig, err := pgxpool.ParseConfig(w.cfg.ConnString())
	if err != nil {
		return err
	}

	p, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return err
	}
	s.DB = p
	return nil
}

func WithDB(cfg pgxpool.Config) Option {
	return DBOption{cfg: cfg}
}

type RedisOption struct {
}

func (w RedisOption) Apply(s *Service) error { // init redis

	return nil
}

func WithRedis() Option {
	return RedisOption{}
}
