package silly

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// You can only run one silly local virtual-kubelet, and it's represented by this global variable
var p SillyProvider

// SillyProvider implements the virtual-kubelet provider interface and runs locally, pretending to run pods
type SillyProvider struct {
	resourceManager *manager.ResourceManager
	nodeName        string
	operatingSystem string
	pods            map[string]*v1.Pod
	startTime       time.Time
}

// PodInfo is information you can query about pods from the silly local virtual-kubelet's server
type PodInfo struct {
	Name string
}

// PodList is the list of pods returned by silly local virtual-kubelet's server
type PodList struct {
	Pods []PodInfo
}

func handler(w http.ResponseWriter, r *http.Request) {
	var podInfo []PodInfo
	var podlist PodList
	for _, pod := range p.pods {
		podInfo = append(podInfo, PodInfo{Name: pod.Name})
	}
	podlist.Pods = podInfo
	json.NewEncoder(w).Encode(podInfo)
}

// NewProvider creates a new Silly Provider
func NewProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string) (*SillyProvider, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("must specify a node name")
	}

	if p.nodeName != "" {
		return nil, fmt.Errorf("silly local virtual-kubelet already exists")
	}
	p.resourceManager = rm
	p.nodeName = nodeName
	p.operatingSystem = operatingSystem
	p.pods = make(map[string]*v1.Pod)
	p.startTime = time.Now()

	http.HandleFunc("/", handler)
	go http.ListenAndServe(":8080", nil)
	return &p, nil
}

// CreatePod accepts a Pod definition and keeps a record of it locally.
func (p *SillyProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	pod.Status.Phase = "Running"
	p.pods[pod.Name] = pod

	// TODO: record the start time
	return nil
}

// UpdatePod replaces the current pod definition with the new version.
func (p *SillyProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	if _, ok := p.pods[pod.Name]; !ok {
		return fmt.Errorf("pod %s not found", pod.Name)
	}
	p.pods[pod.Name] = pod
	return nil
}

// DeletePod deletes the specified pod. Noop if the pod isn't found.
func (p *SillyProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	delete(p.pods, pod.Name)
	return nil
}

// GetPod returns a pod by name if the namespace matches or is empty.
// returns nil if a pod by that name is not found.
func (p *SillyProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
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
func (p *SillyProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	log.Printf("receive GetContainerLogs %q\n", podName)
	return "", nil
}

// GetPodStatus returns the status of a pod by name that has been created locally.
// returns nil if a pod by that name is not found.
func (p *SillyProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, ok := p.pods[name]
	if !ok {
		return nil, nil
	}

	return &pod.Status, nil
}

// GetPods returns a list of all pods created locally.
func (p *SillyProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	var pods []*v1.Pod
	for _, pod := range p.pods {
		pods = append(pods, pod)
	}
	return pods, nil
}

// Capacity returns a resource list containing the capacity limits that we'll make up for local things.
func (p *SillyProvider) Capacity(txc context.Context) v1.ResourceList {
	// TODO: Set something sensible
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of node conditions. For the Local provider it's always Ready.
func (p *SillyProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	return []v1.NodeCondition{
		{
			Type:               "Ready",
			Status:             v1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "local kubelet was born ready.",
		},
	}
}

// OperatingSystem returns the operating system for this provider.
// This is a noop to default to Linux for now.
func (p *SillyProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *SillyProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *SillyProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	return []v1.NodeAddress{
		{
			Type:    "HostName",
			Address: p.nodeName,
		},
	}
}

// NodeDaemonEndpoints returns NodeDaemonEndpoints for the node status
// within Kubernetes.
func (p *SillyProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{}
}

// GetStatsSummary returns dummy stats for all pods known by this provider.
func (p *SillyProvider) GetStatsSummary(ctx context.Context) (*stats.Summary, error) {
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
