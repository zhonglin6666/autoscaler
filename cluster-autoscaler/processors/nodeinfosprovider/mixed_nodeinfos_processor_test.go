/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodeinfosprovider

import (
	"testing"
	"time"

	testprovider "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/test"
	"k8s.io/autoscaler/cluster-autoscaler/context"
	"k8s.io/autoscaler/cluster-autoscaler/simulator"
	kube_util "k8s.io/autoscaler/cluster-autoscaler/utils/kubernetes"
	. "k8s.io/autoscaler/cluster-autoscaler/utils/test"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	schedulerframework "k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestGetNodeInfosForGroups(t *testing.T) {
	ready1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(ready1, true, time.Now())
	ready2 := BuildTestNode("n2", 2000, 2000)
	SetNodeReadyState(ready2, true, time.Now())
	unready3 := BuildTestNode("n3", 3000, 3000)
	SetNodeReadyState(unready3, false, time.Now())
	unready4 := BuildTestNode("n4", 4000, 4000)
	SetNodeReadyState(unready4, false, time.Now())

	tn := BuildTestNode("tn", 5000, 5000)
	tni := schedulerframework.NewNodeInfo()
	tni.SetNode(tn)

	// Cloud provider with TemplateNodeInfo implemented.
	provider1 := testprovider.NewTestAutoprovisioningCloudProvider(
		nil, nil, nil, nil, nil,
		map[string]*schedulerframework.NodeInfo{"ng3": tni, "ng4": tni})
	provider1.AddNodeGroup("ng1", 1, 10, 1) // Nodegroup with ready node.
	provider1.AddNode("ng1", ready1)
	provider1.AddNodeGroup("ng2", 1, 10, 1) // Nodegroup with ready and unready node.
	provider1.AddNode("ng2", ready2)
	provider1.AddNode("ng2", unready3)
	provider1.AddNodeGroup("ng3", 1, 10, 1) // Nodegroup with unready node.
	provider1.AddNode("ng3", unready4)
	provider1.AddNodeGroup("ng4", 0, 1000, 0) // Nodegroup without nodes.

	// Cloud provider with TemplateNodeInfo not implemented.
	provider2 := testprovider.NewTestAutoprovisioningCloudProvider(nil, nil, nil, nil, nil, nil)
	provider2.AddNodeGroup("ng5", 1, 10, 1) // Nodegroup without nodes.

	podLister := kube_util.NewTestPodLister([]*apiv1.Pod{})
	registry := kube_util.NewListerRegistry(nil, nil, podLister, nil, nil, nil, nil, nil, nil, nil)

	predicateChecker, err := simulator.NewTestPredicateChecker()
	assert.NoError(t, err)

	ctx := context.AutoscalingContext{
		CloudProvider:    provider1,
		PredicateChecker: predicateChecker,
		AutoscalingKubeClients: context.AutoscalingKubeClients{
			ListerRegistry: registry,
		},
	}
	res, err := NewMixedTemplateNodeInfoProvider().Process([]*apiv1.Node{unready4, unready3, ready2, ready1}, &ctx, []*appsv1.DaemonSet{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(res))
	info, found := res["ng1"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready1, info.Node())
	info, found = res["ng2"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready2, info.Node())
	info, found = res["ng3"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, tn, info.Node())
	info, found = res["ng4"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, tn, info.Node())

	// Test for a nodegroup without nodes and TemplateNodeInfo not implemented by cloud proivder
	ctx = context.AutoscalingContext{
		CloudProvider:    provider2,
		PredicateChecker: predicateChecker,
		AutoscalingKubeClients: context.AutoscalingKubeClients{
			ListerRegistry: registry,
		},
	}
	res, err = NewMixedTemplateNodeInfoProvider().Process([]*apiv1.Node{}, &ctx, []*appsv1.DaemonSet{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(res))
}

func TestGetNodeInfosForGroupsCache(t *testing.T) {
	ready1 := BuildTestNode("n1", 1000, 1000)
	SetNodeReadyState(ready1, true, time.Now())
	ready2 := BuildTestNode("n2", 2000, 2000)
	SetNodeReadyState(ready2, true, time.Now())
	unready3 := BuildTestNode("n3", 3000, 3000)
	SetNodeReadyState(unready3, false, time.Now())
	unready4 := BuildTestNode("n4", 4000, 4000)
	SetNodeReadyState(unready4, false, time.Now())
	ready5 := BuildTestNode("n5", 5000, 5000)
	SetNodeReadyState(ready5, true, time.Now())
	ready6 := BuildTestNode("n6", 6000, 6000)
	SetNodeReadyState(ready6, true, time.Now())

	tn := BuildTestNode("tn", 10000, 10000)
	tni := schedulerframework.NewNodeInfo()
	tni.SetNode(tn)

	lastDeletedGroup := ""
	onDeleteGroup := func(id string) error {
		lastDeletedGroup = id
		return nil
	}

	// Cloud provider with TemplateNodeInfo implemented.
	provider1 := testprovider.NewTestAutoprovisioningCloudProvider(
		nil, nil, nil, onDeleteGroup, nil,
		map[string]*schedulerframework.NodeInfo{"ng3": tni, "ng4": tni})
	provider1.AddNodeGroup("ng1", 1, 10, 1) // Nodegroup with ready node.
	provider1.AddNode("ng1", ready1)
	provider1.AddNodeGroup("ng2", 1, 10, 1) // Nodegroup with ready and unready node.
	provider1.AddNode("ng2", ready2)
	provider1.AddNode("ng2", unready3)
	provider1.AddNodeGroup("ng3", 1, 10, 1) // Nodegroup with unready node (and 1 previously ready node).
	provider1.AddNode("ng3", unready4)
	provider1.AddNode("ng3", ready5)
	provider1.AddNodeGroup("ng4", 0, 1000, 0) // Nodegroup without nodes (and 1 previously ready node).
	provider1.AddNode("ng4", ready6)

	podLister := kube_util.NewTestPodLister([]*apiv1.Pod{})
	registry := kube_util.NewListerRegistry(nil, nil, podLister, nil, nil, nil, nil, nil, nil, nil)

	predicateChecker, err := simulator.NewTestPredicateChecker()
	assert.NoError(t, err)

	// Fill cache
	ctx := context.AutoscalingContext{
		CloudProvider:    provider1,
		PredicateChecker: predicateChecker,
		AutoscalingKubeClients: context.AutoscalingKubeClients{
			ListerRegistry: registry,
		},
	}
	niProcessor := NewMixedTemplateNodeInfoProvider()
	res, err := niProcessor.Process([]*apiv1.Node{unready4, unready3, ready2, ready1}, &ctx, []*appsv1.DaemonSet{}, nil)
	assert.NoError(t, err)
	// Check results
	assert.Equal(t, 4, len(res))
	info, found := res["ng1"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready1, info.Node())
	info, found = res["ng2"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready2, info.Node())
	info, found = res["ng3"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, tn, info.Node())
	info, found = res["ng4"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, tn, info.Node())
	// Check cache
	cachedInfo, found := niProcessor.nodeInfoCache["ng1"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready1, cachedInfo.Node())
	cachedInfo, found = niProcessor.nodeInfoCache["ng2"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready2, cachedInfo.Node())
	cachedInfo, found = niProcessor.nodeInfoCache["ng3"]
	assert.False(t, found)
	cachedInfo, found = niProcessor.nodeInfoCache["ng4"]
	assert.False(t, found)

	// Invalidate part of cache in two different ways
	provider1.DeleteNodeGroup("ng1")
	provider1.GetNodeGroup("ng3").Delete()
	assert.Equal(t, "ng3", lastDeletedGroup)

	// Check cache with all nodes removed
	res, err = niProcessor.Process([]*apiv1.Node{unready4, unready3, ready2, ready1}, &ctx, []*appsv1.DaemonSet{}, nil)
	assert.NoError(t, err)
	// Check results
	assert.Equal(t, 2, len(res))
	info, found = res["ng2"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready2, info.Node())
	// Check ng4 result and cache
	info, found = res["ng4"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, tn, info.Node())
	// Check cache
	cachedInfo, found = niProcessor.nodeInfoCache["ng2"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready2, cachedInfo.Node())
	cachedInfo, found = niProcessor.nodeInfoCache["ng4"]
	assert.False(t, found)

	// Fill cache manually
	infoNg4Node6 := schedulerframework.NewNodeInfo()
	infoNg4Node6.SetNode(ready6.DeepCopy())
	niProcessor.nodeInfoCache = map[string]*schedulerframework.NodeInfo{"ng4": infoNg4Node6}
	res, err = niProcessor.Process([]*apiv1.Node{unready4, unready3, ready2, ready1}, &ctx, []*appsv1.DaemonSet{}, nil)
	// Check if cache was used
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
	info, found = res["ng2"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready2, info.Node())
	info, found = res["ng4"]
	assert.True(t, found)
	assertEqualNodeCapacities(t, ready6, info.Node())
}

func assertEqualNodeCapacities(t *testing.T, expected, actual *apiv1.Node) {
	t.Helper()
	assert.Equal(t, getNodeResource(expected, apiv1.ResourceCPU), getNodeResource(actual, apiv1.ResourceCPU), "CPU should be the same")
	assert.Equal(t, getNodeResource(expected, apiv1.ResourceMemory), getNodeResource(actual, apiv1.ResourceMemory), "Memory should be the same")
}

func getNodeResource(node *apiv1.Node, resource apiv1.ResourceName) int64 {
	nodeCapacity, found := node.Status.Capacity[resource]
	if !found {
		return 0
	}

	nodeCapacityValue := nodeCapacity.Value()
	if nodeCapacityValue < 0 {
		nodeCapacityValue = 0
	}

	return nodeCapacityValue
}
