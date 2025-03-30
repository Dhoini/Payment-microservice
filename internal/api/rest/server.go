package rest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Dhoini/Payment-microservice/config"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Server представляет HTTP сервер
type Server struct {
	httpServer *http.Server
	router     *gin.Engine
	log        *logger.Logger
	cfg        *config.Config
}

// NewServer создает новый HTTP сервер
func NewServer(router *gin.Engine, cfg *config.Config, log *logger.Logger) *Server {
	return &Server{
		router: router,
		log:    log,
		cfg:    cfg,
	}
}

// Start запускает HTTP сервер
func (s *Server) Start() error {
	// Создание HTTP сервера
	port := s.cfg.Server.Port
	s.httpServer = &http.Server{
		Addr:         ":" + port,
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запуск сервера
	s.log.Info("Starting server on port %s", port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown выполняет graceful shutdown сервера
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Server is shutting down...")
	return s.httpServer.Shutdown(ctx)
}
