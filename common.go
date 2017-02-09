package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================

func SleepMilliseconds(m int) {
	time.Sleep(time.Duration(m) * time.Millisecond)
}

// ============================================================

// A convenience data structure holding the min and max of a plot range.
// Contains "json tags" that facilitate direct assingment in JS page.

type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// ============================================================

// Static graph config. No sync, because not written to after initialization.

type Setup struct {
	Horiz      Range
	ColumnAxes []int
	Colors     []string
}

func GraphSetup(cols ...string) *Setup {
	setup := Setup{}

	tmp := []int{}
	for _, c := range cols {
		if c == "Right" {
			tmp = append(tmp, 1)
		} else {
			tmp = append(tmp, 0)
		}
	}
	setup.ColumnAxes = tmp

	return &setup
}

func (setup *Setup) SetHorizontalRange(vals ...float64) {
	setup.Horiz = Range{vals[0], vals[1]}
}

func (setup *Setup) AssignColumnColors(vals ...string) {
	setup.Colors = []string{}
	for _, v := range vals {
		setup.Colors = append(setup.Colors, v)
	}
}

func (setup *Setup) Serialize() string {
	s, _ := json.Marshal(setup)
	//	if( err ) { }
	return string(s) // return type: really string? or []byte?
}

// ============================================================

// A synchronized data structure, for data exchange between the HTTP server
// and the rest of the program. Also handles marshalling and parsing of data.

type CommBuffer struct {
	nCol int // number of data columns

	msgMutex sync.Mutex
	Msg      struct {
		Ranges struct {
			Left  Range
			Right Range
		}
		Text string
		Data [][]float64
	}

	formMutex sync.Mutex
	Form      map[string]float64
}

func (buf *CommBuffer) InitializeForm(key string, val float64) {
	buf.Form[key] = val
}

func (buf *CommBuffer) SetVerticalRanges(vals ...float64) {
	buf.msgMutex.Lock()
	buf.Msg.Ranges.Left = Range{vals[0], vals[1]}
	buf.Msg.Ranges.Right = Range{vals[2], vals[3]}
	buf.msgMutex.Unlock()
}

func (buf *CommBuffer) Push(vals ...float64) {
	//	if len(vals) != buf.nCol { panic( len(vals), buf.nCols ) }

	now := float64(time.Now().UnixNano()) / 1e9

	tmp := []float64{0.0} // Column 0: placeholder for x-value
	tmp = append(tmp, vals...)
	tmp = append(tmp, now)

	buf.msgMutex.Lock()
	buf.Msg.Data = append(buf.Msg.Data, tmp)
	//	buf.Msg.Data = append( buf.Msg.Data, vals ) // old style!
	buf.msgMutex.Unlock()
}

func (buf *CommBuffer) Text(txt string) {
	buf.msgMutex.Lock()
	buf.Msg.Text = txt
	buf.msgMutex.Unlock()
}

func (buf *CommBuffer) Pop() string {
	buf.msgMutex.Lock()

	buf.Msg.Text = buf.SerializeForm()

	s, _ := json.Marshal(buf.Msg)
	//	if( err ) { }
	buf.Msg.Data = buf.Msg.Data[0:0]

	buf.msgMutex.Unlock()
	return string(s) // return type: really string? or []byte?
}

func (buf *CommBuffer) Write(val string) {
	fields := strings.Split(strings.Replace(val, " ", "", -1), ";")
	for _, f := range fields {
		tmp := strings.Split(f, "=")
		val, _ := strconv.ParseFloat(tmp[1], 64) // ?if no "=" found?

		buf.formMutex.Lock()
		buf.Form[tmp[0]] = val
		buf.formMutex.Unlock()
	}
}

func (buf *CommBuffer) Read(key string) float64 {
	buf.formMutex.Lock()
	defer buf.formMutex.Unlock()
	return buf.Form[key]
}

func (buf *CommBuffer) SerializeForm() string {
	buf.formMutex.Lock()
	defer buf.formMutex.Unlock()

	keys := []string{}
	for key := range buf.Form {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	s := ""
	for _, key := range keys {
		s += fmt.Sprintf("%s=%f; ", key, buf.Form[key])
	}
	return strings.TrimRight(s, "; ")
}

// ============================================================

func InitializeHttp(setup *Setup, port int, page string) *CommBuffer {
	// create the CommBuffer
	buf := CommBuffer{}
	buf.Form = map[string]float64{}

	//	buf.nCol = len( cols )

	// setup HTTP handlers
	http.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, page)
		})
	http.HandleFunc("/setup",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, setup.Serialize())
		})
	http.HandleFunc("/get",
		func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, buf.Pop())
		})
	http.HandleFunc("/put",
		func(w http.ResponseWriter, r *http.Request) {
			buf.Write(r.FormValue("val"))
		})

	log.Println("Starting on port", port)
	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

	return &buf
}
