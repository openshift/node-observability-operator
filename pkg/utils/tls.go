/*
Copyright 2022.

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

package utils

import (
	"crypto/x509"
	"fmt"
	"os"
)

// ReadCACert reads the given certificate file and returns x509 representation.
func ReadCACert(caCertFile string) (*x509.CertPool, error) {
	content, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	if len(content) <= 0 {
		return nil, fmt.Errorf("%s is empty", caCertFile)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(content) {
		return nil, fmt.Errorf("unable to add certificates into caCertPool: %v", err)

	}
	return caCertPool, nil
}
