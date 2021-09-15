package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"time"

	"github.com/alexeyco/simpletable"

	cron "github.com/robfig/cron/v3"
)

type Content int

// Declare related constants for each weekday starting with index 1
const (
	Ip Content = iota
	Pid
	Confidence
	Timestamp
)

var table *simpletable.Table = simpletable.New()

var (
	// Number of connected processes
	totalConnected = 0
	// if we dont get an update after this period then we reduce the confidence
	treshold = 15000
	// Failure point
	failurePoint = 0.2
)

type HeartBeat struct {
	IpAddress string
	Pid       int
	Timestamp int64
}

type Tasks int

func (t *Tasks) SendHeartBeat(heartBeat HeartBeat, reply *HeartBeat) error {

	fmt.Printf("Received heart beat from process with IP %s and PID %d", heartBeat.IpAddress, heartBeat.Pid)
	*reply = heartBeat
	fmt.Println("\n updating Accural failure detection table...")
	isNew := true
	for _, v := range table.Body.Cells {
		if tablePid, err := strconv.Atoi(v[Pid].Text); v[Ip].Text == heartBeat.IpAddress && err != nil && tablePid == heartBeat.Pid {
			// this means the process already exists on our table
			// increment confidence
			currentConfidence, err := strconv.Atoi(v[Confidence].Text)

			if err != nil {
				log.Fatal(err)
			}

			v[Confidence].Text = fmt.Sprintf("%f", float64(currentConfidence)+0.1)

			isNew = false
		} else if err != nil {
			log.Fatal(err)
		}
	}

	if isNew {
		// increment number of connected processes
		totalConnected++
		// Add new row to the table and set the initial confidence to 0.5
		newRow := []*simpletable.Cell{
			{Align: simpletable.AlignRight, Text: heartBeat.IpAddress},
			{Text: fmt.Sprintf("%d", heartBeat.Pid)},
			{Align: simpletable.AlignRight, Text: fmt.Sprintf("%f", 0.5)},
			{Text: fmt.Sprintf("%d", heartBeat.Timestamp)},
		}
		table.Body.Cells = append(table.Body.Cells, newRow)
	}

	renderTable(table.Body.Cells)

	return nil
}

// setup the RPC server
func SetupRpcServer() {
	task := new(Tasks)

	err := rpc.Register(task)

	if err != nil {
		log.Fatal("Format of service Task isn't correct. ", err)
	}

	rpc.HandleHTTP()

	listener, e := net.Listen("tcp", ":1234")

	if e != nil {
		log.Fatal("Listen error: ", e)
	}
	log.Printf("Serving RPC server on port %d", 1234)

	// Start accept incoming HTTP connections
	err = http.Serve(listener, nil)

	if err != nil {
		log.Fatal("Error serving: ", err)
	}
}

func renderTable(cells [][]*simpletable.Cell) {
	t := simpletable.New()

	t.Header = &simpletable.Header{
		Cells: []*simpletable.Cell{
			{Align: simpletable.AlignCenter, Text: "IP"},
			{Align: simpletable.AlignCenter, Text: "PID"},
			{Align: simpletable.AlignCenter, Text: "CONFIDENCE"},
			{Align: simpletable.AlignCenter, Text: "TIME"},
		},
	}

	t.Footer = &simpletable.Footer{
		Cells: []*simpletable.Cell{
			{},
			{Align: simpletable.AlignRight, Text: "Total Processes"},
			{Align: simpletable.AlignRight, Text: fmt.Sprintf("$ %d", totalConnected)},
		},
	}

	t.SetStyle(simpletable.StyleCompactLite)

	t.Body.Cells = cells

	fmt.Println(t.String())
}

func main() {

	//setup cron
	c := cron.New()

	// RPC server
	SetupRpcServer()

	// Cron to check for connected processes
	c.AddFunc("@every 15s", func() {
		fmt.Println("\n Checking for inactive processes...")

		for index, v := range table.Body.Cells {
			// get timestamp
			storedTimestamp, err := strconv.ParseInt(v[Timestamp].Text, 10, 64)
			if err != nil {
				log.Fatal(err)
			}

			currentTimestamp := time.Now().Unix()

			if difference := currentTimestamp - storedTimestamp; difference >= int64(treshold) {
				fmt.Printf("\n Haven't received a heartbeat from process with ip %s and pid %s for the past %d seconds", v[Ip].Text, v[Pid].Text, difference)
				fmt.Println("\n Decrementing process confidence by 0.1")
				confidence, err := strconv.Atoi(v[Confidence].Text)

				if err != nil {
					log.Fatal(err)
				}

				if decrementedConfidence := float64(confidence) - 0.1; decrementedConfidence <= failurePoint {
					fmt.Printf("This process with ip %s and pid %s now has confidence %f which is below the failure point %f and will be marked as dead", v[Ip].Text, v[Pid].Text, decrementedConfidence, failurePoint)
					table.Body.Cells = append(table.Body.Cells[:index], table.Body.Cells[index+1])
				} else {
					v[Confidence].Text = fmt.Sprintf("%f", decrementedConfidence)
				}
				renderTable(table.Body.Cells)
			}

		}

	})

	c.Start()
	defer c.Stop()

}
