package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"

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

var (
	table     *tablewriter.Table = tablewriter.NewWriter(os.Stdout)
	tableData [][]string         = nil
)

var (
	// Number of connected processes
	totalConnected = 0
	// if we dont get an update after this period then we reduce the confidence
	treshold = 15
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

	for _, v := range tableData {
		if tablePid, err := strconv.Atoi(v[Pid]); v[Ip] == heartBeat.IpAddress && tablePid == heartBeat.Pid && err == nil {
			// this means the process already exists on our table
			// increment confidence
			currentConfidence, err := strconv.ParseFloat(v[Confidence], 64)
			if err != nil {
				log.Fatal(err)
			}
			currentConfidence += 0.1
			if currentConfidence > 1.0 {
				currentConfidence = 1.0
			}
			v[Confidence] = fmt.Sprintf("%f", currentConfidence)
			v[Timestamp] = fmt.Sprintf("%d", heartBeat.Timestamp)
			isNew = false
		} else if err != nil {
			log.Fatal(err)
		}
	}

	if isNew {
		// increment number of connected processes
		totalConnected++
		fmt.Printf("Total number of connected processes %d", totalConnected)
		// Add new row to the table and set the initial confidence to 0.5
		newRow := []string{
			heartBeat.IpAddress,
			fmt.Sprintf("%d", heartBeat.Pid),
			fmt.Sprintf("%f", 0.5),
			fmt.Sprintf("%d", heartBeat.Timestamp),
		}
		tableData = append(tableData, newRow)
	}
	renderTable()
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

func main() {
	table.SetHeader([]string{"IP", "PID", "CONFIDENCE", "TIME"})
	table.SetFooter([]string{"", "", "Total Processes", fmt.Sprintf("%d", totalConnected)})
	table.SetCenterSeparator("*")
	table.SetColumnSeparator("╪")
	table.SetRowSeparator("-")
	//setup cron
	c := cron.New()

	// Cron to check for connected processes
	c.AddFunc("@every 15s", func() {
		fmt.Println("\n Checking for inactive processes...")
		for index, v := range tableData {
			// get timestamp
			storedTimestamp, err := strconv.ParseInt(v[Timestamp], 10, 64)
			if err != nil {
				log.Fatal(err)
			}
			currentTimestamp := time.Now().Unix()
			if difference := currentTimestamp - storedTimestamp; difference >= int64(treshold) {
				fmt.Printf("\n Haven't received a heartbeat from process with ip %s and pid %s for the past %d seconds", v[Ip], v[Pid], difference)
				fmt.Println("\n Decrementing process confidence by 0.1")
				confidence, err := strconv.ParseFloat(v[Confidence], 64)
				if err != nil {
					log.Fatal(err)
				}
				if decrementedConfidence := float64(confidence) - 0.1; decrementedConfidence <= failurePoint {
					fmt.Printf("This process with ip %s and pid %s now has confidence %f which is below the failure point %f and will be marked as dead", v[Ip], v[Pid], decrementedConfidence, failurePoint)
					tableData = append(tableData[:index], tableData[index+1:]...)
					totalConnected--
				} else {
					v[Confidence] = fmt.Sprintf("%f", decrementedConfidence)
				}
				renderTable()
			}
		}
	})
	c.Start()
	defer c.Stop()

	// RPC server
	SetupRpcServer()
}

func renderTable() {
	table.ClearRows()
	table.AppendBulk(tableData)
	table.ClearFooter()
	table.SetFooter([]string{"", "", "Total Processes", fmt.Sprintf("%d", totalConnected)})
	table.Render()
}
