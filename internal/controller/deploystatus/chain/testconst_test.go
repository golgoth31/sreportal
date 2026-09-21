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

package chain

// Test helper constants shared across the chain package tests.
const (
	testDefaultBranch = "main"
	testForgeHost     = "github.com"

	// testOperatorNs is the namespace the operator itself runs in (used when seeding fake CRs).
	testOperatorNs = "sreportal"

	// testCommitSHA is a reusable dummy commit SHA for field-mapping assertions.
	testCommitSHA = "abc123"

	// testRepoOwner is a placeholder GitHub org used in forge.RepoRef fixtures.
	testRepoOwner = "acme"

	// testErrKey / testUnresKey are service keys whose states are error / unresolved.
	testErrKey   = "err-key"
	testUnresKey = "unres-key"

	// testRepoNameA / testKindDeployment / testWorkloadApp are shared fixtures used across chain tests.
	testRepoNameA      = "app-a"
	testKindDeployment = "Deployment"
	testWorkloadApp    = "app"
)
