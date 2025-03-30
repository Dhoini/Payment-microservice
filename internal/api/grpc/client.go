package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Client представляет gRPC клиент
type Client struct {
	conn *grpc.ClientConn
	log  *logger.Logger
}

// ClientOptions настройки для gRPC клиента
type ClientOptions struct {
	Address          string
	Timeout          time.Duration
	UseTLS           bool
	KeepAlive        bool
	KeepAliveTime    time.Duration
	KeepAliveTimeout time.Duration
}

// DefaultClientOptions возвращает настройки по умолчанию
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		Address:          "localhost:50051",
		Timeout:          time.Second * 10,
		UseTLS:           false,
		KeepAlive:        true,
		KeepAliveTime:    time.Minute,
		KeepAliveTimeout: time.Second * 20,
	}
}

// NewClient создает новый gRPC клиент
func NewClient(opts *ClientOptions, log *logger.Logger) (*Client, error) {
	log.Debug("Creating gRPC client to %s", opts.Address)

	// Опции для подключения
	var dialOpts []grpc.DialOption

	// Настройка TLS
	if opts.UseTLS {
		// В реальном приложении здесь должна быть настройка TLS с сертификатами
		creds := credentials.NewTLS(&tls.Config{})
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Настройка KeepAlive
	if opts.KeepAlive {
		kacp := keepalive.ClientParameters{
			Time:                opts.KeepAliveTime,    // Интервал для пингов
			Timeout:             opts.KeepAliveTimeout, // Таймаут для пингов
			PermitWithoutStream: true,                  // Разрешить пинги без активных стримов
		}
		dialOpts = append(dialOpts, grpc.WithKeepaliveParams(kacp))
	}

	// Установка таймаута
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Создаем соединение
	conn, err := grpc.DialContext(ctx, opts.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	log.Info("Successfully connected to gRPC server at %s", opts.Address)

	return &Client{
		conn: conn,
		log:  log,
	}, nil
}

// Close закрывает соединение с gRPC сервером
func (c *Client) Close() error {
	if c.conn != nil {
		c.log.Info("Closing gRPC client connection")
		return c.conn.Close()
	}
	return nil
}

// Conn возвращает gRPC соединение
func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}

// NewPaymentServiceClient создает клиент для сервиса платежей
func (c *Client) NewPaymentServiceClient() PaymentServiceClient {
	return NewPaymentServiceClient(c.conn)
}

// NewCustomerServiceClient создает клиент для сервиса клиентов
func (c *Client) NewCustomerServiceClient() CustomerServiceClient {
	return NewCustomerServiceClient(c.conn)
}

// NewSubscriptionServiceClient создает клиент для сервиса подписок
func (c *Client) NewSubscriptionServiceClient() SubscriptionServiceClient {
	return NewSubscriptionServiceClient(c.conn)
}

// Интерфейсы клиентов, которые будут сгенерированы из .proto файлов
type PaymentServiceClient interface{}
type CustomerServiceClient interface{}
type SubscriptionServiceClient interface{}

// Функции, которые будут сгенерированы из .proto файлов
func NewPaymentServiceClient(cc grpc.ClientConnInterface) PaymentServiceClient {
	return nil // Будет заменено сгенерированным кодом
}

func NewCustomerServiceClient(cc grpc.ClientConnInterface) CustomerServiceClient {
	return nil // Будет заменено сгенерированным кодом
}

func NewSubscriptionServiceClient(cc grpc.ClientConnInterface) SubscriptionServiceClient {
	return nil // Будет заменено сгенерированным кодом
}
