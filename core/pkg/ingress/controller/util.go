/*
Copyright 2015 The Kubernetes Authors.

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

package controller

import (
	"strings"

	"github.com/golang/glog"
	"github.com/imdario/mergo"

	"k8s.io/kubernetes/pkg/apis/extensions"

	"k8s.io/ingress/core/pkg/ingress"
	"k8s.io/ingress/core/pkg/ingress/annotations/parser"
	"k8s.io/ingress/core/pkg/ingress/errors"
	"k8s.io/ingress/core/pkg/k8s"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

const ingressLoadbalancerAnno = "ingress.alpha.k8s.io/loadbalancer-name"

// DeniedKeyName name of the key that contains the reason to deny a location
const DeniedKeyName = "Denied"

// newDefaultServer return an BackendServer to be use as default server that returns 503.
func newDefaultServer() ingress.Endpoint {
	return ingress.Endpoint{Address: "127.0.0.1", Port: "8181"}
}

// newUpstream creates an upstream without servers.
func newUpstream(name string) *ingress.Backend {
	return &ingress.Backend{
		Name:      name,
		Endpoints: []ingress.Endpoint{},
	}
}

func isHostValid(host string, cert *ingress.SSLCert) bool {
	if cert == nil {
		return false
	}
	for _, cn := range cert.CN {
		if matchHostnames(cn, host) {
			return true
		}
	}

	return false
}

func matchHostnames(pattern, host string) bool {
	host = strings.TrimSuffix(host, ".")
	pattern = strings.TrimSuffix(pattern, ".")

	if len(pattern) == 0 || len(host) == 0 {
		return false
	}

	patternParts := strings.Split(pattern, ".")
	hostParts := strings.Split(host, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if i == 0 && patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}

	return true
}

// IsValidClass returns true if the given Ingress either doesn't specify
// the ingress.class annotation, or it's set to the configured in the
// ingress controller.
func IsValidClass(ing *extensions.Ingress, config *Configuration) bool {
	currentIngClass := config.IngressClass

	cc, err := parser.GetStringAnnotation(ingressClassKey, ing)
	if err != nil && !errors.IsMissingAnnotations(err) {
		glog.Warningf("unexpected error reading ingress annotation: %v", err)
	}

	// we have 2 valid combinations
	// 1 - ingress with default class | blank annotation on ingress
	// 2 - ingress with specific class | same annotation on ingress
	//
	// and 2 invalid combinations
	// 3 - ingress with default class | fixed annotation on ingress
	// 4 - ingress with specific class | different annotation on ingress
	if (cc == "" && currentIngClass == "") || (currentIngClass == config.DefaultIngressClass) {
		return true
	}

	return cc == currentIngClass
}

func isAssignedToSelf(ing *extensions.Ingress, client *clientset.Clientset) bool {
	if ing.Annotations == nil {
		return false
	}

	podInfo, err := k8s.GetPodDetails(client)
	if err != nil {
		glog.Fatalf("unexpected error obtaining pod information: %v", err)
	}

	return strings.HasPrefix(ing.Annotations[ingressLoadbalancerAnno], podInfo.Name)
}

func mergeLocationAnnotations(loc *ingress.Location, anns map[string]interface{}) {
	if _, ok := anns[DeniedKeyName]; ok {
		loc.Denied = anns[DeniedKeyName].(error)
	}
	delete(anns, DeniedKeyName)
	err := mergo.Map(loc, anns)
	if err != nil {
		glog.Errorf("unexpected error merging extracted annotations in location type: %v", err)
	}
}
