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

package server

import (
	"github.com/spf13/pflag"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	// "k8s.io/kubernetes/pkg/client/typed/dynamic"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/genericapiserver"
	"k8s.io/kubernetes/pkg/storage/storagebackend"
)

// TPROptions contains the complete configuration for an API server that
// communicates with the core Kubernetes API server to use third party resources (TPRs)
// as a database. It is exported so that integration tests can use it
type TPROptions struct {
	defaultGlobalNamespace string
	clIface                clientset.Interface
	globalNamespace        string
}

// NewTPROptions creates a new, empty TPROptions struct
func NewTPROptions() *TPROptions {
	return &TPROptions{}
}

// NewStorageFactory returns a new StorageFactory from the config in opts
func (s *TPROptions) storageFactory() genericapiserver.StorageFactory {
	return genericapiserver.NewDefaultStorageFactory(
		storagebackend.Config{},
		"application/json",
		api.Codecs,
		genericapiserver.NewDefaultResourceEncodingConfig(),
		genericapiserver.NewResourceConfig(),
	)
}

func (s *TPROptions) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.globalNamespace, "global-namespace", s.defaultGlobalNamespace, ""+
		"The namespace in which to store all TPRs that represent global service-catalog resources.")
}
