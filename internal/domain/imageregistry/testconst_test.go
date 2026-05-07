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

package imageregistry

// Test-only string constants extracted to satisfy goconst lint rule.
const (
	tNsDefault       = "default"
	tPortalMain      = "main"
	tKindDeploy      = "Deployment"
	tContainerApp    = "app"
	tImgNginxDocker  = "docker.io/library/nginx:1.25.0"
	tContainerAPI    = "api"
	tImgRedisDocker  = "docker.io/library/redis:7.0.0"
	tImgRedisMirror  = "mirror.io/library/redis:7.0.0"
	tHostIndexDocker = "index.docker.io"
	tVersion100      = "1.0.0"
	tVersion123      = "1.2.3"
	tVersion124      = "1.2.4"
	tVersionV120     = "v1.2.0"
	tVersionRC       = "1.3.0-rc.1"
	tVersion140RC1   = "1.4.0-rc.1"
	tWorkloadWeb     = "web"
)
