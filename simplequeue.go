
package main

import (
	_ "fmt"
	"math/rand"
	"sync"
)

// ============================================================

type Queue struct {
	mutex sync.Mutex
	queue []int
}

func (q *Queue) Push( i int ) {
	q.mutex.Lock()
	q.queue = append( q.queue, i )
	q.mutex.Unlock()
}

func (q *Queue) Pop() (int, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.queue) == 0 { return 0, false } // empty queue

	i := q.queue[0]
	copy( q.queue[0:], q.queue[1:] )
	q.queue = q.queue[:len(q.queue)-1] 

	return i, true
}

func (q *Queue) Count() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.queue)
}

// ============================================================

type Semaphore struct {
	mutex sync.Mutex
	toTerminate int
}

func (sema *Semaphore) SetTerminate( val int ) {
	// Sets the number of threads to terminate to the given value
	sema.mutex.Lock()
	sema.toTerminate = val
	sema.mutex.Unlock()
}

func (sema *Semaphore) TerminateSelf() bool {
	// Returns true if the calling thread should terminate
	sema.mutex.Lock()
	defer sema.mutex.Unlock()

	if sema.toTerminate > 0 {
		sema.toTerminate -= 1
		return true
	}

	return false
}

// ============================================================

// Counter with explicit add, sub, and get methods

type Counter struct {
	mutex sync.Mutex
	counter int
}

func (c *Counter) Add() {
	c.mutex.Lock()
	c.counter += 1
	c.mutex.Unlock()
}

func (c *Counter) Sub() {
	c.mutex.Lock()
	c.counter -= 1
	c.mutex.Unlock()
}

func (c *Counter) Val() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.counter
}
	
// ============================================================

type Millis int

func sleepMillis( i Millis ) {
	SleepMilliseconds( int(i) )
}

// ============================================================

// Put items of random size (up to "work") on queue, sleep "idle" idle between

func GoProduce( q *Queue, idle Millis, work Millis ) {
	for {
		q.Push( rand.Intn( int(work) ) )
		sleepMillis(idle)
	}
}

// Take items off queue, sleep "retry" if queue empty
// Check semaphore between items, return if instructed to terminate

func GoConsume( q *Queue, retry Millis, pool *Counter, sema *Semaphore ) {
	pool.Add()          // Inform pool of your state
	defer pool.Sub()

	for {
		if sema.TerminateSelf() { return }

		work, ok := q.Pop()
		if !ok {
			sleepMillis( retry )
		} else {
			sleepMillis( Millis(work) )
//			rc.put( time.Now().UnixNano() ) // ???
		}
	}
}

// ============================================================

type Controller struct {
	kp, ki, kd float64
	cumul, prev float64
}

func (c *Controller) CalcPid( crr, setpoint int ) float64 {
	err := float64( crr - setpoint )
	c.cumul += err

	res := c.kp*err + c.ki*c.cumul + c.kd*(err - c.prev)
	if res < 0 {
		res = 0
	}

	c.prev = err
	
	return res
}

func (c *Controller) Regulate( q *Queue, setpoint int, pool *Counter,
	sema *Semaphore ) {

	desired := c.CalcPid( q.Count(), setpoint )
	actual := pool.Val()

	diff := int(desired) - actual

	if diff < 0 {
		sema.SetTerminate( -diff )           // kill some threads
	} else {
		for i:=0; i<diff; i++ { 
			go GoConsume( q, 100, pool, sema ) // start more threads
		}
		
	}
}

// ============================================================

func main() {
	setup := GraphSetup( "Left", "Left", "Right" );
	setup.SetHorizontalRange( -25.0, 5.0 )
	setup.AssignColumnColors( "red", "black", "blue" )
	
	buf := InitializeHttp( setup, 8080, "home.html" )
	buf.SetVerticalRanges( 0.0, 50.0, 0.0, 20.0 )
	buf.InitializeForm( "setp", 20.0 )
	buf.InitializeForm( "kp", 0.5 )
	
	q := Queue{queue: make( []int, 0 )}

	pool := Counter{}
	sema := Semaphore{}

	c := Controller{kp: 1.0, ki: 1.0, kd: 0.0}
	
	go GoProduce( &q, 230, 500 )
	go GoProduce( &q, 240, 500 )
	go GoProduce( &q, 250, 1500 )
	go GoProduce( &q, 260, 500 )
	go GoProduce( &q, 270, 500 )

	setpoint := 20
	for {
		c.Regulate( &q, setpoint, &pool, &sema )

		qlen := q.Count()
		threads := pool.Val()

//		fmt.Println( qlen, setpoint, threads )
		buf.Push( float64(qlen), float64(setpoint), float64(threads) )

		setpoint = int(buf.Read( "setp" ))
		c.kp = buf.Read( "kp" )

		sleepMillis( 100 )
	}
}
