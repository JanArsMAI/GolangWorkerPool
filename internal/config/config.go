package config

import (
	"encoding/json"
	"log"
	"os"
)

const (
	DefaultQueueSize       = 100
	DefaultNumberOfWorkers = 10
	DefaultTimeout         = 30
)

type WorkerPoolConfig struct {
	QueueSize       int `json:"queueSize"`
	NumberOfWorkers int `json:"numberOfWorkers"`
}

type Config struct {
	WorkerPool WorkerPoolConfig `json:"workerPool"`
	Timeout    int              `json:"timeout"`
	Path       string           `json:"path"`
}

func MustLoad() *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH is not set")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	//считываем файл из .env
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("failed to read config file: %s", err)
	}

	var cfg Config
	if err := json.Unmarshal(configFile, &cfg); err != nil {
		log.Fatalf("failed to parse config JSON: %s", err)
	}

	//если пути для логов нет, то завершаем работу
	if cfg.Path == "" {
		log.Fatal("No path in configs")
	}
	//ставим константные значения, в случае когда поля пустые
	if cfg.WorkerPool.QueueSize <= 0 {
		cfg.WorkerPool.QueueSize = DefaultQueueSize
		log.Printf("Using default QueueSize: %d\n", cfg.WorkerPool.QueueSize)
	}
	if cfg.WorkerPool.NumberOfWorkers <= 0 {
		cfg.WorkerPool.NumberOfWorkers = DefaultNumberOfWorkers
		log.Printf("Using default NumberOfWorkers: %d\n", cfg.WorkerPool.NumberOfWorkers)
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
		log.Printf("Using default Timeout: %d\n", cfg.Timeout)
	}
	return &cfg
}
