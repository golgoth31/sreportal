# Changelog

## [1.3.1](https://github.com/golgoth31/sreportal/compare/v1.3.0...v1.3.1) (2026-02-26)


### Bug Fixes

* **deps:** update module golang.org/x/net to v0.51.0 ([#69](https://github.com/golgoth31/sreportal/issues/69)) ([a69593d](https://github.com/golgoth31/sreportal/commit/a69593dc08211bd7c813dab05073532eac6ca0a5))
* helm chart ([7407dc6](https://github.com/golgoth31/sreportal/commit/7407dc636ef4deb51b67266701c6e1c018ac31a6))

## [1.3.0](https://github.com/golgoth31/sreportal/compare/v1.2.0...v1.3.0) (2026-02-25)


### Features

* show original resource for dns from external-dns source ([f9bdf62](https://github.com/golgoth31/sreportal/commit/f9bdf626579cac84c9a7947c6fcac26571c3cc38))

## [1.2.0](https://github.com/golgoth31/sreportal/compare/v1.1.0...v1.2.0) (2026-02-25)


### Features

* update web ui ([20e02fe](https://github.com/golgoth31/sreportal/commit/20e02fe88f5f7d2989d4e0fc8efa78671a21a5ea))

## [1.1.0](https://github.com/golgoth31/sreportal/compare/v1.0.4...v1.1.0) (2026-02-25)


### Features

* **web:** migrate UI to Angular Material and add MCP page ([fd8a829](https://github.com/golgoth31/sreportal/commit/fd8a82917f493eeec7898ce36bf97ba6713365ff))

## [1.0.4](https://github.com/golgoth31/sreportal/compare/v1.0.3...v1.0.4) (2026-02-25)


### Bug Fixes

* **dns:** prevent duplicate FQDN entries and group names in listings ([2f88ef5](https://github.com/golgoth31/sreportal/commit/2f88ef52890ff58e15d578508276daff57f5f7ac))

## [1.0.3](https://github.com/golgoth31/sreportal/compare/v1.0.2...v1.0.3) (2026-02-25)


### Bug Fixes

* **source_controller:** ensure DNS resources exist for portal FQDN aggregation ([20ec891](https://github.com/golgoth31/sreportal/commit/20ec8911bcfcc9871cc15c24356b512282dbbf39))

## [1.0.2](https://github.com/golgoth31/sreportal/compare/v1.0.1...v1.0.2) (2026-02-25)


### Bug Fixes

* **grpc:** lock DNS queries to aggregated DNS status for predictable results ([1db41ed](https://github.com/golgoth31/sreportal/commit/1db41edff310858a2f500398494d3a26f4d997d3))

## [1.0.1](https://github.com/golgoth31/sreportal/compare/v1.0.0...v1.0.1) (2026-02-25)


### Bug Fixes

* **mcp:** serve MCP via web port to simplify deployment ([8d1a58b](https://github.com/golgoth31/sreportal/commit/8d1a58b3278f766e158a9aa1c8dd9b37acd27367))

## [1.0.0](https://github.com/golgoth31/sreportal/compare/v0.9.2...v1.0.0) (2026-02-25)


### ⚠ BREAKING CHANGES

* consolidate MCP endpoint onto web server to simplify lifecycle

### Features

* consolidate MCP endpoint onto web server to simplify lifecycle ([8d52b24](https://github.com/golgoth31/sreportal/commit/8d52b24c7c1f9e05c42c0b3a8f8b30e69b6ed1e0))

## [0.9.2](https://github.com/golgoth31/sreportal/compare/v0.9.1...v0.9.2) (2026-02-24)


### Bug Fixes

* **deployment:** derive portal namespace from downward API at runtime ([0cd1248](https://github.com/golgoth31/sreportal/commit/0cd1248636af293c8e15a43e463c8bd7264fb48f))

## [0.9.1](https://github.com/golgoth31/sreportal/compare/v0.9.0...v0.9.1) (2026-02-23)


### Bug Fixes

* **webhook:** align admission endpoints with sreportal.io paths ([9cda97b](https://github.com/golgoth31/sreportal/commit/9cda97ba4b835739289d6a3db026f8efbfe5ac31))

## [0.9.0](https://github.com/golgoth31/sreportal/compare/v0.8.0...v0.9.0) (2026-02-23)


### Features

* **api:** migrate API group domain to sreportal.io ([72e2926](https://github.com/golgoth31/sreportal/commit/72e2926543364c550f49555fdcfb1d27fe028e3d))

## [0.8.0](https://github.com/golgoth31/sreportal/compare/v0.7.1...v0.8.0) (2026-02-23)


### Features

* **portal:** support custom TLS config for remote portal connections ([0fe78d5](https://github.com/golgoth31/sreportal/commit/0fe78d529e76cc36d0f1dac9dfda4832b3040259))
* **remote:** add insecureSkipVerify option to RemotePortalSpec ([311a311](https://github.com/golgoth31/sreportal/commit/311a311188be5deeb15e962196487909eb3dedb4))

## [0.7.1](https://github.com/golgoth31/sreportal/compare/v0.7.0...v0.7.1) (2026-02-23)


### Bug Fixes

* **mcp:** improve observability of client session lifecycle ([5cf33b7](https://github.com/golgoth31/sreportal/commit/5cf33b716e1da18be58d80122480ce66648532a7))
* **mcp:** use "http" transport type for Claude Code compatibility ([d97b2ba](https://github.com/golgoth31/sreportal/commit/d97b2ba2fe5436175ac72c2053c124b90aa605b0))

## [0.7.0](https://github.com/golgoth31/sreportal/compare/v0.6.7...v0.7.0) (2026-02-23)


### Features

* **mcp:** migrate from SSE to Streamable HTTP transport ([7f6040b](https://github.com/golgoth31/sreportal/commit/7f6040b8ddf9c108560392d72e5959e412b5f40c))


### Bug Fixes

* **mcp:** default dev and helm to streamable-http transport ([ea38607](https://github.com/golgoth31/sreportal/commit/ea386076509b8c615445d65c5d7b019561522f9d))

## [0.6.7](https://github.com/golgoth31/sreportal/compare/v0.6.6...v0.6.7) (2026-02-22)


### Bug Fixes

* **deps:** update angular monorepo to v21 ([#50](https://github.com/golgoth31/sreportal/issues/50)) ([8d9ccf1](https://github.com/golgoth31/sreportal/commit/8d9ccf1fd65c5dd50f1a3fdd1cba1227208e8e32))
* **deps:** update kubernetes packages to v0.35.1 ([#26](https://github.com/golgoth31/sreportal/issues/26)) ([6d915ab](https://github.com/golgoth31/sreportal/commit/6d915ab2f22965613d1c7f94e0fce1227ba2c342))
* **deps:** update module github.com/labstack/echo/v5 to v5.0.4 ([#27](https://github.com/golgoth31/sreportal/issues/27)) ([5f2c05e](https://github.com/golgoth31/sreportal/commit/5f2c05ef4a49073ef45cf7d2a48bca9eef3213d2))
* **deps:** update module github.com/onsi/ginkgo/v2 to v2.28.1 ([#29](https://github.com/golgoth31/sreportal/issues/29)) ([3d7f4e5](https://github.com/golgoth31/sreportal/commit/3d7f4e5234942189a339ac7d7bc222973c41500a))
* **deps:** update module github.com/onsi/gomega to v1.39.1 ([#30](https://github.com/golgoth31/sreportal/issues/30)) ([7ad72a9](https://github.com/golgoth31/sreportal/commit/7ad72a9883f43adc7456e7e1a5cdc481b6d533d3))
* **deps:** update module golang.org/x/net to v0.50.0 ([#38](https://github.com/golgoth31/sreportal/issues/38)) ([c0a30d0](https://github.com/golgoth31/sreportal/commit/c0a30d03e7c9eccaea161554ed3a305c3f6b2a6a))
* **deps:** update module google.golang.org/protobuf to v1.36.11 ([#28](https://github.com/golgoth31/sreportal/issues/28)) ([002f0e6](https://github.com/golgoth31/sreportal/commit/002f0e6ad939d9bc6e5785771e85815c65c7b30c))
* **deps:** update module istio.io/client-go to v1.29.0 ([#39](https://github.com/golgoth31/sreportal/issues/39)) ([fd1a457](https://github.com/golgoth31/sreportal/commit/fd1a4578fcb8d69719deb02567e189af23ccf97d))
* web ui fqdn in alphabetical order ([98c98f9](https://github.com/golgoth31/sreportal/commit/98c98f95299a459042dc2d00a4ec1f43f45e7345))

## [0.6.6](https://github.com/golgoth31/sreportal/compare/v0.6.5...v0.6.6) (2026-02-22)


### Bug Fixes

* CI ([60ae323](https://github.com/golgoth31/sreportal/commit/60ae32377153dbf0ea6c39bc227b3808b8bb3aad))

## [0.6.5](https://github.com/golgoth31/sreportal/compare/v0.6.4...v0.6.5) (2026-02-22)


### Bug Fixes

* doc ([0c3e880](https://github.com/golgoth31/sreportal/commit/0c3e8806addee0eb743f22cd0be29a6498e7c138))

## [0.6.4](https://github.com/golgoth31/sreportal/compare/v0.6.3...v0.6.4) (2026-02-21)


### Bug Fixes

* groups ([aeb8d8d](https://github.com/golgoth31/sreportal/commit/aeb8d8d89425005ec82f5b4972c878a0d94725bf))

## [0.6.3](https://github.com/golgoth31/sreportal/compare/v0.6.2...v0.6.3) (2026-02-19)


### Bug Fixes

* **helm:** align webhook resource name to avoid admission failures ([ca07164](https://github.com/golgoth31/sreportal/commit/ca07164a14c8d369bac41bbe8b7f6dbe9b13c8ea))

## [0.6.2](https://github.com/golgoth31/sreportal/compare/v0.6.1...v0.6.2) (2026-02-18)


### Bug Fixes

* **webhook:** align webhook resources with DNS CRD naming ([9c54681](https://github.com/golgoth31/sreportal/commit/9c546819da058f13c71554bf7601e191bd2e9b91))

## [0.6.1](https://github.com/golgoth31/sreportal/compare/v0.6.0...v0.6.1) (2026-02-18)


### Bug Fixes

* **rbac:** grant access needed to discover service endpoints ([8d0b109](https://github.com/golgoth31/sreportal/commit/8d0b1093c6ff53db0a3b5bc1efa1fa6913d2d196))

## [0.6.0](https://github.com/golgoth31/sreportal/compare/v0.5.0...v0.6.0) (2026-02-18)


### Features

* add operator ConfigMap to kustomize and fix CI pipelines ([2f19dca](https://github.com/golgoth31/sreportal/commit/2f19dca6c210eaae891745da1100578eb78039e0))

## [0.5.0](https://github.com/golgoth31/sreportal/compare/v0.4.1...v0.5.0) (2026-02-18)


### Features

* add sreportal.io/ignore annotation to exclude endpoints from discovery ([4e5dcb6](https://github.com/golgoth31/sreportal/commit/4e5dcb6eb46a556ed6d9e99d0150ca4c18f5cdbd))
* support multi-group FQDNs via comma-separated annotation ([#14](https://github.com/golgoth31/sreportal/issues/14)) ([b671a4d](https://github.com/golgoth31/sreportal/commit/b671a4df107a3fa6dc5742b91ba00b2ee30427bc))


### Bug Fixes

* **helm:** expose web and MCP endpoints for external access ([20678a3](https://github.com/golgoth31/sreportal/commit/20678a3f6675a37e0081cf8711a1f6d6efb49f10))

## [0.4.1](https://github.com/golgoth31/sreportal/compare/v0.4.0...v0.4.1) (2026-02-18)


### Bug Fixes

* **build:** simplify dev release pipeline after retiring custom setup ([e58f57a](https://github.com/golgoth31/sreportal/commit/e58f57a0d69ff043f0f16fb7ad601a201d4a9878))
* **helm:** centralize metrics monitoring and drop redundant RBAC ([4ba6d46](https://github.com/golgoth31/sreportal/commit/4ba6d4605b38b9dde3a5018ea15dc1b76539dc88))

## [0.4.0](https://github.com/golgoth31/sreportal/compare/v0.3.6...v0.4.0) (2026-02-18)


### Features

* **web:** support embedded UI with dev-mode filesystem override ([1e60eee](https://github.com/golgoth31/sreportal/commit/1e60eeeb4c88ebf5d2fc44a4fd2bec5bbe704aeb))


### Bug Fixes

* **release:** align helm chart version updates with release automation ([73d58bd](https://github.com/golgoth31/sreportal/commit/73d58bdbdcbd442c4baf511cea15dc8da1d1e47f))

## [0.3.6](https://github.com/golgoth31/sreportal/compare/v0.3.5...v0.3.6) (2026-02-18)


### Bug Fixes

* **release-please:** keep Helm appVersion and image tag in sync ([8a78051](https://github.com/golgoth31/sreportal/commit/8a780515e3abd3d3a45a67ba91a1fcca67b4c4b8))

## [0.3.5](https://github.com/golgoth31/sreportal/compare/v0.3.4...v0.3.5) (2026-02-17)


### Bug Fixes

* **ci:** publish releases to the new OCI registry location ([cc90607](https://github.com/golgoth31/sreportal/commit/cc90607ac04960b803b0c0ca6d51acd8f8bea202))

## [0.3.4](https://github.com/golgoth31/sreportal/compare/v0.3.3...v0.3.4) (2026-02-17)


### Bug Fixes

* **docker:** correct COPY path for web assets in goreleaser Dockerfile ([351826c](https://github.com/golgoth31/sreportal/commit/351826c295c1747047d421cb4d644ce849e9e067))

## [0.3.3](https://github.com/golgoth31/sreportal/compare/v0.3.2...v0.3.3) (2026-02-17)


### Bug Fixes

* **links:** prevent link source check from silently passing on bad data ([1330aba](https://github.com/golgoth31/sreportal/commit/1330abaddef6649ad0f80a5795f4cce878334278))

## [0.3.2](https://github.com/golgoth31/sreportal/compare/v0.3.1...v0.3.2) (2026-02-17)


### Bug Fixes

* CI ([adaa7e0](https://github.com/golgoth31/sreportal/commit/adaa7e0b049cb5bdb8bedef93de45ba32dacaaee))

## [0.3.1](https://github.com/golgoth31/sreportal/compare/v0.3.0...v0.3.1) (2026-02-17)


### Bug Fixes

* **ci:** restrict release workflows to semver tags ([04d0d2b](https://github.com/golgoth31/sreportal/commit/04d0d2b9275171e4260c7c1b0c4c38881b88aefd))

## [0.3.0](https://github.com/golgoth31/sreportal/compare/v0.2.0...v0.3.0) (2026-02-17)


### Features

* add Helm chart, Hugo config, api-docs tooling, and Makefile targets ([f4b82d1](https://github.com/golgoth31/sreportal/commit/f4b82d13e2b027d4cacf75a97545d41bdad4f868))
* add MCP server for AI assistant integration ([#1](https://github.com/golgoth31/sreportal/issues/1)) ([5f2aecd](https://github.com/golgoth31/sreportal/commit/5f2aecd2dfcee9b745948cefe827a2bd22cfa753))
* init ([5ce9504](https://github.com/golgoth31/sreportal/commit/5ce95045d53b350afa2e261ec6f3e08b72d88218))
* remote portal ([1957def](https://github.com/golgoth31/sreportal/commit/1957def52575faeac3ee1fa2092c8b39fef995f2))

## [0.2.0](https://github.com/golgoth31/sreportal/compare/sreportal-v0.1.0...sreportal-v0.2.0) (2026-02-17)


### Features

* add Helm chart, Hugo config, api-docs tooling, and Makefile targets ([f4b82d1](https://github.com/golgoth31/sreportal/commit/f4b82d13e2b027d4cacf75a97545d41bdad4f868))
* add MCP server for AI assistant integration ([#1](https://github.com/golgoth31/sreportal/issues/1)) ([5f2aecd](https://github.com/golgoth31/sreportal/commit/5f2aecd2dfcee9b745948cefe827a2bd22cfa753))
* init ([5ce9504](https://github.com/golgoth31/sreportal/commit/5ce95045d53b350afa2e261ec6f3e08b72d88218))
* remote portal ([1957def](https://github.com/golgoth31/sreportal/commit/1957def52575faeac3ee1fa2092c8b39fef995f2))
