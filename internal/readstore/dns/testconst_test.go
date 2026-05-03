/*
Copyright 2026.

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

package dns_test

// Shared string constants for tests in this package — extracted to satisfy
// goconst and keep test data consistent.
const (
	tIP10001    = "10.0.0.1"
	tIP10002    = "10.0.0.2"
	tPortalMain = "main"
	tEnvStaging = "staging"
	tSrcIngress = "ingress"
	tSrcService = "service"
	tFQDNApp    = "app.example.com"
)
