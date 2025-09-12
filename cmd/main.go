package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/JanArsMAI/GolangWorkerPool/internal/config"
	workerpool "github.com/JanArsMAI/GolangWorkerPool/internal/worker_pool"
)

type JobType int

var (
	TypeFirst  JobType = 1
	TypeSecond JobType = 2
)

type Job struct {
	JobId   int     `json:"job_id"`
	JobType JobType `json:"job_type"`
}

type JobResult struct {
	JobId     int       `json:"job_id"`
	JobType   JobType   `json:"job_type"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
	Error     string    `json:"error,omitempty"`
}

func main() {
	cfg := config.MustLoad()

	// cоздаем директорию если не существует
	if err := os.MkdirAll(cfg.Path, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	// cоздаем файл для результатов
	resultFile, err := os.Create(cfg.Path + "/results.json")
	if err != nil {
		log.Fatalf("Failed to create results file: %v", err)
	}
	defer resultFile.Close()

	// Создаем encoder для записи JSON результатов
	encoder := json.NewEncoder(resultFile)
	encoder.SetIndent("", "  ")

	resultChan := make(chan JobResult, 100)
	// горутина для записи результатов в файл
	go func() {
		for result := range resultChan {
			if err := encoder.Encode(result); err != nil {
				log.Printf("Failed to write result to file: %v", err)
			}
		}
	}()
	fmt.Printf("Starting worker pool with %d workers and queue size %d\n",
		cfg.WorkerPool.NumberOfWorkers, cfg.WorkerPool.QueueSize)
	ctx := context.Background()
	pool := workerpool.NewWorkerPool(
		ctx,
		cfg.WorkerPool.NumberOfWorkers,
		cfg.WorkerPool.QueueSize,
		func() {
			log.Println("Task completed")
		},
	)
	defer pool.Stop()

	// Читаем задачи из файла
	jobs, err := readJobsFromFile(cfg.Path + "/jobs.json")
	if err != nil {
		log.Printf("Failed to read jobs from file, using default jobs: %v", err)
		// cоздаем default jobs если файл не найден
		jobs = generateDefaultJobs(15)
	}

	firstTypeHook := func(job Job) {
		result := JobResult{
			JobId:     job.JobId,
			JobType:   job.JobType,
			Status:    "completed",
			StartTime: time.Now(),
		}
		resultChan <- result
	}

	secondTypeHook := func(job Job) {
		result := JobResult{
			JobId:     job.JobId,
			JobType:   job.JobType,
			Status:    "completed",
			StartTime: time.Now(),
		}
		resultChan <- result
	}

	// добавляем задачи в пул
	for _, job := range jobs {
		err := pool.Submit(createTask(job, firstTypeHook, secondTypeHook, resultChan))
		if err != nil {
			result := JobResult{
				JobId:   job.JobId,
				JobType: job.JobType,
				Status:  "failed",
				Error:   err.Error(),
			}
			resultChan <- result

			switch err {
			case workerpool.ErrPoolStopped:
				log.Printf("Failed to submit job %d: %v", job.JobId, err)
			case workerpool.ErrQueueFull:
				log.Printf("Failed to submit job %d: %v", job.JobId, err)
			default:
				log.Printf("Failed to submit job %d: unknown error %v", job.JobId, err)
			}
		} else {
			log.Printf("Submitted job %d (Type: %d)", job.JobId, job.JobType)
		}
		time.Sleep(50 * time.Millisecond)
	}

	fmt.Println("All jobs submitted. Waiting for completion...")
	time.Sleep(5 * time.Second)

	// Закрываем канал результатов
	close(resultChan)
	fmt.Printf("Application finished. Results saved to %s/results.json\n", cfg.Path)
}

// функция считывания джоб из файла
func readJobsFromFile(filename string) ([]Job, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var jobs []Job
	if err := json.Unmarshal(file, &jobs); err != nil {
		return nil, err
	}

	return jobs, nil
}

// функция генерации джоб(на случай, если путь не указан)
func generateDefaultJobs(count int) []Job {
	var jobs []Job
	for i := 1; i <= count; i++ {
		jobType := TypeFirst
		if i%2 == 0 {
			jobType = TypeSecond
		}
		jobs = append(jobs, Job{
			JobId:   i,
			JobType: jobType,
		})
	}
	return jobs
}

// функция создания задачи, она передаётся в worker pool
func createTask(job Job, firstHook, secondHook func(Job), resultChan chan<- JobResult) func() {
	return func() {
		startTime := time.Now()

		// Создаем результат
		result := JobResult{
			JobId:     job.JobId,
			JobType:   job.JobType,
			Status:    "processing",
			StartTime: startTime,
		}
		resultChan <- result

		// Обработка задачи
		err := processJob(job)

		endTime := time.Now()
		result.EndTime = endTime
		result.Duration = endTime.Sub(startTime).String()

		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
		} else {
			result.Status = "completed"
			// Вызываем хук в зависимости от типа задачи
			switch job.JobType {
			case TypeFirst:
				firstHook(job)
			case TypeSecond:
				secondHook(job)
			default:
				log.Printf("Unknown job type: %d", job.JobType)
			}
		}

		resultChan <- result
	}
}

// функция обработки джоб(таск) - именно эта функция выполняется внутри таски
func processJob(job Job) error {
	log.Printf("Processing job %d (Type: %d)...", job.JobId, job.JobType)

	var processingTime time.Duration
	switch job.JobType {
	case TypeFirst:
		processingTime = time.Duration(800+job.JobId*50) * time.Millisecond
	case TypeSecond:
		processingTime = 500 * time.Millisecond
	default:
		processingTime = 1 * time.Second
	}

	time.Sleep(processingTime)
	log.Printf("Finished job %d (took %v)", job.JobId, processingTime)
	return nil
}
