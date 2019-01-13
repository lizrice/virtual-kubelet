package tello

import (
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate stringer -type state
type state int

//go:generate stringer -type input
type input int
type action func(*TelloProvider)
type transition struct {
	newState state
	do       action
}

type validStates [stateSize][inputSize]transition

const (
	disconnected state = iota // No connection to drone
	connected                 // Connected but not taken off yet
	takingOff                 // Taking off
	ready                     // Ready for commands
	landing                   // Landing
	stateSize                 // Only used for the lengh of state
)

const (
	tryConnection  input = iota // Please try connecting to drone
	connectionMade              // Received connect event from drone
	takeOff                     // Received take off event from drone
	atHeight                    // Checked height, and it was > 0
	flightTimeOver              // Flight duration timer popped
	land                        // Landing event from drone
	onGround                    // Checked height, and it was 0
	halt                        // Please try to halt drone
	connectionLost              // Connection to drone lost (after timeout)
	done                        // Drone freed
	inputSize                   // Only used for the length of signal
)

// getValidStates returns the valid state transitions
func getValidStates() (vs validStates) {
	vs.setState(disconnected, connectionMade, connected, droneTakeOff)
	vs.setState(disconnected, tryConnection, disconnected, droneStart)
	vs.setState(disconnected, connectionLost, disconnected, noOp)
	vs.setState(disconnected, onGround, disconnected, noOp)
	vs.setState(disconnected, halt, disconnected, noOp)

	vs.setState(connected, takeOff, takingOff, droneWaitForHeight)
	vs.setState(connected, tryConnection, connected, noOp)
	vs.setState(connected, connectionLost, connected, noOp)
	vs.setState(connected, halt, landing, droneHalt)

	vs.setState(takingOff, tryConnection, takingOff, noOp)
	vs.setState(takingOff, takeOff, takingOff, noOp)
	vs.setState(takingOff, onGround, takingOff, droneTakeOff)
	vs.setState(takingOff, atHeight, ready, droneReady)
	vs.setState(takingOff, connectionLost, landing, droneHalt)
	vs.setState(takingOff, halt, landing, droneLand)

	vs.setState(ready, tryConnection, ready, noOp)
	vs.setState(ready, flightTimeOver, landing, droneLand)
	vs.setState(ready, land, landing, noOp)
	vs.setState(ready, connectionLost, landing, droneLand)
	vs.setState(ready, halt, landing, droneLand)

	vs.setState(landing, onGround, landing, droneHalt)
	vs.setState(landing, connectionLost, landing, droneHalt)
	vs.setState(landing, land, landing, noOp)
	vs.setState(landing, halt, landing, noOp)
	vs.setState(landing, onGround, landing, droneHalt)
	vs.setState(landing, done, disconnected, droneDone)
	return vs
}

func (vs *validStates) setState(s state, i input, n state, a action) {
	vs[s][i] = transition{newState: n, do: a}
}

func (p *TelloProvider) droneStateMachine() {
	validStates := getValidStates()
	log.Println("Valid states obtained")

	for {
		select {
		case input := <-p.inputChan:
			p.Lock()
			log.Printf("Drone state input %s in state %s\n", input, p.droneState)
			t := validStates[p.droneState][input]
			if t.do == nil {
				log.Println("** INVALID STATE TRANSITION")
			} else {
				p.droneState = t.newState
				p.lastTransitionTime = metav1.Now()
			}

			p.Unlock()

			if t.do != nil {
				go t.do(p)
			}
		}
	}
}
