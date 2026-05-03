// Copyright 2026 The SRE Portal Authors.
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

package remoteclient

// Shared string literals used across tests in this package, extracted to
// satisfy the goconst linter (min-occurrences: 3).
const (
	tSecretA    = "secret-a"
	tFQDNApp    = "app.example.com"
	tIP19216811 = "192.168.1.1"
	tEnvProd    = "production"
	tIP10001    = "10.0.0.1"
	tEnvDev     = "development"
	tPortalMain = "main"
	tTitleMain  = "Main Portal"
	tNsDefault  = "default"
)
