package main

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers/tello"

	"context"
	"time"

	"k8s.io/api/core/v1"
)

func main() {
	t, err := tello.NewProvider("main", nil, "liz", "os")
	if err != nil {
		panic("Couldn't start provider")
	}

	time.Sleep(10 * time.Second)

	t.CreatePod(context.TODO(), &v1.Pod{})

	time.Sleep(15 * time.Second)

	t.Close()
}
