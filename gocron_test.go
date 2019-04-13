package gocron

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var err = 1

func task() {
	fmt.Println("I am a running job.")
}

func taskWithParams(a int, b string) {
	fmt.Println(a, b)
}

func taskTime() {
	fmt.Println("before running job.")
	time.Sleep(3 * time.Second)
	fmt.Println("after running job.")
}

type JobTest struct {
	Job
	isRunning bool
}

func NewJobTest(intervel string) *JobTest {
	job := &JobTest{*NewJob(intervel), false}
	job.isRunning = false
	job.Do(func() {
		//defer func() {
		//	if p := recover(); p != nil {
		//		fmt.Println("panic recover! p:", p)
		//		//str, ok := p.(string)
		//		//if ok {
		//		//	err = errors.New(str)
		//		//} else {
		//		//	err = errors.New("panic")
		//		//}
		//		debug.PrintStack()
		//	}
		//}()
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

func TestSyncStop(t *testing.T) {

	defaultScheduler.Every("* * * * * * *").Do(task)
	job := NewJobTest("* * * * * * *")
	defaultScheduler.AddJob(&job.Job)
	//defaultScheduler.AddJob((*Job)(unsafe.Pointer(job)))
	//defaultScheduler.Every(1).Second().Do(taskTime)
	defaultScheduler.Start()
	time.Sleep(2 * time.Second)
	fmt.Println(job.isRunning)
	defaultScheduler.StopSync()
	fmt.Println("StopSync ok")
	if defaultScheduler.Len() != 0 {
		t.Fail()
		t.Logf("TestSyncStop not empty")
	}
}

func TestStop(t *testing.T) {
	defaultScheduler.Every("* * * * * * *").Do(taskTime)
	defaultScheduler.Start()
	time.Sleep(2 * time.Second)
	defaultScheduler.Stop()
	if defaultScheduler.Len() != 0 {
		t.Fail()
		t.Logf("TestStop not empty")
	}
}

func TestSecond(*testing.T) {
	defaultScheduler.Every("* * * * * * *").Do(task)
	defaultScheduler.Every("* * * * * * *").Do(taskWithParams, 1, "hello")
	defaultScheduler.Start()
	time.Sleep(10 * time.Second)
}

// This is a basic test for the issue described here: https://github.com/jasonlvhit/gocron/issues/23
func TestScheduler_Weekdays(t *testing.T) {
	scheduler := NewScheduler()

	job1 := scheduler.Every("0 59 23 * * 1 *")
	job2 := scheduler.Every("0 59 23 * * 3 *")
	job1.Do(task)
	job2.Do(task)
	t.Logf("job1 scheduled for %s", job1.NextScheduledTime())
	t.Logf("job2 scheduled for %s", job2.NextScheduledTime())
	if job1.NextScheduledTime() == job2.NextScheduledTime() {
		t.Fail()
		t.Logf("Two jobs scheduled at the same time on two different weekdays should never run at the same time.[job1: %s; job2: %s]", job1.NextScheduledTime(), job2.NextScheduledTime())
	}
}

// This ensures that if you schedule a job for today's weekday, but the time is already passed, it will be scheduled for
// next week at the requested time.
func TestScheduler_WeekdaysTodayAfter(t *testing.T) {
	scheduler := NewScheduler()

	now := time.Now()
	timeToSchedule := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-1, 0, 0, time.Local)

	job := callTodaysWeekday(scheduler.Every(fmt.Sprintf("0 %d %d * * %d *", timeToSchedule.Minute(), timeToSchedule.Hour(), timeToSchedule.Weekday())))
	job.Do(task)
	t.Logf("job is scheduled for %s", job.NextScheduledTime())
	if job.NextScheduledTime().Weekday() != timeToSchedule.Weekday() {
		t.Fail()
		t.Logf("Job scheduled for current weekday for earlier time, should still be scheduled for current weekday (but next week)")
	}
	nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, now.Hour(), now.Minute()-1, 0, 0, time.Local)
	if !job.NextScheduledTime().Equal(nextWeek) {
		t.Fail()
		t.Logf("Job should be scheduled for the correct time next week.")
	}
}

// utility function for testing the weekday functions *on* the current weekday.
func callTodaysWeekday(job JobInterface) JobInterface {
	return job
}

func TestScheduler_Remove(t *testing.T) {
	scheduler := NewScheduler()
	inter := scheduler.Every("0 * * * * * ").Do(task)
	scheduler.Every("0 * * * * * ").Do(taskWithParams, 1, "hello")
	if scheduler.Len() != 2 {
		t.Fail()
		t.Logf("Incorrect number of jobs - expected 2, actual %d", scheduler.Len())
	}
	scheduler.Remove(inter)
	if scheduler.Len() != 1 {
		t.Fail()
		t.Logf("Incorrect number of jobs after removing 1 job - expected 1, actual %d", scheduler.Len())
	}
	scheduler.Remove(inter)
	if scheduler.Len() != 1 {
		t.Fail()
		t.Logf("Incorrect number of jobs after removing non-existent job - expected 1, actual %d", scheduler.Len())
	}
}

func TestRemoveMulti(t *testing.T) {
	const TestLen = 50
	s := [TestLen]int{}
	for a := 0; a < 50; a++ {
		s[a] = a + 1
	}
	fmt.Println("orgin", s)
	fmt.Print("remove ")
	minIdx := -1
	r := make(map[int]int)
	c := 0
	for i := 0; i < TestLen; i++ {
		//
		if rand.Uint32()%2 == 0 {
			c++
			r[i] = c
			fmt.Print(fmt.Sprintf("%d ", s[i]))
			if minIdx == -1 {
				minIdx = i
			}
		}
	}

	fmt.Println("remove", len(r))
	if minIdx >= 0 {
		s[minIdx] = 0
		for j := minIdx + 1; j < TestLen; j++ {
			if _, ok := r[j]; ok {
				s[j] = 0
				continue
			}
			s[minIdx], s[j] = s[j], 0
			minIdx++
		}
	}
	fmt.Println("after", s, minIdx, TestLen-len(r))
	if minIdx != TestLen-len(r) {
		t.Fail()
		t.Logf("remove error %d-%d", minIdx, TestLen-len(r))
	}
}
