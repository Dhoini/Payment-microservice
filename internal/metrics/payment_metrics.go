package metrics

import (
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PaymentMetrics интерфейс для метрик платежей
type PaymentMetrics interface {
	IncPaymentCreated(currency string)
	IncPaymentCompleted(currency string)
	IncPaymentFailed(currency string)
	IncPaymentRefunded(currency string)
	ObservePaymentAmount(amount float64, currency string, status string)
}

type paymentMetrics struct {
	log             *logger.Logger
	paymentsCreated *prometheus.CounterVec
	paymentsStatus  *prometheus.CounterVec
	paymentsAmount  *prometheus.HistogramVec
}

// NewPaymentMetrics создает новые метрики платежей
func NewPaymentMetrics(registry *prometheus.Registry, log *logger.Logger) PaymentMetrics {
	paymentsCreated := promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_created_total",
			Help: "The total number of created payments",
		},
		[]string{"currency"},
	)

	paymentsStatus := promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_status_total",
			Help: "The total number of payments by status",
		},
		[]string{"status", "currency"},
	)

	paymentsAmount := promauto.With(registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payments_amount",
			Help:    "Payment amounts distribution",
			Buckets: prometheus.ExponentialBuckets(10, 10, 5), // 10, 100, 1000, 10000, 100000
		},
		[]string{"currency", "status"},
	)

	return &paymentMetrics{
		log:             log,
		paymentsCreated: paymentsCreated,
		paymentsStatus:  paymentsStatus,
		paymentsAmount:  paymentsAmount,
	}
}

// IncPaymentCreated увеличивает счетчик созданных платежей
func (m *paymentMetrics) IncPaymentCreated(currency string) {
	m.paymentsCreated.WithLabelValues(currency).Inc()
}

// IncPaymentCompleted увеличивает счетчик завершенных платежей
func (m *paymentMetrics) IncPaymentCompleted(currency string) {
	m.paymentsStatus.WithLabelValues("completed", currency).Inc()
}

// IncPaymentFailed увеличивает счетчик неудачных платежей
func (m *paymentMetrics) IncPaymentFailed(currency string) {
	m.paymentsStatus.WithLabelValues("failed", currency).Inc()
}

// IncPaymentRefunded увеличивает счетчик возвращенных платежей
func (m *paymentMetrics) IncPaymentRefunded(currency string) {
	m.paymentsStatus.WithLabelValues("refunded", currency).Inc()
}

// ObservePaymentAmount записывает сумму платежа
func (m *paymentMetrics) ObservePaymentAmount(amount float64, currency string, status string) {
	m.paymentsAmount.WithLabelValues(currency, status).Observe(amount)
}
