package main

import (
	"fmt"
	"github.com/zhengtg/gocron"
	"time"
)

func task() {
	fmt.Println("I am runnning task.")
}

func taskWithParams(a int, b string) {
	fmt.Println(a, b)
}

type JobTest struct {
	gocron.Job
	isRunning bool
}

func NewJobTest(intervel uint64) *JobTest {
	job := &JobTest{*gocron.NewJob(intervel), false}
	job.isRunning = false
	job.Do(func() {
		job.isRunning = true
		job.jobFunction()
		job.isRunning = false
	})
	return job
}

func (self *JobTest) jobFunction() string {
	fmt.Println("before this is a test job!!")
	time.Sleep(3 * time.Second)
	//panic("error test")
	fmt.Println("after this is a test job!!")
	return fmt.Sprint("this is a test job!!")
}

func main() {
	//// Do jobs with params
	//gocron.Every(1).Second().Do(taskWithParams, 1, "hello")

	// Do jobs without params
	gocron.Every(1).Second().Do(task)
	gocron.Every(2).Seconds().Do(task)
	gocron.Every(1).Minute().Do(task)
	gocron.Every(2).Minutes().Do(task)
	gocron.Every(1).Hour().Do(task)
	gocron.Every(2).Hours().Do(task)
	gocron.Every(1).Day().Do(task)
	gocron.Every(2).Days().Do(task)

	// Do jobs on specific weekday
	gocron.Every(1).Monday().Do(task)
	gocron.Every(1).Thursday().Do(task)

	// function At() take a string like 'hour:min'
	gocron.Every(1).Day().At("10:30").Do(task)
	gocron.Every(1).Monday().At("18:30").Do(task)

	// remove, clear and next_run
	_, time := gocron.NextRun()
	fmt.Println(time)

	// gocron.Remove(task)
	// gocron.Clear()

	// function Start start all the pending jobs
	<-gocron.Start()

	// also , you can create a your new scheduler,
	// to run two scheduler concurrently
	s := gocron.NewScheduler()
	job := NewJobTest(1).Second()
	s.AddJob(job)
	s.Every(3).Seconds().Do(task)

	_, time := s.NextRun()
	fmt.Println(time)
	<-s.Start()
}
