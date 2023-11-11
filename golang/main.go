package main

import (
	"fmt"
	"sync"
	"time"
)

// ЗАДАНИЕ:
// * сделать из плохого кода хороший;
// * важно сохранить логику появления ошибочных тасков;
// * сделать правильную мультипоточность обработки заданий.
// Обновленный код отправить через merge-request.

// приложение эмулирует получение и обработку тасков, пытается и получать и обрабатывать в многопоточном режиме
// В конце должно выводить успешные таски и ошибки выполнены остальных тасков

// A Ttype represents a meaninglessness of our life
type Ttype struct {
	id         int
	cT         string // время создания
	fT         string // время выполнения
	taskRESULT []byte
	haveError  bool // Во время выполнения была ошибка
}

const (
	debugMode = false
	taskCount = 10
)

func main() {
	wg := &sync.WaitGroup{}
	tasksChannel := make(chan Ttype, taskCount)
	tasksForSortChannel := make(chan Ttype)
	doneTasks := make(chan Ttype)
	unDoneTasks := make(chan error)

	result := make(map[int]Ttype, taskCount)
	var taskErrors []error

	wg.Add(1)
	go taskCreator(tasksChannel, wg, taskCount)

	wg.Add(1)
	go taskWorker(tasksChannel, tasksForSortChannel, wg)

	wg.Add(1)
	go taskSorter(tasksForSortChannel, doneTasks, unDoneTasks, wg)

	wg.Add(1)
	go theBestOfTheBestOkTaskMapCreator(doneTasks, wg, result)

	wg.Add(1)
	go func() {
		taskErrors = theBestOfTheBestSliceErrorMessagesCreator(unDoneTasks, wg)
	}()

	wg.Wait()

	if debugMode {
		fmt.Printf("общее количество НЕ выполненных заданий: %d\n", len(taskErrors))
		fmt.Printf("общее количество выполненных заданий: %d\n", len(result))
	}

	printResults(result, taskErrors)
}

// Дружище, на тебе канал, и количество заданий, которые ты должен сгенерировать
// Когда закончишь, закрой, пожалуйста канал (ты ж писатель)
// Ты будешь запускаться асинхронно, поэтому сообщи, пожалуйста нам, когда закончишь
func taskCreator(taskCh chan<- Ttype, wg *sync.WaitGroup, taskCount int) {
	// Ок, генерирую таски
	localWg := sync.WaitGroup{}
	for i := 0; i < taskCount; i++ {
		localWg.Add(1)
		go func(taskNumber int) {
			ft := time.Now().Format(time.RFC3339)
			if time.Now().Nanosecond()%2 > 0 { // вот такое условие появления ошибочных тасков
				ft = "Some error occured"
			}

			if debugMode {
				task := Ttype{cT: ft, id: taskNumber + 1}
				fmt.Printf("Создана задача № %d: %v\n", taskNumber+1, task)
				taskCh <- task // передаем таск на выполнение
			} else {
				taskCh <- Ttype{cT: ft, id: taskNumber + 1} // передаем таск на выполнение
			}
			localWg.Done()
		}(i)

	}
	localWg.Wait()
	// закрываю канал для записи
	close(taskCh)
	// Сообщаю вызывающей горутине, что я закончил работу
	wg.Done()
}

// Дружище, я дам тебе канал с заданиями. Обработай их, пожалуйста, как время будет
// Результат работы передай, пожалуйста, в выходной канал.
// Когда закончишь - сообщи. И канал закрыть не забудь!
func taskWorker(taskCh <-chan Ttype, taskChResult chan<- Ttype, wg *sync.WaitGroup) {
	localWg := sync.WaitGroup{}
	for task := range taskCh {
		localWg.Add(1)
		go func(task Ttype) {
			tt, err := time.Parse(time.RFC3339, task.cT)
			switch {
			case err != nil:
				task.haveError = true
				task.taskRESULT = []byte("something went wrong")
			default:
				if tt.After(time.Now().Add(-20 * time.Second)) {
					task.taskRESULT = []byte("task has been successed")
				} else {
					task.haveError = true
					task.taskRESULT = []byte("something went wrong")
				}
			}

			task.fT = time.Now().Format(time.RFC3339Nano)

			time.Sleep(time.Millisecond * 150)

			if debugMode {
				fmt.Printf("Обработана задача № %d: %v\n", task.id, task)
			}
			taskChResult <- task
			localWg.Done()
		}(task)
	}
	localWg.Wait()
	close(taskChResult)
	wg.Done()
}

// Господин сортировщик, я дам тебе канал с заданиями, ты их, пожалуйста раскидай по двум каналам
// В один, если задание содержит ошибку, в другой, если задание ошибку не содержит.
// Когда закончишь - маякни, ок ?
func taskSorter(taskCh <-chan Ttype, doneTasks chan<- Ttype, undoneTasks chan<- error, wg *sync.WaitGroup) {
	// Да без проблем, давай свои задачи
	localWg := sync.WaitGroup{}
	for task := range taskCh {
		localWg.Add(1)
		go func(task Ttype) {
			if task.haveError {
				if debugMode {
					fmt.Printf("Задача № %d содержит ошибку\n", task.id)
				}
				undoneTasks <- fmt.Errorf("task id %d time %s, error %s", task.id, task.cT, task.taskRESULT)
			} else {
				if debugMode {
					fmt.Printf("В задаче № %d ошибок нет\n", task.id)
				}
				doneTasks <- task
			}
			localWg.Done()
		}(task)
	}
	localWg.Wait()
	close(doneTasks)
	close(undoneTasks)
	wg.Done()
}

// О великий создатель хорошо выполненных заданий!
// Снизойди до меня. Прошу создать мапку с хорошо выполненными заданиями
func theBestOfTheBestOkTaskMapCreator(doneTasks <-chan Ttype, wg *sync.WaitGroup, result map[int]Ttype) {
	if result == nil {
		println("передана не инициализированная мапа")
		return
	}
	//	mu := sync.Mutex{}
	localWg := sync.WaitGroup{}
	for task := range doneTasks {
		localWg.Add(1)
		go func(task Ttype) {
			//	mu.Lock()
			result[task.id] = task
			if debugMode {
				fmt.Printf("Задача № %d добавлена в мапу выполненных заданий\n", task.id)
			}
			//	mu.Unlock()
			localWg.Done()
		}(task)
	}
	localWg.Wait()
	wg.Done()
}

// О великий создатель срезов ошибок, возникших при создании заданий!
// Снизойди до меня. Прошу создать срез с ошибками при выполнении заданий
func theBestOfTheBestSliceErrorMessagesCreator(errorsCh <-chan error, wg *sync.WaitGroup) []error {
	var result []error
	localWg := sync.WaitGroup{}
	for err := range errorsCh {
		localWg.Add(1)
		go func() {
			result = append(result, err)
			if debugMode {
				fmt.Println("в срез ошибок НЕ выполненных заданий добавлена ошибка: ", err)
			}
			localWg.Done()
		}()

	}
	localWg.Wait()
	wg.Done()
	return result
}

func printResults(doneTasks map[int]Ttype, errors []error) {
	println("Errors:")
	for _, err := range errors {
		println(err.Error())
	}

	println("Done tasks:")
	if debugMode {
		for k, v := range doneTasks {
			fmt.Printf("Задача № %d: %v\n", k, v)
		}
	} else {
		for r := range doneTasks {
			println(r)
		}
	}

}
