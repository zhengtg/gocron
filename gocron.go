// goCron : A Golang Job Scheduling Package.
//
// An in-process scheduler for periodic jobs that uses the builder pattern
// for configuration. Schedule lets you run Golang functions periodically
// at pre-determined intervals using a simple, human-friendly syntax.
//
// Inspired by the Ruby module clockwork <https://github.com/tomykaira/clockwork>
// and
// Python package schedule <https://github.com/dbader/schedule>
//
// See also
// http://adam.heroku.com/past/2010/4/13/rethinking_cron/
// http://adam.heroku.com/past/2010/6/30/replace_cron_with_clockwork/
//
// Copyright 2014 Jason Lyu. jasonlvhit@gmail.com .
// All rights reserved.
// Use of this source code is governed by a BSD-style .
// license that can be found in the LICENSE file.
package gocron

import (
	"errors"
	"github.com/gorhill/cronexpr"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Time location, default set by the time.Local (*time.Location)
var loc = time.Local

// Change the time location
func ChangeLoc(newLocation *time.Location) {
	loc = newLocation
}

// Max number of jobs, hack it if you need.
const MAXJOBNUM = 10000

type JobInterface interface {
	SetName(string)
	GetName() string
	SetWaitGroup(wg *sync.WaitGroup)
	ShouldRun() bool
	Run()
	AfterRun()
	SetCronExpr(string)
	GetCronExpr() string
	ScheduleNextRun()
	NextScheduledTime() time.Time
	//Do and JobFunction
	Do(jobFun interface{}, params ...interface{}) (job JobInterface)
	JobFunction()
}

type Job struct {
	// the job jobFunc to run, func[jobFunc]
	jobFunc string

	// cron express for job
	cronExprString string

	cronExpress *cronexpr.Expression

	// datetime of last run
	lastRun time.Time
	// datetime of next run
	nextRun time.Time

	// Map for the function task store
	funcs map[string]interface{}

	// Map for function and  params of function
	fparams map[string]([]interface{})

	// Sync job for schedule
	wg *sync.WaitGroup

	// Job name
	jobName string
}

// Create a new job with the time interval.
func NewJob(cronExpr string) *Job {
	return &Job{
		"",
		cronExpr,
		nil,
		time.Unix(0, 0),
		time.Unix(0, 0),
		make(map[string]interface{}),
		make(map[string]([]interface{})),
		nil, "",
	}
}

// True if the job should be run now
func (j *Job) ShouldRun() bool {
	return time.Now().After(j.nextRun)
}

func (j *Job) Run() {
	j.AddSync()
	defer j.ClearSync()
	defer j.AfterRun()

	j.JobFunction()
}

//Run the job and immediately reschedule it
func (j *Job) run() (result []reflect.Value, err error) {
	f := reflect.ValueOf(j.funcs[j.jobFunc])
	params := j.fparams[j.jobFunc]
	if len(params) != f.Type().NumIn() {
		err = errors.New("the number of param is not adapted")
		return
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}

	j.AddSync()
	defer j.ClearSync()

	result = f.Call(in)
	j.lastRun = time.Now()
	j.ScheduleNextRun()
	return
}

// add running flag for job
func (j *Job) AddSync() {
	if j.wg != nil {
		j.wg.Add(1)
	}
}

// clear running flag for job
func (j *Job) ClearSync() {
	if j.wg != nil {
		j.wg.Done()
	}
}

// call after finish job
func (j *Job) AfterRun() {
	j.lastRun = time.Now()
	j.ScheduleNextRun()
}

func (j *Job) SetWaitGroup(wg *sync.WaitGroup) {
	j.wg = wg
}

func (j *Job) GetName() string {
	return j.jobName
}
func (j *Job) SetName(name string) {
	j.jobName = name
}

func (j *Job) GetCronExpr() string {
	return j.cronExprString
}

func (j *Job) SetCronExpr(cron string) {
	j.cronExprString = cron
}

func (j *Job) JobFunction() {
	f := reflect.ValueOf(j.funcs[j.jobFunc])
	params := j.fparams[j.jobFunc]
	if len(params) != f.Type().NumIn() {
		panic("the number of param is not adapted")
		return
	}
	in := make([]reflect.Value, len(params))
	for k, param := range params {
		in[k] = reflect.ValueOf(param)
	}

	f.Call(in)
}

// for given function fn, get the name of function.
func getFunctionName(fn interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf((fn)).Pointer()).Name()
}

