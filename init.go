package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Option interface {
	Apply(service *Service) error
}

type SubService interface {
	Ready() bool
	Name() string
	Close() error
}

type GRPCServer struct {
	address string
	server  *grpc.Server
}

type Service struct {
	Name        string
	ctx         context.Context
	GRPCServers []*GRPCServer
	HTTPServers []*http.Server
	DB          *pgxpool.Pool
	isReady     *atomic.Value
	ErrChan     chan error
	SubServices map[string]SubService
	sigHandler  SignalTrap
	startTime   time.Time
	version     string
}

func New(ctx context.Context, name string, options ...Option) (*Service, error) {
	isReady := &atomic.Value{}
	isReady.Store(false)

	s := &Service{
		Name:        name,
		ErrChan:     make(chan error),
		ctx:         ctx,
		isReady:     isReady,
		SubServices: make(map[string]SubService),
		sigHandler:  TermSignalTrap(),
	}

	for _, o := range options {
		if err := o.Apply(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Service) GetContext() context.Context {
	return s.ctx
}

func (s *Service) SetContext(ctx context.Context) {
	s.ctx = ctx
}

func (s *Service) AddHTTPServer(httpServer *http.Server) {
	s.HTTPServers = append(s.HTTPServers, httpServer)
}

func (s *Service) AddGRPCService(serverName string, service interface{}, description *grpc.ServiceDesc) error {
	for _, grpcServer := range s.GRPCServers {
		if grpcServer.address == serverName {
			grpcServer.server.RegisterService(description, service)
			log.Debug().Msgf("GRPC service registered. service - %s, server - %s", description.ServiceName, serverName)
			return nil
		}
	}
	return errors.New("gRPC server not found")
}

func (s *Service) IsAlive() bool {
	isGrpcAlive := true
	if s.GRPCServers != nil {
		isGrpcAlive = s.checkGRPCServerUp()
		if !isGrpcAlive {
			log.Debug().Msg("grpc servers not ready")
		}
	}

	areHTTPServersAlive := true
	for _, httpServer := range s.HTTPServers {
		if !s.checkHTTPServerUp(httpServer) {
			areHTTPServersAlive = false
		}
	}

	isDBAlive := true
	if s.DB != nil && !s.checkDBAlive() {
		isDBAlive = false
	}

	return isGrpcAlive && areHTTPServersAlive && isDBAlive
}

func (s *Service) Start() error {
	s.startTime = time.Now()
	log.Info().Time("start_time", s.startTime).Msg("service starting")

	ctx := s.GetContext()

	for _, httpServ := range s.HTTPServers {
		httpServ := httpServ
		go func() {
			log.Info().Msgf("started http server address %s", httpServ.Addr)
			defer log.Info().Msg("stopped http server")

			if err := httpServ.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.ErrChan <- fmt.Errorf("http: failed to serve %v", err)
			}
		}()
	}

	for _, grpcServer := range s.GRPCServers {
		grpcServer := grpcServer

		go func() {
			log.Info().Msgf("started grpc server address %s", grpcServer.address)
			defer log.Info().Msg("stopped grpc server")

			listener, err := net.Listen("tcp", grpcServer.address)
			if err != nil {
				s.ErrChan <- fmt.Errorf("failed to listenn %v", err)
				return
			}

			if err = grpcServer.server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				s.ErrChan <- fmt.Errorf("grpc: failed to serve %v", err)
			}
		}()
	}

	go s.Ready()

	{
		if err := s.sigHandler.Wait(ctx); err != nil && !errors.Is(err, ErrTermSig) {
			log.Error().Msgf("failed to caught signal %v", log.Err(err))
			return err
		}
		log.Info().Msg("termination signal received")
	}

	go func() {
		for err := range s.ErrChan {
			log.Error().Err(err).Msg("service error occurred")
		}
	}()

	return nil
}

func (s *Service) Stop() {
	log.Info().Msg("initiating graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, service := range s.SubServices {
		if err := service.Close(); err != nil {
			log.Error().Err(err).Str("service", service.Name()).Msg("failed to stop service")
		} else {
			log.Debug().Str("service", service.Name()).Msg("subservice stopped")
		}
	}

	for _, grpcServer := range s.GRPCServers {
		grpcServer.server.GracefulStop()
		log.Debug().Msg("grpc server stopped")
	}

	for _, httpServer := range s.HTTPServers {
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Str("addr", httpServer.Addr).Msg("failed to shutdown http server")
		} else {
			log.Debug().Str("addr", httpServer.Addr).Msg("http server stopped")
		}
	}

	if s.DB != nil {
		s.DB.Close()
		log.Debug().Msg("db connection closed")
	}

	close(s.ErrChan)

	log.Info().Msg("graceful shutdown completed")
}

func (s *Service) Ready() {
	areSubServicesReady := true
	for _, subService := range s.SubServices {
		if !subService.Ready() {
			log.Error().Msgf("subservice not ready subservice %s", subService.Name())
			areSubServicesReady = false
		}
		log.Info().Msgf("subservice is ready subservice %s", subService.Name())
	}

	isGRPCReady := true
	if s.GRPCServers != nil {
		isGRPCReady = s.checkGRPCServerUp()
		if !isGRPCReady {
			log.Error().Msg("grpc server not ready")
		}
	}

	areHTTPServersReady := true
	for _, httpServer := range s.HTTPServers {
		if !s.checkHTTPServerUp(httpServer) {
			areHTTPServersReady = false
		}
	}

	isDBReady := true
	if s.DB != nil && !s.checkDBAlive() {
		isDBReady = false
	}

	s.isReady.Swap(areSubServicesReady && isGRPCReady && areHTTPServersReady && isDBReady)
}
func (s *Service) checkHTTPServerUp(httpServer *http.Server) bool {
	err := errors.New("http server not ready")
	var conn net.Conn
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	for err != nil {
		if conn, err = net.DialTimeout("tcp", httpServer.Addr, 1*time.Second); err != nil {
			log.Debug().Msg(err.Error())
		}
	}
	log.Debug().Msg("http server ready")
	return true
}

func (s *Service) checkGRPCServerUp() bool {
	var conn *grpc.ClientConn
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	for _, server := range s.GRPCServers {
		var err error
		if conn, err = grpc.NewClient(server.address, grpc.WithTransportCredentials(insecure.NewCredentials())); err != nil {
			log.Debug().Msg(err.Error())
			return false
		}

		log.Debug().Msgf("grpc server ready %s", server.address)
	}
	return true
}

func (s *Service) checkDBAlive() bool {
	if s.DB == nil {
		return true
	}

	err := s.DB.Ping(s.ctx)
	if err != nil {
		log.Debug().Err(err).Msg("db is not ready")
		return false
	}

	log.Debug().Msg("db is ready")
	return true
}

func (s *Service) checkRedisAlive() bool {
	return false
}
