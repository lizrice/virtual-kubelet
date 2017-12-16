package silly

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// You can only run one silly local virtual-kubelet, and it's represented by this global variable
var p LocalProvider

// LocalProvider implements the virtual-kubelet provider interface and runs locally, pretending to run pods
type LocalProvider struct {
	resourceManager *manager.ResourceManager
	nodeName        string
	operatingSystem string
	pods            map[string]*v1.Pod
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

// NewLocalProvider creates a new LocalProvider
func NewLocalProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string) (*LocalProvider, error) {
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

	http.HandleFunc("/", handler)
	go http.ListenAndServe(":8080", nil)
	return &p, nil
}

// CreatePod accepts a Pod definition and keeps a record of it locally.
func (p *LocalProvider) CreatePod(pod *v1.Pod) error {
	pod.Status.Phase = "Running"
	p.pods[pod.Name] = pod

	// TODO: record the start time
	return nil
}

// UpdatePod replaces the current pod definition with the new version.
func (p *LocalProvider) UpdatePod(pod *v1.Pod) error {
	if _, ok := p.pods[pod.Name]; !ok {
		return fmt.Errorf("pod %s not found", pod.Name)
	}
	p.pods[pod.Name] = pod
	return nil
}

// DeletePod deletes the specified pod. Noop if the pod isn't found.
func (p *LocalProvider) DeletePod(pod *v1.Pod) error {
	delete(p.pods, pod.Name)
	return nil
}

// GetPod returns a pod by name if the namespace matches or is empty.
// returns nil if a pod by that name is not found.
func (p *LocalProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	pod, ok := p.pods[name]
	if !ok {
		return nil, nil
	}

	if namespace == "" || namespace == pod.ObjectMeta.Namespace {
		return pod, nil
	}

	return nil, nil
}

// GetPodStatus returns the status of a pod by name that has been created locally.
// returns nil if a pod by that name is not found.
func (p *LocalProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	pod, ok := p.pods[name]
	if !ok {
		return nil, nil
	}

	return &pod.Status, nil
}

// GetPods returns a list of all pods created locally.
func (p *LocalProvider) GetPods() ([]*v1.Pod, error) {
	var pods []*v1.Pod
	for _, pod := range p.pods {
		pods = append(pods, pod)
	}
	return pods, nil
}

// Capacity returns a resource list containing the capacity limits that we'll make up for local things.
func (p *LocalProvider) Capacity() v1.ResourceList {
	// TODO: Set something sensible
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of node conditions. For the Local provider it's always Ready.
func (p *LocalProvider) NodeConditions() []v1.NodeCondition {
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
func (p *LocalProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}