// Specifies the jobFunc that should be called every time the job runs
//
func (j *Job) Do(jobFun interface{}, params ...interface{}) (job JobInterface) {
	typ := reflect.TypeOf(jobFun)
	if typ.Kind() != reflect.Func {
		panic("only function can be schedule into the job queue.")
	}

	fname := getFunctionName(jobFun)
	j.funcs[fname] = jobFun
	j.fparams[fname] = params
	j.jobFunc = fname
	//schedule the next run
	j.ScheduleNextRun()
	j.jobName = fname
	return j
}

//Compute the instant when this job should run next
func (j *Job) ScheduleNextRun() {
	if j.cronExpress == nil {
		j.cronExpress = cronexpr.MustParse(j.cronExprString)
		if j.cronExpress.Next(time.Now()).IsZero() {
			panic("invalid cron express string")
		}
	}

	if j.lastRun == time.Unix(0, 0) {
		j.lastRun = time.Now()
	}
	j.nextRun = j.cronExpress.Next(j.lastRun)
}

// NextScheduledTime returns the time of when this job is to run next
func (j *Job) NextScheduledTime() time.Time {
	return j.nextRun
}

// Class Scheduler, the only data member is the list of jobs.
type Scheduler struct {
	// Array store jobs
	jobs [MAXJOBNUM]JobInterface

	// Size of jobs which jobs holding.
	size int

	// sign chan for stop
	signChan chan bool

	// manager jobs
	wg *sync.WaitGroup
}

// Scheduler implements the sort.Interface{} for sorting jobs, by the time nextRun

func (s *Scheduler) Len() int {
	return s.size
}

func (s *Scheduler) Swap(i, j int) {
	s.jobs[i], s.jobs[j] = s.jobs[j], s.jobs[i]
}

func (s *Scheduler) Less(i, j int) bool {
	return s.jobs[j].NextScheduledTime().After(s.jobs[i].NextScheduledTime())
}

// Create a new scheduler
func NewScheduler() *Scheduler {
	return &Scheduler{[MAXJOBNUM]JobInterface{}, 0, make(chan bool, 1), new(sync.WaitGroup)}
}

// Get the current runnable jobs, which shouldRun is True
func (s *Scheduler) getRunnableJobs() (running_jobs [MAXJOBNUM]JobInterface, n int) {
	runnableJobs := [MAXJOBNUM]JobInterface{}
	n = 0
	sort.Sort(s)
	for i := 0; i < s.size; i++ {
		if s.jobs[i].ShouldRun() {

			runnableJobs[n] = s.jobs[i]
			//fmt.Println(runnableJobs)
			n++
		} else {
			break
		}
	}
	return runnableJobs, n
}

// Datetime when the next job should run.
func (s *Scheduler) NextRun() (JobInterface, time.Time) {
	if s.size <= 0 {
		return nil, time.Now()
	}
	sort.Sort(s)
	return s.jobs[0], s.jobs[0].NextScheduledTime()
}

// Schedule a new periodic job
func (s *Scheduler) AddJob(job JobInterface) JobInterface {
	if job == nil {
		panic("job is nil")
	}
	job.SetWaitGroup(s.wg)
	job.ScheduleNextRun()
	s.jobs[s.size] = job
	s.size++
	return job
}

// Schedule a new periodic job
func (s *Scheduler) Every(cron string) JobInterface {
	job := NewJob(cron)
	job.SetWaitGroup(s.wg)
	s.jobs[s.size] = job
	s.size++
	return job
}

// Run all the jobs that are scheduled to run.
func (s *Scheduler) RunPending() {
	runnableJobs, n := s.getRunnableJobs()

	if n != 0 {
		for i := 0; i < n; i++ {
			runnableJobs[i].Run()
		}
	}
}

