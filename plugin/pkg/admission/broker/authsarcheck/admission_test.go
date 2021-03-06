/*
Copyright 2017 The Kubernetes Authors.

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

package authsarcheck

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	authorizationapi "k8s.io/api/authorization/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
)

// newHandlerForTest returns a configured handler for testing.
func newHandlerForTest(kubeClient kubeclientset.Interface) (admission.Interface, kubeinformers.SharedInformerFactory, error) {
	kf := kubeinformers.NewSharedInformerFactory(kubeClient, 5*time.Minute)
	handler, err := NewSARCheck()
	if err != nil {
		return nil, kf, err
	}
	pluginInitializer := scadmission.NewPluginInitializer(nil, nil, kubeClient, kf)
	pluginInitializer.Initialize(handler)
	err = admission.ValidateInitialization(handler)
	return handler, kf, err
}

// newMockKubeClientForTest creates a mock kubernetes client that is configured
// to allow any SAR creations.
func newMockKubeClientForTest(userInfo *user.DefaultInfo) *kubefake.Clientset {
	mockClient := &kubefake.Clientset{}
	allowed := true
	if userInfo.GetName() == "system:serviceaccount:test-ns:forbidden" {
		allowed = false
	}
	mockClient.AddReactor("create", "subjectaccessreviews", func(action core.Action) (bool, runtime.Object, error) {
		mysar := &authorizationapi.SubjectAccessReview{
			Status: authorizationapi.SubjectAccessReviewStatus{
				Allowed: allowed,
				Reason:  "seemed friendly enough",
			},
		}
		return true, mysar, nil
	})
	return mockClient
}

// TestAdmissionBroker tests Admit to ensure that the result from the SAR check
// is properly checked.
func TestAdmissionBroker(t *testing.T) {
	// Anonymous struct fields:
	// name: short description of the testing
	// broker: a fake broker object
	// userInfo: info about the user submitting the request
	// allowed: flag for whether or not the broker should be admitted
	clusterCases := []struct {
		name     string
		broker   *servicecatalog.ClusterServiceBroker
		userInfo *user.DefaultInfo
		allowed  bool
	}{
		{
			name: "broker with no auth",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "broker with basic auth, user authenticated",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Basic: &servicecatalog.ClusterBasicAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "broker with bearer token, user authenticated",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Bearer: &servicecatalog.ClusterBearerTokenAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "broker with bearer token, unauthenticated user",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Bearer: &servicecatalog.ClusterBearerTokenAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:forbidden",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: false,
		},
		{
			name: "broker with empty authInfo",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:forbidden",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "broker with authInfo, empty strings for Namespace/Name",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Bearer: &servicecatalog.ClusterBearerTokenAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "",
								Name:      "",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
	}

	for _, tc := range clusterCases {
		mockKubeClient := newMockKubeClientForTest(tc.userInfo)
		handler, kubeInformerFactory, err := newHandlerForTest(mockKubeClient)
		if err != nil {
			t.Errorf("unexpected error initializing handler: %v", err)
		}
		kubeInformerFactory.Start(wait.NeverStop)

		err = handler.(admission.MutationInterface).Admit(admission.NewAttributesRecord(tc.broker, nil, servicecatalog.Kind("ClusterServiceBroker").WithVersion("version"), tc.broker.Namespace, tc.broker.Name, servicecatalog.Resource("clusterservicebrokers").WithVersion("version"), "", admission.Create, false, tc.userInfo), nil)
		if err != nil && tc.allowed || err == nil && !tc.allowed {
			t.Errorf("Create test '%s' reports: Unexpected error returned from admission handler: %v", tc.name, err)
		}
	}

	// Anonymous struct fields:
	// name: short description of the testing
	// broker: a fake broker object
	// userInfo: info about the user submitting the request
	// allowed: flag for whether or not the broker should be admitted
	namespacedCases := []struct {
		name     string
		broker   *servicecatalog.ServiceBroker
		userInfo *user.DefaultInfo
		allowed  bool
	}{
		{
			name: "namespace broker with no auth",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
					Name:      "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "namespace broker with basic auth, user authenticated",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
					Name:      "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Basic: &servicecatalog.BasicAuthConfig{
							SecretRef: &servicecatalog.LocalObjectReference{
								Name: "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "namespace broker with bearer token, user authenticated",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
					Name:      "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &servicecatalog.LocalObjectReference{
								Name: "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "namespace broker with bearer token, unauthenticated user",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
					Name:      "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &servicecatalog.LocalObjectReference{
								Name: "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:forbidden",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: false,
		},
		{
			name: "namespace broker with empty authInfo",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:forbidden",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
		{
			name: "namespace broker with authInfo, empty strings for Namespace/Name",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-ns",
					Name:      "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &servicecatalog.LocalObjectReference{
								Name: "",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Manual",
					},
				},
			},
			userInfo: &user.DefaultInfo{
				Name:   "system:serviceaccount:test-ns:catalog",
				Groups: []string{"system:serviceaccount", "system:serviceaccounts:test-ns"},
			},
			allowed: true,
		},
	}

	for _, tc := range namespacedCases {
		mockKubeClient := newMockKubeClientForTest(tc.userInfo)
		handler, kubeInformerFactory, err := newHandlerForTest(mockKubeClient)
		if err != nil {
			t.Errorf("unexpected error initializing handler: %v", err)
		}
		kubeInformerFactory.Start(wait.NeverStop)

		err = handler.(admission.MutationInterface).Admit(admission.NewAttributesRecord(tc.broker, nil, servicecatalog.Kind("ServiceBroker").WithVersion("version"), tc.broker.Namespace, tc.broker.Name, servicecatalog.Resource("servicebrokers").WithVersion("version"), "", admission.Create, false, tc.userInfo), nil)
		if err != nil && tc.allowed || err == nil && !tc.allowed {
			t.Errorf("Create test '%s' reports: Unexpected error returned from admission handler: %v", tc.name, err)
		}
	}
}
