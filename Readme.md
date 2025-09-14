# GolangWorkerPool
В качестве задания для отбора на стажировку в компанию "Kaspersky" было предложено реализовать Worker Pool - один из основных паттернов конкурентного программирования. Представлено реализованное решение этого задания. Выполнил: Арсланов Ян Альфредович. 

## Задача 
Реализовать интерфейс Pool, который позволяет выполнять задачи параллельно. 

``` type Pool interface { 
  func Submit(task func()) error //добавить задачу в пул
  func Stop() error // остановить воркер пул, дождаться выполнения всех добавленных ранее в очередь задач.
} 
```

## Запуск и настройка

1. Необходимо склонировать файлы из репозитория. 

```git clone https://github.com/JanArsMAI/GolangWorkerPool.git```

2. Далее нужно в корне проекта создать файл `.env` и прописать в нём путь до конфигурационного файла в переменной PATH. 
Пример: ```PATH=cfg/config.json```

3. Далее в папке `cfg` в файле `config.json` установить нужную конфигурацию, ограничив размер очереди или число воркеров, таймаут выполнения задачи и путь,  мы будем считывать данные и записывать резльтат. 

4. В рабочей папке создаём `jobs.json` и `results.json`, в `jobs.json` можем указать наши задачи, которые хотим обработать в нижеприведённом формате. Если не указать, то задания сгенеряться автоматически. Пример:
```
  {"job_id": 1, "job_type": 1},
  {"job_id": 2, "job_type": 2},
  {"job_id": 3, "job_type": 1},
  {"job_id": 4, "job_type": 2},
  {"job_id": 5, "job_type": 1},
  {"job_id": 6, "job_type": 2},
  {"job_id": 7, "job_type": 1},
  {"job_id": 8, "job_type": 2},
  {"job_id": 9, "job_type": 1},
  {"job_id": 10, "job_type": 2}
```

6. Запускаем работу с помощью команды: ```go run cmd/main.go```
7.  В файле `results.json` будут статусы по каждой таске. 


## Реализация и общая идея решения 

Основные файлы проекта: 
1. `config.go` - считывает конфиги пользователя
2. `main.go` - входная точка приложения, внутри которой происходит инициализация воркер пула, конфигов, а также отправка задач в очередь и запись в файл результатов. 
3. `worker_pool.go` - реализация логики воркер пула. 
4. `worker_pool_test.go` - файл с тестами. 

Реализация Worker Pool построена на принципах конкурентного программирования Go с использованием:
- Горутин для параллельного выполнения задач
- Каналов для управления очередью задач и синхронизации
- Context для graceful shutdown
- Mutex для thread-safe доступа к общему состоянию


Структура Компонентов:

1. Worker Pool: 
```
type WorkerPool struct {
	taskQueue       chan Task          //очередь задач
	workerWaitGroup sync.WaitGroup     //wait group для управления воркерами
	mu              sync.RWMutex       //мьютекс для избежания race condition
	afterTaskHook   func()             //хук выполненной задачи
	isStopped       bool               //флаг остановки
	cancel          context.CancelFunc //функция остановки
}
```

2. Механизм работы 

Инициализация:
- Создается буферизированный канал заданного размера
- Запускается N горутин-воркеров
- Каждый воркер слушает канал задач и context для отмены

Добавление задачи: 
```
func (p *WorkerPool) Submit(task func()) error {
	p.mu.RLock() //блокируем мьютекс
	defer p.mu.RUnlock() // в defer откладываем разблокировку

	if p.isStopped {
		return ErrPoolStopped //проверка, что Worker Pool работает
	}
	select {
	case p.taskQueue <- task: //запись задачи в канал
		return nil
	default:
		return ErrQueueFull //если канал заполнен, то возвращаем ошибку
	}
}
```

Обработка задачи:
```
func (p *WorkerPool) process(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():     // получен сигнал остановки из контекста
            return
        case task, ok := <-p.taskQueue:
            if !ok {           //закрытый канал
                return
            }
            task()             // выполняем задачу
            if p.afterTaskHook != nil {
                p.afterTaskHook() //вызываем хук
            }
        }
    }
}
``` 

Остановка воркер пула: 
```
func (p *WorkerPool) Stop() error {
    p.mu.Lock() //блокируем мьютекс
    defer p.mu.Unlock() //откладываем разблокировку
    
    if p.isStopped {
        return nil //проверка на остановку
    }
    
    p.isStopped = true
    p.cancel()        //отправка сигнала остановки воркерам
    close(p.taskQueue) // закрытие канала задач
    p.workerWaitGroup.Wait() // ожидание завершения воркеров
    
    return nil
}
```

Ключевые особенности реализации:

1. Ограничение очереди
- Размер очереди задается через queueSize
- При переполнении возвращается ErrQueueFull
- Предотвращает неконтролируемый рост памяти благодаря ограниченным размерам очередли
- Конфиги позволяют пользователю настроить нужное число воркеров и ограничить размер очереди, а также задать путь выходного файла

2. Концепция Graceful Shutdown
- Stop() дожидается завершения всех текущих задач
- Контекст обеспечивает корректную остановку воркеров

3. Безопасное изменения данных
- RWMutex защищает состояние пула от race conditions
- Атомарные операции с флагом isStopped