// Run all jobs regardless if they are scheduled to run or not
func (s *Scheduler) RunAll() {
	for i := 0; i < s.size; i++ {
		s.jobs[i].Run()
	}
}

// Run all jobs with delay seconds
func (s *Scheduler) RunAllwithDelay(d int) {
	for i := 0; i < s.size; i++ {
		s.jobs[i].Run()
		time.Sleep(time.Duration(d))
	}
}

// Remove specific job j
func (s *Scheduler) Remove(j JobInterface) {
	i := 0
	found := false

	for ; i < s.size; i++ {
		if s.jobs[i] == j {
			found = true
			break
		}
	}

	if !found {
		return
	}

	for j := (i + 1); j < s.size; j++ {
		s.jobs[i] = s.jobs[j]
		i++
	}
	s.size = s.size - 1
}

// Remove specific job j
func (s *Scheduler) RemoveOnceName(name string) {
	i := 0
	found := false

	for ; i < s.size; i++ {
		if s.jobs[i].GetName() == name {
			found = true
			break
		}
	}

	if !found {
		return
	}

	for j := (i + 1); j < s.size; j++ {
		s.jobs[i] = s.jobs[j]
		i++
	}
	s.size = s.size - 1
}

// Remove specific job j
func (s *Scheduler) RemoveAllName(name string) {
	minIdx := -1
	r := make(map[int]bool)
	for i := 0; i < s.size; i++ {
		if s.jobs[i].GetName() == name {
			r[i] = true
			if minIdx == -1 {
				minIdx = i
			}
		}
	}

	if minIdx == -1 {
		return
	}

	s.jobs[minIdx] = nil
	for j := minIdx + 1; j < s.size; j++ {
		if _, ok := r[j]; ok {
			s.jobs[j] = nil
			continue
		}
		s.jobs[minIdx], s.jobs[j] = s.jobs[j], nil
		minIdx++
	}
	s.size = minIdx
	//s.size = s.size - len(r)
}

// Delete all scheduled jobs
func (s *Scheduler) Clear() {
	for i := 0; i < s.size; i++ {
		s.jobs[i] = nil
	}
	s.size = 0
}

// Start all the pending jobs
// Add seconds ticker
func (s *Scheduler) Start() chan bool {
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				s.RunPending()
			case <-s.signChan:
				return
			}
		}
	}()

	return s.signChan
}

func (s *Scheduler) Stop() {
	s.signChan <- false
	s.Clear()
}

func (s *Scheduler) StopSync() {
	s.Stop()
	s.wg.Wait()
}

// The following methods are shortcuts for not having to
// create a Schduler instance

var defaultScheduler = NewScheduler()
var jobs = defaultScheduler.jobs

// Schedule a new periodic job
func Every(cron string) JobInterface {
	return defaultScheduler.Every(cron)
}

// Run all jobs that are scheduled to run
//
// Please note that it is *intended behavior that run_pending()
// does not run missed jobs*. For example, if you've registered a job
// that should run every minute and you only call run_pending()
// in one hour increments then your job won't be run 60 times in
// between but only once.
func RunPending() {
	defaultScheduler.RunPending()
}

// Run all jobs regardless if they are scheduled to run or not.
func RunAll() {
	defaultScheduler.RunAll()
}

// Run all the jobs with a delay in seconds
//
// A delay of `delay` seconds is added between each job. This can help
// to distribute the system load generated by the jobs more evenly over
// time.
func RunAllwithDelay(d int) {
	defaultScheduler.RunAllwithDelay(d)
}

// Run all jobs that are scheduled to run
func Start() chan bool {
	return defaultScheduler.Start()
}

// Clear
func Clear() {
	defaultScheduler.Clear()
}

// Remove
func Remove(j JobInterface) {
	defaultScheduler.Remove(j)
}

// NextRun gets the next running time
func NextRun() (job JobInterface, time time.Time) {
	return defaultScheduler.NextRun()
}
