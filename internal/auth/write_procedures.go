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

package auth

// WriteProcedures lists the Connect procedures that require portal-scoped authentication.
var WriteProcedures = map[string]bool{
	"/sreportal.v1.ReleaseService/AddRelease":       true,
	"/sreportal.v1.StatusService/CreateComponent":   true,
	"/sreportal.v1.StatusService/UpdateComponent":     true,
	"/sreportal.v1.StatusService/DeleteComponent":     true,
	"/sreportal.v1.StatusService/CreateMaintenance":   true,
	"/sreportal.v1.StatusService/UpdateMaintenance":   true,
	"/sreportal.v1.StatusService/DeleteMaintenance":   true,
	"/sreportal.v1.StatusService/CreateIncident":      true,
	"/sreportal.v1.StatusService/UpdateIncident":      true,
	"/sreportal.v1.StatusService/DeleteIncident":      true,
}
