package tello

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"gobot.io/x/gobot/platforms/dji/tello"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// You can only run one tello local virtual-kubelet, and it's represented by this global variable
var p TelloProvider

// TelloProvider implements the virtual-kubelet provider interface and runs locally, pretending to run pods
// and allowing you to send commands to a locally connected Tello drone
type TelloProvider struct {
	sync.Mutex
	resourceManager    *manager.ResourceManager
	nodeName           string
	operatingSystem    string
	pods               map[string]*v1.Pod
	startTime          time.Time
	drone              *tello.Driver
	droneState         string
	droneStateChan     chan string
	lastTransitionTime metav1.Time
	wg                 sync.WaitGroup
}

// PodInfo is information you can query about pods from the tello local virtual-kubelet's server
type PodInfo struct {
	Name string
}

// PodList is the list of pods returned by tello local virtual-kubelet's server
type PodList struct {
	Pods []PodInfo
}

// NewProvider creates a new Tello Provider
func NewProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string) (*TelloProvider, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("must specify a node name")
	}

	if p.nodeName != "" {
		return nil, fmt.Errorf("tello local virtual-kubelet already exists")
	}

	p.resourceManager = rm
	p.nodeName = nodeName
	p.operatingSystem = operatingSystem
	p.pods = make(map[string]*v1.Pod)
	p.startTime = time.Now()
	p.drone = droneConnect()
	p.droneStateChan = make(chan string)

	go p.updateDroneStatus()

	return &p, nil
}

// Close allows us to land the drone before the program finishes
func (p *TelloProvider) Close() {
	fmt.Println("Close tello provider - wait for completion")
	p.Lock()
	defer p.Unlock()
	p.droneState = haltState

	p.wg.Wait()
}

func (p *TelloProvider) updateDroneStatus() {
	for {
		select {
		case state := <-p.droneStateChan:
			log.Printf("Drone state changed: %s\n", state)
			p.Lock()
			p.droneState = state
			p.lastTransitionTime = metav1.Now()
			p.Unlock()
		}
	}
}

// CreatePod accepts a Pod definition and keeps a record of it locally.
func (p *TelloProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	p.Lock()
	defer p.Unlock()

	pod.Status.Phase = "Running"
	p.pods[pod.Name] = pod

	p.drone.Flip(tello.FlipBack)
	// TODO: record the start time
	return nil
}

// UpdatePod replaces the current pod definition with the new version.
func (p *TelloProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.pods[pod.Name]; !ok {
		return fmt.Errorf("pod %s not found", pod.Name)
	}
	p.pods[pod.Name] = pod
	return nil
}

// DeletePod deletes the specified pod. Noop if the pod isn't found.
func (p *TelloProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	p.Lock()
	defer p.Unlock()

	delete(p.pods, pod.Name)
	return nil
}

// GetPod returns a pod by name if the namespace matches or is empty.
// returns nil if a pod by that name is not found.
func (p *TelloProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	p.Lock()
	defer p.Unlock()

	pod, ok := p.pods[name]
	if !ok {
		return nil, nil
	}

	if namespace == "" || namespace == pod.ObjectMeta.Namespace {
		return pod, nil
	}

	return nil, nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *TelloProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	p.Lock()
	defer p.Unlock()

	log.Printf("receive GetContainerLogs %q\n", podName)
	return "", nil
}

// GetPodStatus returns the status of a pod by name that has been created locally.
// returns nil if a pod by that name is not found.
func (p *TelloProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	p.Lock()
	defer p.Unlock()

	pod, ok := p.pods[name]
	if !ok {
		return nil, nil
	}

	return &pod.Status, nil
}

// GetPods returns a list of all pods created locally.
func (p *TelloProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	p.Lock()
	defer p.Unlock()

	var pods []*v1.Pod
	for _, pod := range p.pods {
		pods = append(pods, pod)
	}
	return pods, nil
}

// Capacity returns a resource list containing the capacity limits that we'll make up for local things.
func (p *TelloProvider) Capacity(txc context.Context) v1.ResourceList {
	p.Lock()
	defer p.Unlock()

	// TODO: Set something sensible
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of node conditions. For the Local provider it's always Ready.
func (p *TelloProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	var nc v1.NodeCondition

	nc.Reason = p.droneState
	switch p.droneState {
	case takeOffState:
		nc.Type = v1.NodeReady
		nc.Status = v1.ConditionTrue
		nc.Message = "Tello ready for commands"

	default:
		nc.Type = "NotReady"
		nc.Status = v1.ConditionFalse
		switch p.droneState {
		case initState:
			nc.Message = "Tello virtual kubelet initializing"
		case landingState:
			nc.Message = "Tello landed"
		case connectedState:
			nc.Message = "Tello connected, waiting for takeoff"
		case haltState:
			nc.Message = "Tello halting"
		}
	}

	nc.LastHeartbeatTime = metav1.Now()
	nc.LastTransitionTime = p.lastTransitionTime
	return []v1.NodeCondition{nc}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *TelloProvider) OperatingSystem() string {
	p.Lock()
	defer p.Unlock()

	return providers.OperatingSystemLinux
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *TelloProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	p.Lock()
	defer p.Unlock()

	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *TelloProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	p.Lock()
	defer p.Unlock()

	return []v1.NodeAddress{
		{
			Type:    "HostName",
			Address: p.nodeName,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *TelloProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	p.Lock()
	defer p.Unlock()

	return &v1.NodeDaemonEndpoints{}
}

// GetStatsSummary returns dummy stats for all pods known by this provider.
func (p *TelloProvider) GetStatsSummary(ctx context.Context) (*stats.Summary, error) {
	p.Lock()
	defer p.Unlock()

	// Create the Summary object that will later be populated with node and pod stats.
	res := &stats.Summary{}

	// Populate the Summary object with basic node stats.
	res.Node = stats.NodeStats{
		NodeName:  p.nodeName,
		StartTime: metav1.NewTime(p.startTime),
	}

	// Populate the Summary object with dummy stats for each pod known by this provider.
	for _, pod := range p.pods {
		// Create a PodStats object to populate with pod stats.
		pss := stats.PodStats{
			PodRef: stats.PodReference{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       string(pod.UID),
			},
			StartTime: pod.CreationTimestamp,
		}

		// Iterate over all containers in the current pod to compute dummy stats.
		for _, container := range pod.Spec.Containers {
			pss.Containers = append(pss.Containers, stats.ContainerStats{
				Name:      container.Name,
				StartTime: pod.CreationTimestamp,
			})
		}

		res.Pods = append(res.Pods, pss)
	}

	// Return the dummy stats.
	return res, nil
}
