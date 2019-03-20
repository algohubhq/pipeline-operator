// Copyright 2018 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1 "github.com/coreos/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeMonitoringV1 struct {
	*testing.Fake
}

func (c *FakeMonitoringV1) Alertmanagers(namespace string) v1.AlertmanagerInterface {
	return &FakeAlertmanagers{c, namespace}
}

func (c *FakeMonitoringV1) Prometheuses(namespace string) v1.PrometheusInterface {
	return &FakePrometheuses{c, namespace}
}

func (c *FakeMonitoringV1) PrometheusRules(namespace string) v1.PrometheusRuleInterface {
	return &FakePrometheusRules{c, namespace}
}

func (c *FakeMonitoringV1) ServiceMonitors(namespace string) v1.ServiceMonitorInterface {
	return &FakeServiceMonitors{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeMonitoringV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
