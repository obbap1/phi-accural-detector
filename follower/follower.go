package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"time"
)

type HeartBeat struct {
	IpAddress string
	Pid       int
	Timestamp int64
}

func getIpAndProcessId() (string, int, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", 0, err
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), os.Getpid(), nil
			}
		}
	}
	return "", 0, errors.New("no addresses/interfaces")
}

// Connect to the server
func Connect() {
	client, err := rpc.DialHTTP("tcp", "10.0.4.204:1234")
	if err != nil {
		log.Fatal("Connection error: ", err)
	}

	ip, pid, err := getIpAndProcessId()

	if err != nil {
		log.Fatal(err)
	}

	var reply HeartBeat

	body := HeartBeat{IpAddress: ip, Pid: pid, Timestamp: time.Now().Unix()}

	// initial call to setup table
	err = client.Call("Tasks.SendHeartBeat", body, &reply)

	fmt.Println(reply)

	if err != nil {
		log.Fatal(err)
	}

	for {
		// fetch random number between 10 and 30 seconds
		rand.Seed(time.Now().UnixNano())
		min, max := 10, 30
		random := rand.Intn(max-min+1) + min
		fmt.Printf("\n Process with ip %s and pid %d is sleeping for %d seconds", ip, pid, random)
		time.Sleep(time.Duration(random) * time.Second)
		body.Timestamp = time.Now().Unix()
		err = client.Call("Tasks.SendHeartBeat", body, &reply)

		if err != nil {
			log.Fatal(err)
		}
	}

}

func main() {
	Connect()
}
