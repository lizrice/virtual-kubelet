package tello

import (
	"fmt"
	"log"
	"time"

	"gobot.io/x/gobot"
	"gobot.io/x/gobot/platforms/dji/tello"
)

const (
	flightTime = 10 // number of seconds we will fly for

	// drone states
	initState      = "init"
	startState     = "start"
	connectedState = "connected"
	takeOffState   = "ready"
	landingState   = "notReady"
	haltState      = "halt"
)

func droneConnect() (drone *tello.Driver) {
	log.Println("droneConnect")
	drone = tello.NewDriver("8888")
	p.droneState = initState
	lostConnection := false
	timeoutChan := make(chan bool)

	var currentFlightData *tello.FlightData

	work := func() {

		drone.On(tello.FlightDataEvent, func(data interface{}) {
			currentFlightData = data.(*tello.FlightData)
			timeoutChan <- true
		})

		drone.On(tello.ConnectedEvent, func(data interface{}) {
			fmt.Println("Connected event from drone")
			p.droneStateChan <- connectedState
			p.wg.Add(1)
		})

		drone.On(tello.TakeoffEvent, func(data interface{}) {
			fmt.Println("Take Off event from drone")
			p.droneStateChan <- takeOffState
			takeOffTimer := time.NewTimer(flightTime * time.Second)
			go func() {
				<-takeOffTimer.C
				fmt.Printf("Flight timer popped while state is %s\n", getDroneState())
				drone.Land()
			}()
		})

		drone.On(tello.LandingEvent, func(data interface{}) {
			fmt.Println("Landing event from drone")
			p.droneStateChan <- landingState
			landingTimer := time.NewTimer(5 * time.Second)
			go func() {
				<-landingTimer.C
				fmt.Printf("Landing timer popped while state is %s\n", getDroneState())
				p.droneStateChan <- haltState
			}()
		})

		gobot.Every(5*time.Second, func() {
			droneState := getDroneState()
			if droneState == connectedState {
				log.Println("Taking off")
				drone.TakeOff()
			}
		})

		gobot.Every(1*time.Second, func() {
			printFlightData(currentFlightData)
		})
	}

	// Loop for starting and stopping the drone
	go func() {
		for {
			droneInAir := false
			state := ""

			p.Lock()
			if state == takeOffState && p.droneState == takeOffState {
				droneInAir = true
			}
			state = p.droneState
			p.Unlock()

			log.Printf("Current drone state: %s\n", state)
			if droneInAir {
				time.Sleep(time.Second)
			}

			switch state {
			case initState:
				robot := gobot.NewRobot("tello",
					[]gobot.Connection{},
					[]gobot.Device{drone},
					work,
				)

				log.Println("Starting robot")
				go robot.Start()

				// Give the drone a chance to take off
				time.Sleep(5 * time.Second)

			case haltState:
				log.Println("Halting drone")
				drone.Halt()

				time.Sleep(10 * time.Second)
				currentFlightData = nil
				log.Println("Drone should be halted")
				p.Lock()
				p.droneState = initState
				p.Unlock()

				p.wg.Done()

			default:
				timer := time.NewTimer(5 * time.Second)
				select {
				case <-timer.C:
					if lostConnection {
						p.Lock()
						log.Printf("No updates for at least 5 seconds; current state %s\n", p.droneState)
						p.droneState = haltState
						p.Unlock()
					}
					log.Println("timer popped")
					lostConnection = true
				case <-timeoutChan:
					lostConnection = false
				}
			}
		}
	}()

	return drone
}

func getDroneState() string {
	p.Lock()
	defer p.Unlock()
	return p.droneState
}

func printFlightData(d *tello.FlightData) {
	if d == nil {
		fmt.Println(" -- no flight data")
	} else {
		fmt.Printf(" -- Battery: %d%% -- Height: %d \n", d.BatteryPercentage, d.Height)
	}
}
