package strategy

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"fmt"
)

type Strategy interface {
	// Add new objects
	Add(obj interface{}) error

	// Update objects
	Update(obj interface{}) error

	// Delete objects
	Delete(obj interface{}) error
}

type predictiveStrategy struct {
	resourceStore store.ResourceStore

	// for each node keep its NodeInfo
	nodeInfo map[string]*schedulercache.NodeInfo
}

func newStubPod(nodeName string, node *api.Node) *api.Pod {
	// Generate unique pod's name.
	// Given all new pods get's name pod-INDEX or similar,
	// node's NodeInfo's pod names will never collide.
	// pod-stub or similar is sufficient.

	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: "pod-stub", Namespace: nodeName, ResourceVersion: "10"},
		Spec:       apitesting.DeepEqualSafePodSpec(),
	}

	limitResourceList := make(map[api.ResourceName]resource.Quantity)
	requestsResourceList := make(map[api.ResourceName]resource.Quantity)
	for key, value := range node.Status.Allocatable {
		limitResourceList[key] = value
		requestsResourceList[key] = value
	}

	// set pod's resource consumption
	pod.Spec.Containers = []api.Container{
		{
			Resources: api.ResourceRequirements{
				Limits: limitResourceList,
				Requests: requestsResourceList,
			},
		},
	}

	// schedule the pod on the node
	pod.Spec.NodeName = testStrategyNode

	return pod
}

func (s *predictiveStrategy) addPod(pod *api.Pod) error {
	// 1. get node the pod is scheduled on
	node := &api.Node{
		ObjectMeta: api.ObjectMeta{Name: pod.Spec.NodeName},
	}
	foundNode, exists, err := s.resourceStore.Get("nodes", meta.Object(node))
	if err != nil {
		return fmt.Errorf("Unable to get node: %v", err)
	}
	if !exists {
		return fmt.Errorf("Unable to find scheduled pod's node")
	}
	scheduledNode := foundNode.(api.Node)
	fmt.Println(scheduledNode)

	// 2. update the node info to include new pod's resources
	info, exists := s.nodeInfo[pod.Spec.NodeName]
	if !exists {
		// create stub pod representing the captured node's allocated resources
		stub := newStubPod(pod.Spec.NodeName, &scheduledNode)

		// create node info with the pod
		info = schedulercache.NewNodeInfo(stub)
	}

	fmt.Println(info)

	// The node's allocated resources are grabed from the system (cgroup's)
	// so the schedulercache.NodeInfo as actually never used.
	// Some pods can even be without resource limits.
	// However, the point of the cluster capacity is to calculate how many instances
	// of a pod of a given shape (with specified limits) can be scheduled.
	// Thus the assumption here is the operator choose the limits the way
	// they would be actualy consumed by the underlying system and thus
	// calculated as a sum of the current captured allocation + multiple of addition of all pod's instances.


	// So, get the node's allocatable, create a pod that represents that amount
	// and populate node's NodeInfo with the pod.
	// Once new pods get added to the info, resources get re-computed accordingly to the assumption.

	// 3. reflect new pod and update node in the resource store
	return nil
}

// Simulate creation of new object (only pods currently supported)
// The method returns error on the first occurence of processing error.
// If so, all succesfully processed objects up to the first failed are reflected in the resource store.
func (s *predictiveStrategy) Add(obj interface{}) error {
	switch item := obj.(type) {
		case *api.Pod:
			return s.addPod(item)
		default:
			return fmt.Errorf("resource kind not recognized")
	}
}

func (s *predictiveStrategy) Update(obj interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func (s *predictiveStrategy) Delete(obj interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func NewPredictiveStrategy(resourceStore store.ResourceStore) *predictiveStrategy {
	return &predictiveStrategy{
		resourceStore: resourceStore,
	}
}