## main.go

Определяем структуру джобы, в зависимости от типа задачи будут выполняться по разному: с разной логикой и временем выполнения
```
type JobType int

var (
	TypeFirst  JobType = 1
	TypeSecond JobType = 2
)

type Job struct {
	JobId   int     `json:"job_id"`
	JobType JobType `json:"job_type"`
}
``` 

Структура результирующей записи:
```
type JobResult struct {
	JobId     int       `json:"job_id"`
	JobType   JobType   `json:"job_type"`
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
	Error     string    `json:"error,omitempty"`
}
``` 

Помимо функции main реализовано несколько других вспомогательных функций:
```
// функция считывания джоб из файла
func readJobsFromFile(filename string) ([]Job, error) {
	...
}

// функция генерации джоб(на случай, если путь файла входа не указан)
func generateDefaultJobs(count int) []Job {
	...
}

// функция создания задачи, она передаётся в worker pool
func createTask(job Job, firstHook, secondHook func(Job), resultChan chan<- JobResult) func() {
	...
}

// функция обработки джоб(таск) - именно эта функция выполняется внутри таски
func processJob(job Job) error {
	...
}
```

## Пример результата работы 
Входной файл `jobs.json`:
```
[
  {"job_id": 1, "job_type": 1},
  {"job_id": 2, "job_type": 2},
  {"job_id": 3, "job_type": 1},
  {"job_id": 4, "job_type": 2},
  {"job_id": 5, "job_type": 1},
  {"job_id": 6, "job_type": 2},
  {"job_id": 7, "job_type": 1},
  {"job_id": 8, "job_type": 2},
  {"job_id": 9, "job_type": 1},
  {"job_id": 10, "job_type": 2}
]
```
Файл конфигурации `config.json`
```
{
  "workerPool": {
    "queueSize": 20,
    "numberOfWorkers": 5
  },
  "timeout": 30,
  "path": "./working_folder"
}
```
Результат в файле `results.json`:
```
{
  "job_id": 2,
  "job_type": 2,
  "status": "completed",
  "start_time": "2025-09-14T11:08:50.7506149+03:00",
  "end_time": "2025-09-14T11:08:51.2509765+03:00",
  "duration": "500.3616ms"
}
{
  "job_id": 4,
  "job_type": 2,
  "status": "completed",
  "start_time": "2025-09-14T11:08:50.851209+03:00",
  "end_time": "2025-09-14T11:08:51.3516487+03:00",
  "duration": "500.4397ms"
}
{
  "job_id": 1,
  "job_type": 1,
  "status": "completed",
  "start_time": "2025-09-14T11:08:50.6822183+03:00",
  "end_time": "2025-09-14T11:08:51.5505489+03:00",
  "duration": "868.3306ms"
}
{
  "job_id": 3,
  "job_type": 1,
  "status": "completed",
  "start_time": "2025-09-14T11:08:50.8009384+03:00",
  "end_time": "2025-09-14T11:08:51.7511348+03:00",
  "duration": "950.1964ms"
}
{
  "job_id": 6,
  "job_type": 2,
  "status": "completed",
  "start_time": "2025-09-14T11:08:51.2514838+03:00",
  "end_time": "2025-09-14T11:08:51.7517716+03:00",
  "duration": "500.2878ms"
}
{
  "job_id": 5,
  "job_type": 1,
  "status": "completed",
  "start_time": "2025-09-14T11:08:50.9015121+03:00",
  "end_time": "2025-09-14T11:08:51.9515716+03:00",
  "duration": "1.0500595s"
}
{
  "job_id": 8,
  "job_type": 2,
  "status": "completed",
  "start_time": "2025-09-14T11:08:51.5505489+03:00",
  "end_time": "2025-09-14T11:08:52.0508004+03:00",
  "duration": "500.2515ms"
}
{
  "job_id": 10,
  "job_type": 2,
  "status": "completed",
  "start_time": "2025-09-14T11:08:51.7517716+03:00",
  "end_time": "2025-09-14T11:08:52.2522987+03:00",
  "duration": "500.5271ms"
}
{
  "job_id": 7,
  "job_type": 1,
  "status": "completed",
  "start_time": "2025-09-14T11:08:51.3516487+03:00",
  "end_time": "2025-09-14T11:08:52.5029254+03:00",
  "duration": "1.1512767s"
}
{
  "job_id": 9,
  "job_type": 1,
  "status": "completed",
  "start_time": "2025-09-14T11:08:51.7511348+03:00",
  "end_time": "2025-09-14T11:08:53.0015564+03:00",
  "duration": "1.2504216s"
}
```

## Тестирование
Были реализованы модульные тесты при помощи стандартного пакета `testing`, покрытие можно увидеть тут:
```
go test -cover ./internal/worker_pool/...
ok      github.com/JanArsMAI/GolangWorkerPool/internal/worker_pool      (cached)        coverage: 96.9% of statements
```
 Внутри тестов реализованы следующие сценарии:

 - Тесты на переполнение очереди

 - Тесты на graceful shutdown

 - Тесты на конкурентное добавление задач

 - Тесты на обработку ошибок

 - Тесты на работу хуков