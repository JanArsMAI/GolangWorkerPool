package workerpool

import (
	"context"
	"sync"
	"testing"
	"time"
)

// тест создания воркер пула
func TestNewWorkerPool(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 2, 10, nil)

	if pool == nil {
		t.Error("Expected pool to be created, got nil")
	}

	// Проверяем что пул не остановлен при создании
	if pool.isStopped {
		t.Error("Pool should not be stopped after creation")
	}
}

// тест на создание задачи
func TestSubmitTask(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 1, 1, nil)
	defer pool.Stop()
	var wg sync.WaitGroup
	wg.Add(1)
	err := pool.Submit(func() {
		defer wg.Done()
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	wg.Wait()
}

// тест на добавление задачи в остановленный пул
func TestSubmitToStoppedPool(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 1, 1, nil)
	pool.Stop()
	err := pool.Submit(func() {})
	if err != ErrPoolStopped {
		t.Errorf("Expected ErrPoolStopped, got %v", err)
	}
}

// попытка добавить в полную очередь задачу
func TestQueueFull(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 1, 1, nil)
	defer pool.Stop()

	//первая задача - должна добавиться
	err := pool.Submit(func() {
		time.Sleep(100 * time.Millisecond)
	})

	if err != nil {
		t.Errorf("First submit should succeed, got %v", err)
	}

	//вторая задача - должна получить ошибку переполнения
	err = pool.Submit(func() {})

	if err != ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}
}

// тест функции Stop и ожидания завершения всех задач из очереди
func TestStopWaitsForCompletion(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 1, 10, nil)

	var taskCompleted bool
	var mu sync.Mutex

	err := pool.Submit(func() {
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		taskCompleted = true
		mu.Unlock()
	})

	if err != nil {
		t.Errorf("Submit failed: %v", err)
	}

	// останавливаем пул - он должен дождаться выполнения задачи
	pool.Stop()

	mu.Lock()
	if !taskCompleted {
		t.Error("Stop should wait for task completion")
	}
	mu.Unlock()
}

// тест на нескольких воркеров
func TestMultipleWorkers(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 3, 10, nil)
	defer pool.Stop()

	var mu sync.Mutex
	completedTasks := make(map[int]bool)
	var wg sync.WaitGroup

	// запускаем несколько задач
	for i := 0; i < 5; i++ {
		wg.Add(1)
		taskID := i

		pool.Submit(func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			mu.Lock()
			completedTasks[taskID] = true
			mu.Unlock()
		})
	}
	wg.Wait()
	if len(completedTasks) != 5 {
		t.Errorf("Expected 5 completed tasks, got %d", len(completedTasks))
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewWorkerPool(ctx, 1, 10, nil)
	// Отменяем контекст через канселфанк
	cancel()

	// Даем время на обработку отмены
	time.Sleep(10 * time.Millisecond)

	// воркер пул должен быть в состоянии принимать задачи пока не вызван Stop
	err := pool.Submit(func() {})
	if err != nil {
		t.Errorf("Pool should accept tasks until Stop is called, got %v", err)
	}

	pool.Stop()
}

func TestConcurrentSubmit(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 3, 100, nil)
	defer pool.Stop()

	var wg sync.WaitGroup
	const numTasks = 50

	wg.Add(numTasks)

	// Concurrent submission
	for i := 0; i < numTasks; i++ {
		go func(taskNum int) {
			defer wg.Done()

			err := pool.Submit(func() {
				time.Sleep(time.Duration(taskNum%10) * time.Millisecond)
			})

			if err != nil {
				t.Errorf("Task %d failed to submit: %v", taskNum, err)
			}
		}(i)
	}

	wg.Wait()
}

// несколько стопов
func TestStopIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 1, 10, nil)

	pool.Stop()
	pool.Stop()
	pool.Stop()

	err := pool.Submit(func() {})
	if err != ErrPoolStopped {
		t.Errorf("Expected ErrPoolStopped after multiple stops, got %v", err)
	}
}

// тест на одного воркера
func TestTaskExecutionOrder(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 1, 10, nil)
	defer pool.Stop()

	var executionOrder []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(3)

	// Задачи должны выполняться в порядке отправки
	pool.Submit(func() {
		defer wg.Done()
		mu.Lock()
		executionOrder = append(executionOrder, 1)
		mu.Unlock()
	})

	pool.Submit(func() {
		defer wg.Done()
		mu.Lock()
		executionOrder = append(executionOrder, 2)
		mu.Unlock()
	})

	pool.Submit(func() {
		defer wg.Done()
		mu.Lock()
		executionOrder = append(executionOrder, 3)
		mu.Unlock()
	})

	wg.Wait()

	// проверяем порядок выполнения (для одного воркера должен сохраняться)
	expected := []int{1, 2, 3}
	for i, val := range executionOrder {
		if val != expected[i] {
			t.Errorf("Expected order %v, got %v", expected, executionOrder)
			break
		}
	}
}
