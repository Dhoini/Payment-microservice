package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Dhoini/Payment-microservice/config"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Server gRPC сервер
type Server struct {
	grpcServer *grpc.Server
	log        *logger.Logger
	cfg        *config.Config
	listener   net.Listener
}

// NewServer создает новый gRPC сервер
func NewServer(cfg *config.Config, log *logger.Logger) *Server {
	// Опции для gRPC
	var opts []grpc.ServerOption

	// Настройки keepalive для gRPC
	kaParams := keepalive.ServerParameters{
		MaxConnectionIdle:     time.Minute * 5,  // Максимальное время простоя соединения
		MaxConnectionAge:      time.Hour,        // Максимальное время жизни соединения
		MaxConnectionAgeGrace: time.Minute * 5,  // Дополнительное время для завершения запросов при закрытии соединения
		Time:                  time.Minute * 2,  // Время между пингами для проверки активности
		Timeout:               time.Second * 20, // Таймаут после которого соединение закрывается если нет ответа на пинг
	}

	opts = append(opts, grpc.KeepaliveParams(kaParams))

	// Настройка TLS, если необходимо
	if cfg.GRPC.UseTLS {
		creds, err := credentials.NewServerTLSFromFile(cfg.GRPC.CertFile, cfg.GRPC.KeyFile)
		if err != nil {
			log.Fatal("Failed to load TLS credentials: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// Создаем gRPC сервер с опциями
	grpcServer := grpc.NewServer(opts...)

	return &Server{
		grpcServer: grpcServer,
		log:        log,
		cfg:        cfg,
	}
}

// RegisterServices регистрирует все gRPC сервисы
func (s *Server) RegisterServices(paymentService PaymentServiceServer, customerService CustomerServiceServer, subscriptionService SubscriptionServiceServer) {
	// Регистрируем сервисы
	RegisterPaymentServiceServer(s.grpcServer, paymentService)
	RegisterCustomerServiceServer(s.grpcServer, customerService)
	RegisterSubscriptionServiceServer(s.grpcServer, subscriptionService)

	// Включаем reflection для удобства отладки (например, с помощью grpcurl)
	reflection.Register(s.grpcServer)
}

// Start запускает gRPC сервер
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.cfg.GRPC.Host, s.cfg.GRPC.Port)
	s.log.Info("Starting gRPC server on %s", addr)

	// Создаем слушателя TCP
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

	// Запускаем gRPC сервер
	if err := s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop останавливает gRPC сервер
func (s *Server) Stop() {
	s.log.Info("Stopping gRPC server")
	s.grpcServer.GracefulStop()
	if s.listener != nil {
		s.listener.Close()
	}
}

// PaymentServiceServer интерфейс сервиса платежей
type PaymentServiceServer interface {
	CreatePayment(context.Context, *CreatePaymentRequest) (*PaymentResponse, error)
	GetPayment(context.Context, *GetPaymentRequest) (*PaymentResponse, error)
	ListPayments(context.Context, *ListPaymentsRequest) (*ListPaymentsResponse, error)
	UpdatePaymentStatus(context.Context, *UpdatePaymentStatusRequest) (*PaymentResponse, error)
	ListPaymentsByCustomer(context.Context, *ListPaymentsByCustomerRequest) (*ListPaymentsResponse, error)
	RefundPayment(context.Context, *RefundPaymentRequest) (*PaymentResponse, error)
}

// CustomerServiceServer интерфейс сервиса клиентов
type CustomerServiceServer interface {
	CreateCustomer(context.Context, *CreateCustomerRequest) (*CustomerResponse, error)
	GetCustomer(context.Context, *GetCustomerRequest) (*CustomerResponse, error)
	ListCustomers(context.Context, *ListCustomersRequest) (*ListCustomersResponse, error)
	UpdateCustomer(context.Context, *UpdateCustomerRequest) (*CustomerResponse, error)
	DeleteCustomer(context.Context, *DeleteCustomerRequest) (*DeleteCustomerResponse, error)
}

// SubscriptionServiceServer интерфейс сервиса подписок
type SubscriptionServiceServer interface {
	CreateSubscription(context.Context, *CreateSubscriptionRequest) (*SubscriptionResponse, error)
	GetSubscription(context.Context, *GetSubscriptionRequest) (*SubscriptionResponse, error)
	ListSubscriptions(context.Context, *ListSubscriptionsRequest) (*ListSubscriptionsResponse, error)
	ListSubscriptionsByCustomer(context.Context, *ListSubscriptionsByCustomerRequest) (*ListSubscriptionsResponse, error)
	CancelSubscription(context.Context, *CancelSubscriptionRequest) (*SubscriptionResponse, error)
	PauseSubscription(context.Context, *PauseSubscriptionRequest) (*SubscriptionResponse, error)
	ResumeSubscription(context.Context, *ResumeSubscriptionRequest) (*SubscriptionResponse, error)
	CreateSubscriptionPlan(context.Context, *CreateSubscriptionPlanRequest) (*SubscriptionPlanResponse, error)
	GetSubscriptionPlan(context.Context, *GetSubscriptionPlanRequest) (*SubscriptionPlanResponse, error)
	ListSubscriptionPlans(context.Context, *ListSubscriptionPlansRequest) (*ListSubscriptionPlansResponse, error)
	UpdateSubscriptionPlan(context.Context, *UpdateSubscriptionPlanRequest) (*SubscriptionPlanResponse, error)
	DeleteSubscriptionPlan(context.Context, *DeleteSubscriptionPlanRequest) (*DeleteSubscriptionPlanResponse, error)
}

// Типы запросов и ответов, необходимые для интерфейсов
// Эти типы будут определены при генерации кода из proto-файлов

// CreatePaymentRequest запрос на создание платежа
type CreatePaymentRequest struct{}

// PaymentResponse ответ с платежом
type PaymentResponse struct{}

// GetPaymentRequest запрос на получение платежа
type GetPaymentRequest struct{}

// ListPaymentsRequest запрос на получение списка платежей
type ListPaymentsRequest struct{}

// ListPaymentsResponse ответ со списком платежей
type ListPaymentsResponse struct{}

// UpdatePaymentStatusRequest запрос на обновление статуса платежа
type UpdatePaymentStatusRequest struct{}

// ListPaymentsByCustomerRequest запрос на получение платежей клиента
type ListPaymentsByCustomerRequest struct{}

// RefundPaymentRequest запрос на возврат платежа
type RefundPaymentRequest struct{}

// CreateCustomerRequest запрос на создание клиента
type CreateCustomerRequest struct{}

// CustomerResponse ответ с клиентом
type CustomerResponse struct{}

// GetCustomerRequest запрос на получение клиента
type GetCustomerRequest struct{}

// ListCustomersRequest запрос на получение списка клиентов
type ListCustomersRequest struct{}

// ListCustomersResponse ответ со списком клиентов
type ListCustomersResponse struct{}

// UpdateCustomerRequest запрос на обновление клиента
type UpdateCustomerRequest struct{}

// DeleteCustomerRequest запрос на удаление клиента
type DeleteCustomerRequest struct{}

// DeleteCustomerResponse ответ на удаление клиента
type DeleteCustomerResponse struct{}

// CreateSubscriptionRequest запрос на создание подписки
type CreateSubscriptionRequest struct{}

// SubscriptionResponse ответ с подпиской
type SubscriptionResponse struct{}

// GetSubscriptionRequest запрос на получение подписки
type GetSubscriptionRequest struct{}

// ListSubscriptionsRequest запрос на получение списка подписок
type ListSubscriptionsRequest struct{}

// ListSubscriptionsResponse ответ со списком подписок
type ListSubscriptionsResponse struct{}

// ListSubscriptionsByCustomerRequest запрос на получение подписок клиента
type ListSubscriptionsByCustomerRequest struct{}

// CancelSubscriptionRequest запрос на отмену подписки
type CancelSubscriptionRequest struct{}

// PauseSubscriptionRequest запрос на приостановку подписки
type PauseSubscriptionRequest struct{}

// ResumeSubscriptionRequest запрос на возобновление подписки
type ResumeSubscriptionRequest struct{}

// CreateSubscriptionPlanRequest запрос на создание плана подписки
type CreateSubscriptionPlanRequest struct{}

// SubscriptionPlanResponse ответ с планом подписки
type SubscriptionPlanResponse struct{}

// GetSubscriptionPlanRequest запрос на получение плана подписки
type GetSubscriptionPlanRequest struct{}

// ListSubscriptionPlansRequest запрос на получение списка планов подписок
type ListSubscriptionPlansRequest struct{}

// ListSubscriptionPlansResponse ответ со списком планов подписок
type ListSubscriptionPlansResponse struct{}

// UpdateSubscriptionPlanRequest запрос на обновление плана подписки
type UpdateSubscriptionPlanRequest struct{}

// DeleteSubscriptionPlanRequest запрос на удаление плана подписки
type DeleteSubscriptionPlanRequest struct{}

// DeleteSubscriptionPlanResponse ответ на удаление плана подписки
type DeleteSubscriptionPlanResponse struct{}

// RegisterPaymentServiceServer регистрирует сервис платежей
func RegisterPaymentServiceServer(s *grpc.Server, srv PaymentServiceServer) {
	// Будет реализовано при генерации кода из proto
}

// RegisterCustomerServiceServer регистрирует сервис клиентов
func RegisterCustomerServiceServer(s *grpc.Server, srv CustomerServiceServer) {
	// Будет реализовано при генерации кода из proto
}

// RegisterSubscriptionServiceServer регистрирует сервис подписок
func RegisterSubscriptionServiceServer(s *grpc.Server, srv SubscriptionServiceServer) {
	// Будет реализовано при генерации кода из proto
}
