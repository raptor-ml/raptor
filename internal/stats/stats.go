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
func IncNumberOfFeatures() {
	numOfFeatures.Inc()
}
func DecNumberOfFeatures() {
	numOfFeatures.Dec()
}
func IncrFeatureGets() {
	featureGets.Inc()
}
func IncrFeatureSets() {
	featureSets.Inc()
}
func IncrFeatureUpdates() {
	featureUpdates.Inc()
}
func IncrFeatureAppends() {
	featureAppends.Inc()
}
func IncrFeatureIncrements() {
	featureIncrements.Inc()
}
func IncrFeatureDescriptorReqs() {
	fdReqs.Inc()
}
