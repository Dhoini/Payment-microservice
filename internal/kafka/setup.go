package kafka

import (
	"context"
	"errors"
	"fmt"
	"github.com/Dhoini/Payment-microservice/pkg/logger" // Ваш логгер
	kafkaGo "github.com/segmentio/kafka-go"             // Kafka клиент
	"net"
	"strconv"
	"strings"
	"time"
)

// EnsureKafkaTopics проверяет и создает необходимые топики Kafka.
func EnsureKafkaTopics(brokers []string, log *logger.Logger) error {
	// Определяем необходимые топики и их конфигурацию
	requiredTopics := map[string]kafkaGo.TopicConfig{
		TopicSubscriptionCreated: {
			Topic:             TopicSubscriptionCreated,
			NumPartitions:     3,
			ReplicationFactor: 1,
		},
		TopicSubscriptionCancelled: {
			Topic:             TopicSubscriptionCancelled,
			NumPartitions:     2,
			ReplicationFactor: 1,
		},
		// "payment_events": { // Если нужен
		// 	Topic:             "payment_events",
		// 	NumPartitions:     1,
		// 	ReplicationFactor: 1,
		// },
	}

	log.Infow("Ensuring Kafka topics exist...", "topics", getTopicNames(requiredTopics))

	// --- Проверка адреса брокера ---
	if len(brokers) == 0 || brokers[0] == "" {
		log.Errorw("Kafka broker address is empty")
		return errors.New("kafka broker address is empty")
	}
	// Используем _ для неиспользуемой переменной host
	_, portStr, err := net.SplitHostPort(strings.TrimSpace(brokers[0]))
	if err != nil {
		log.Errorw("Invalid Kafka broker address format", "broker", brokers[0], "error", err)
		return fmt.Errorf("invalid broker address %s: %w", brokers[0], err)
	}
	_, err = strconv.Atoi(portStr)
	if err != nil {
		log.Errorw("Invalid Kafka broker port", "broker", brokers[0], "error", err)
		return fmt.Errorf("invalid broker port %s: %w", brokers[0], err)
	}
	// --- Конец проверки адреса ---

	// Подключаемся к первому брокеру для админских операций
	connCtx, cancelConn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelConn()

	conn, err := kafkaGo.DialLeader(connCtx, "tcp", brokers[0], "", 0) // Используем DialLeader для поиска контроллера
	if err != nil {
		log.Errorw("Failed to connect to Kafka broker for topic creation", "broker", brokers[0], "error", err)
		return fmt.Errorf("kafka connection failed: %w", err)
	}
	defer conn.Close()

	log.Debugw("Connected to Kafka controller", "address", conn.RemoteAddr().String())

	partitions, err := conn.ReadPartitions()
	if err != nil {
		log.Errorw("Failed to read partitions from Kafka", "error", err)
		return fmt.Errorf("kafka read partitions failed: %w", err)
	}

	existingTopics := make(map[string]bool)
	for _, p := range partitions {
		existingTopics[p.Topic] = true
	}
	log.Debugw("Found existing topics", "count", len(existingTopics))

	var topicsToCreate []kafkaGo.TopicConfig // Можно так для избежания варнинга
	for topicName, config := range requiredTopics {
		if !existingTopics[topicName] {
			log.Infow("Topic needs to be created", "topic", topicName)
			topicsToCreate = append(topicsToCreate, config)
		} else {
			log.Debugw("Topic already exists", "topic", topicName)
		}
	}

	if len(topicsToCreate) > 0 {
		log.Infow("Attempting to create topics...", "count", len(topicsToCreate))

		err = conn.CreateTopics(topicsToCreate...)

		if err != nil {
			// --- ИСПРАВЛЕННАЯ ПРОВЕРКА ОШИБКИ ---
			if errors.Is(err, kafkaGo.TopicAlreadyExists) {
				log.Warnw("One or more topics already existed during creation attempt", "topics", getTopicNamesFromConfig(topicsToCreate))
				// Это не фатальная ошибка, обнуляем ее
				err = nil
			} else {
				// Другая ошибка при создании
				log.Errorw("Failed to create topics", "error", err, "topics", getTopicNamesFromConfig(topicsToCreate))
				// Возвращаем исходную ошибку, обернутую
				return fmt.Errorf("kafka create topics failed: %w", err)
			}
			// --- Конец исправления ---
		}

		// Если после проверки на TopicAlreadyExists ошибка все еще есть (т.е. это была другая ошибка)
		if err != nil {
			return err // Возвращаем ее
		}
		// Логируем успех только если реально что-то создавали и не было ошибки
		log.Infow("Successfully created or verified topics", "topics", getTopicNamesFromConfig(topicsToCreate))

	} else {
		log.Infow("All required topics already exist.")
	}

	return nil // Ошибок не было
}

// --- Вспомогательные функции getTopicNames и getTopicNamesFromConfig остаются без изменений ---
// getTopicNames (без изменений)
func getTopicNames(topicMap map[string]kafkaGo.TopicConfig) []string {
	names := make([]string, 0, len(topicMap))
	for name := range topicMap {
		names = append(names, name)
	}
	return names
}

// getTopicNamesFromConfig (без изменений)
func getTopicNamesFromConfig(topicConfigs []kafkaGo.TopicConfig) []string {
	names := make([]string, 0, len(topicConfigs))
	for _, tc := range topicConfigs {
		names = append(names, tc.Topic)
	}
	return names
}
