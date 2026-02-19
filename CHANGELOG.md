# Changelog

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
