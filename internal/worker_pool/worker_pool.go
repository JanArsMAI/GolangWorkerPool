package workerpool

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrPoolStopped = errors.New("error. Worker pool is stopped")
	ErrQueueFull   = errors.New("error. Worker pool's queue is full")
)

type Pool interface {
	Submit(task func()) error
	Stop() error
}

type Task func()

type WorkerPool struct {
	taskQueue       chan Task          //очередь задач
	workerWaitGroup sync.WaitGroup     //wait group для управления воркерами
	mu              sync.RWMutex       //мьютекс для избежания race condition
	afterTaskHook   func()             //хук выполненной задачи
	isStopped       bool               //флаг остановки
	cancel          context.CancelFunc //функция остановки
}

func NewWorkerPool(ctx context.Context, numOfWorkers, queueSize int, hook func()) *WorkerPool {
	ctx, cancelFunc := context.WithCancel(ctx)
	wp := &WorkerPool{
		isStopped:     false,
		taskQueue:     make(chan Task, queueSize),
		cancel:        cancelFunc,
		afterTaskHook: hook,
	}
	//обрабатываем задачи с помощью воркеров
	for i := 0; i < numOfWorkers; i++ {
		wp.workerWaitGroup.Add(1)
		go func(workerId int) {
			defer wp.workerWaitGroup.Done()
			wp.process(ctx)
		}(i)
	}
	return wp
}

// основная функция обработки тасок из очереди
func (p *WorkerPool) process(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-p.taskQueue:
			if !ok {
				return
			}
			task()
			// вызываем хук после выполнения задачи, если он установлен
			if p.afterTaskHook != nil {
				p.afterTaskHook()
			}
		}
	}
}

// функция  добавления задачи в пул
func (p *WorkerPool) Submit(task func()) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.isStopped {
		return ErrPoolStopped
	}
	select {
	case p.taskQueue <- task:
		return nil
	default:
		return ErrQueueFull
	}
}

// функция остановки воркер пула - все добавленные в очередь задачи выполняются
func (p *WorkerPool) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isStopped {
		return nil
	}

	p.isStopped = true
	p.cancel()               //вызываем cancelFunc для отмены контекста и остановки воркеров
	close(p.taskQueue)       // закрываем канал задач
	p.workerWaitGroup.Wait() //ждем завершения всех воркеров
	return nil
}
