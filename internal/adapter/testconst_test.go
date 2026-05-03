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

package adapter

// Shared test constants. Defined here so goconst is satisfied without
// turning every test into a const-init exercise.
const (
	tValDefault         = "Default"
	tIP10001            = "10.0.0.1"
	tEnvProd            = "production"
	tEnvStaging         = "staging"
	tLabelGroup         = "sreportal.io/group"
	tFQDNAPI            = "api.example.com"
	tRecordCNAME        = "CNAME"
	tFQDNLB             = "lb.example.com"
	tIP10002            = "10.0.0.2"
	tIP10003            = "10.0.0.3"
	tValManual          = "manual"
	tEndpointSvcProdAPI = "service/production/api-svc"
	tSrcService         = "service"
	tSrcIngress         = "ingress"
	tSrcDNSEndpoint     = "dnsendpoint"
	tSrcIstioGateway    = "istio-gateway"
	tFQDNDNS            = "dns.example.com"
	tCompAPIGateway     = "API Gateway"
)
