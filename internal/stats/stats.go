/*
Copyright (c) 2022 RaptorML authors.

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

package stats

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	numOfFeatures = prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: coreSubsystemKey,
		Name:      "number_of_features",
		Help:      "Number of features that are being served by the engine.",
	})
	featureGets = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: coreSubsystemKey,
		Name:      "number_of_feature_gets",
		Help:      "Number of features GET requests.",
	})
	featureSets = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: coreSubsystemKey,
		Name:      "number_of_feature_sets",
		Help:      "Number of features SET requests.",
	})
	featureUpdates = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: coreSubsystemKey,
		Name:      "number_of_feature_updates",
		Help:      "Number of features UPDATE requests.",
	})
	featureAppends = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: coreSubsystemKey,
		Name:      "number_of_feature_appends",
		Help:      "Number of features APPEND requests.",
	})
	featureIncrements = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: coreSubsystemKey,
		Name:      "number_of_feature_increments",
		Help:      "Number of features INCR requests.",
	})
	fdReqs = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: coreSubsystemKey,
		Name:      "number_of_fd_reqs",
		Help:      "Number of FeatureDescriptor requests.",
	})
)

func init() {
	prometheus.MustRegister(
		numOfFeatures,
		featureGets,
		featureSets,
		featureUpdates,
		featureAppends,
		featureIncrements,
		fdReqs,
	)
}

// IncNumberOfFeatures increments the number of features that are being served by the engine.
func IncNumberOfFeatures() {
	numOfFeatures.Inc()
}

// DecNumberOfFeatures decrements the number of features that are being served by the engine.
func DecNumberOfFeatures() {
	numOfFeatures.Dec()
}

// IncrFeatureGets increments the number of feature `Get` requests.
func IncrFeatureGets() {
	featureGets.Inc()
}

// IncrFeatureSets increments the number of feature `Set` requests.
func IncrFeatureSets() {
	featureSets.Inc()
}

// IncrFeatureUpdates increments the number of feature `Update` requests.
func IncrFeatureUpdates() {
	featureUpdates.Inc()
}

// IncrFeatureAppends increments the number of feature `Append` requests.
func IncrFeatureAppends() {
	featureAppends.Inc()
}

// IncrFeatureIncrements increments the number of feature `Increment` requests.
func IncrFeatureIncrements() {
	featureIncrements.Inc()
}

// IncrFeatureDescriptorReqs increments the number of feature descriptor requests.
func IncrFeatureDescriptorReqs() {
	fdReqs.Inc()
}
