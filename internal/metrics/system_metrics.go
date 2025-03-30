package metrics

import (
	"runtime"
	"time"

	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SystemMetrics интерфейс для системных метрик
type SystemMetrics interface {
	RecordGoroutines()
	RecordMemory()
	StartRecording(interval time.Duration)
	Stop()
}

type systemMetrics struct {
	log          *logger.Logger
	goroutines   prometheus.Gauge
	memoryAlloc  prometheus.Gauge
	memoryTotal  prometheus.Gauge
	memorySystem prometheus.Gauge
	memoryGC     prometheus.Counter
	stopCh       chan struct{}
}

// NewSystemMetrics создает новые системные метрики
func NewSystemMetrics(registry *prometheus.Registry, log *logger.Logger) SystemMetrics {
	goroutines := promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Name: "system_goroutines",
			Help: "Current number of goroutines",
		},
	)

	memoryAlloc := promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Name: "system_memory_alloc_bytes",
			Help: "Currently allocated memory in bytes",
		},
	)

	memoryTotal := promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Name: "system_memory_total_alloc_bytes",
			Help: "Total memory allocation in bytes",
		},
	)

	memorySystem := promauto.With(registry).NewGauge(
		prometheus.GaugeOpts{
			Name: "system_memory_system_bytes",
			Help: "Total memory obtained from system in bytes",
		},
	)

	memoryGC := promauto.With(registry).NewCounter(
		prometheus.CounterOpts{
			Name: "system_memory_gc_total",
			Help: "Total number of garbage collections",
		},
	)

	return &systemMetrics{
		log:          log,
		goroutines:   goroutines,
		memoryAlloc:  memoryAlloc,
		memoryTotal:  memoryTotal,
		memorySystem: memorySystem,
		memoryGC:     memoryGC,
		stopCh:       make(chan struct{}),
	}
}

// RecordGoroutines записывает количество горутин
func (m *systemMetrics) RecordGoroutines() {
	m.goroutines.Set(float64(runtime.NumGoroutine()))
}

// RecordMemory записывает метрики памяти
func (m *systemMetrics) RecordMemory() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.memoryAlloc.Set(float64(memStats.Alloc))
	m.memoryTotal.Set(float64(memStats.TotalAlloc))
	m.memorySystem.Set(float64(memStats.Sys))
	m.memoryGC.Add(float64(memStats.NumGC))
}

// StartRecording начинает запись метрик с заданным интервалом
func (m *systemMetrics) StartRecording(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.RecordGoroutines()
				m.RecordMemory()
			case <-m.stopCh:
				return
			}
		}
	}()
	m.log.Info("System metrics recording started with interval %s", interval)
}

// Stop останавливает запись метрик
func (m *systemMetrics) Stop() {
	close(m.stopCh)
	m.log.Info("System metrics recording stopped")
}
