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

package core

import (
	"time"

	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kube_record "k8s.io/kubernetes/pkg/client/record"

	"k8s.io/contrib/cluster-autoscaler/config/dynamic"
	"k8s.io/contrib/cluster-autoscaler/simulator"
)

// AutoscalerOptions is the whole set of options for configuring an autoscaler
type AutoscalerOptions struct {
	AutoscalingOptions
	dynamic.ConfigFetcherOptions
}

// Autoscaler is the main component of CA which scales up/down node groups according to its configuration
// The configuration can be injected at the creation of an autoscaler
type Autoscaler interface {
	// RunOnce represents an iteration in the control-loop of CA
	RunOnce(currentTime time.Time)
}

// NewAutoscaler creates an autoscaler of an appropriate type according to the parameters
func NewAutoscaler(opts AutoscalerOptions, predicateChecker *simulator.PredicateChecker, kubeClient kube_client.Interface, kubeEventRecorder kube_record.EventRecorder) Autoscaler {
	var autoscaler Autoscaler
	if opts.ConfigMapName != "" {
		autoscalerBuilder := NewAutoscalerBuilder(opts.AutoscalingOptions, predicateChecker, kubeClient, kubeEventRecorder)
		configFetcher := dynamic.NewConfigFetcher(opts.ConfigFetcherOptions, kubeClient, kubeEventRecorder)
		autoscaler = NewDynamicAutoscaler(autoscalerBuilder, configFetcher)
	} else {
		autoscaler = NewStaticAutoscaler(opts.AutoscalingOptions, predicateChecker, kubeClient, kubeEventRecorder)
	}
	return autoscaler
}
