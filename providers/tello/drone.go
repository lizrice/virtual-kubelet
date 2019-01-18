package tello

import (
	"fmt"
	"log"
	"time"

	"gobot.io/x/gobot"
	"gobot.io/x/gobot/platforms/dji/tello"
)

const (
	flightTime = 30 // number of seconds we will fly for
)

func droneConnect() (drone *tello.Driver) {
	log.Println("droneConnect")
	drone = tello.NewDriver("8888")
	return drone
}

func (p *TelloProvider) droneTimeoutWatcher() {
	lostConnection := false
	for {
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-p.timeoutChan:
			lostConnection = false
		case <-timer.C:
			if lostConnection {
				log.Println("No updates for at least 5 seconds")
				p.Lock()
				p.fd = nil
				p.Unlock()
				p.inputChan <- connectionLost
			}
			lostConnection = true
		}
	}
}

func getDroneState() state {
	p.Lock()
	defer p.Unlock()
	return p.droneState
}

func printFlightData(d *tello.FlightData) {
	if d != nil {
		fmt.Printf(" -- Battery: %d%% -- Height: %d \n", d.BatteryPercentage, d.Height)
	}
}

func droneStart(p *TelloProvider) {
	drone := p.drone
	work := func() {

		drone.On(tello.FlightDataEvent, func(data interface{}) {
			p.Lock()
			p.fd = data.(*tello.FlightData)
			p.Unlock()
			p.timeoutChan <- true
		})

		drone.On(tello.ConnectedEvent, func(data interface{}) {
			fmt.Println("Connected event from drone")
			p.inputChan <- connectionMade
			p.wg.Add(1)
		})

		drone.On(tello.TakeoffEvent, func(data interface{}) {
			fmt.Println("Take Off event from drone")
			p.inputChan <- takeOff
		})

		drone.On(tello.LandingEvent, func(data interface{}) {
			fmt.Println("Landing event from drone")
			p.inputChan <- land
			landingTimer := time.NewTimer(5 * time.Second)
			go func() {
				<-landingTimer.C
				fmt.Printf("Landing timer popped while state is %s\n", getDroneState())
				p.inputChan <- onGround
			}()
		})

		gobot.Every(1*time.Second, func() {
			printFlightData(p.fd)
		})
	}

	robot := gobot.NewRobot("tello",
		[]gobot.Connection{},
		[]gobot.Device{drone},
		work,
	)

	log.Println("Starting robot")
	go robot.Start()

	// Give the drone a chance to connect, and then we might try again
	time.Sleep(5 * time.Second)
	log.Println("Try connection again")
	p.inputChan <- tryConnection
}

func droneTakeOff(p *TelloProvider) {
	log.Println("Ask drone to take off")
	p.Lock()
	p.drone.TakeOff()
	p.Unlock()
}

func droneWaitForHeight(p *TelloProvider) {
	log.Println("Giving drone time to reach height")

	// TODO!! check drone height first before we say it's ready
	time.Sleep(3 * time.Second)

	if p.fd.Height == 0 {
		fmt.Println("Drone still on ground")
		p.inputChan <- onGround
	} else {
		p.inputChan <- atHeight
		takeOffTimer := time.NewTimer(flightTime * time.Second)
		go func() {
			<-takeOffTimer.C
			fmt.Printf("Flight timer popped while state is %s\n", getDroneState())
			p.inputChan <- flightTimeOver
		}()
	}
}

func droneReady(p *TelloProvider) {
	log.Println("Drone ready to do work")
	// TODO!! Set capacity so that a flip job could be scheduled

}

func droneLand(p *TelloProvider) {
	log.Println("Ask drone to land")
	p.Lock()
	p.drone.Land()
	p.Unlock()
}

func droneWaitForLand(p *TelloProvider) {
	log.Println("Giving drone time to land")

	// TODO!! check drone height first before we say it's ready
	time.Sleep(3 * time.Second)
	p.inputChan <- onGround
}

func droneHalt(p *TelloProvider) {
	log.Println("Halting drone")
	p.Lock()
	p.drone.Halt()
	p.Unlock()

	time.Sleep(5 * time.Second)
	p.inputChan <- done
}

func droneDone(p *TelloProvider) {
	log.Println("Drone is done")
	p.wg.Done()
}

func noOp(p *TelloProvider) {
	log.Println("Valid transition, no action")
}
