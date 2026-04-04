# Changelog

## [1.26.2](https://github.com/golgoth31/sreportal/compare/v1.26.1...v1.26.2) (2026-04-04)


### Bug Fixes

* **deps:** pin dependencies ([#189](https://github.com/golgoth31/sreportal/issues/189)) ([7c1db0a](https://github.com/golgoth31/sreportal/commit/7c1db0abc4a336deff30507ecb249412cd47812b))
* **deps:** pin dependencies ([#192](https://github.com/golgoth31/sreportal/issues/192)) ([8a2143c](https://github.com/golgoth31/sreportal/commit/8a2143c1f59b341dd2d9bc64922865087b9cba01))
* **deps:** pin dependencies ([#193](https://github.com/golgoth31/sreportal/issues/193)) ([26a6c51](https://github.com/golgoth31/sreportal/commit/26a6c5111beadefd0f2975998abc67729bbeab8b))
* **deps:** update dependency @base-ui/react to v1.3.0 ([#199](https://github.com/golgoth31/sreportal/issues/199)) ([1887c62](https://github.com/golgoth31/sreportal/commit/1887c62bd05814921486f761066fb394ffd233d1))
* **deps:** update dependency @tanstack/react-query to v5.96.2 ([#133](https://github.com/golgoth31/sreportal/issues/133)) ([1bf5e3f](https://github.com/golgoth31/sreportal/commit/1bf5e3ff9c8f4bc5f3f41a5342a391ff38f08280))
* **deps:** update dependency lucide-react to v1 ([#183](https://github.com/golgoth31/sreportal/issues/183)) ([709589d](https://github.com/golgoth31/sreportal/commit/709589d842f3fc0f616b99d819fdb5602f33fe32))
* **deps:** update dependency react-router to ^7.14.0 ([#174](https://github.com/golgoth31/sreportal/issues/174)) ([f0fc1d7](https://github.com/golgoth31/sreportal/commit/f0fc1d776b4db81c62ab42279b32da0acc4b123d))
* **deps:** update dependency recharts to v3 ([#184](https://github.com/golgoth31/sreportal/issues/184)) ([53ad490](https://github.com/golgoth31/sreportal/commit/53ad490417b7fa56ea4c4276c18e89d042152273))
* **deps:** update module github.com/labstack/echo/v5 to v5.1.0 ([#178](https://github.com/golgoth31/sreportal/issues/178)) ([e4b1e24](https://github.com/golgoth31/sreportal/commit/e4b1e24bb3cab9a6df8d3a2a8b2c46644919985a))
* **deps:** update module github.com/mark3labs/mcp-go to v0.46.0 ([#179](https://github.com/golgoth31/sreportal/issues/179)) ([f79d83c](https://github.com/golgoth31/sreportal/commit/f79d83cd8a583f61aed8daa68fa4d7b68f63506e))
* **deps:** update tailwind to v4.2.2 ([#194](https://github.com/golgoth31/sreportal/issues/194)) ([558c01d](https://github.com/golgoth31/sreportal/commit/558c01d0065816f94e59e84a727fd7a9add88c06))
* remove invalid packageRules priority from Renovate config ([856d94c](https://github.com/golgoth31/sreportal/commit/856d94c60ba83c71a542a70e5abbe1436756ca34))
* repair Renovate config (matchPackageNames, dedupe extends) ([90f5636](https://github.com/golgoth31/sreportal/commit/90f5636a89709400010d70abdb87f5d05eb30d95))
* **test:** wait for cache sync in all controller test suites ([ed49c36](https://github.com/golgoth31/sreportal/commit/ed49c368b15d8e162ce0f733a9c96131b2e8e63e))
* **web:** compatibilité TS 6 (baseUrl, types Recharts chart) ([232d6d7](https://github.com/golgoth31/sreportal/commit/232d6d73c52ea4378ceffa823d54d2b6c909d571))
* **web:** Recharts charts, tests sans bruit console ([afbc48c](https://github.com/golgoth31/sreportal/commit/afbc48c697182bdf1b9f36521510c847c2aec849))

## [1.26.1](https://github.com/golgoth31/sreportal/compare/v1.26.0...v1.26.1) (2026-04-03)


### Bug Fixes

* **netpol:** replace Impact Analysis with Flow Explorer ([#167](https://github.com/golgoth31/sreportal/issues/167)) ([c53ff74](https://github.com/golgoth31/sreportal/commit/c53ff745c13ec4020a45a2868059f8af49b86c05))

## [1.26.0](https://github.com/golgoth31/sreportal/compare/v1.25.1...v1.26.0) (2026-04-01)


### Features

* **component:** add 30-day daily worst-status history with backfill ([#168](https://github.com/golgoth31/sreportal/issues/168)) ([6134b87](https://github.com/golgoth31/sreportal/commit/6134b870e57d85d6a953f586853cac64e0fb5133))

## [1.25.1](https://github.com/golgoth31/sreportal/compare/v1.25.0...v1.25.1) (2026-03-31)


### Bug Fixes

* **emoji:** resolve Slack emoji shortcodes in alerts and releases ([#165](https://github.com/golgoth31/sreportal/issues/165)) ([0f4c049](https://github.com/golgoth31/sreportal/commit/0f4c049eea8355046c7ed92f4925906a2bf495dd))

## [1.25.0](https://github.com/golgoth31/sreportal/compare/v1.24.0...v1.25.0) (2026-03-31)


### Features

* **grpc:** gate services on portal feature flags and sync remote features ([1dc46fe](https://github.com/golgoth31/sreportal/commit/1dc46fe5035311d8ff5deb0fb4d7898c235680c1))


### Bug Fixes

* **netpol:** purge flow graph data when networkPolicy feature is disabled ([71bd2c4](https://github.com/golgoth31/sreportal/commit/71bd2c46f6cc1fa6c3713ac8af3651293a46978e))
* **netpol:** replace unicode escape sequences with actual characters in UI views ([c48811e](https://github.com/golgoth31/sreportal/commit/c48811e38bf4bf4a21a25499cdc4e5d17c1b7c3e))

## [1.24.0](https://github.com/golgoth31/sreportal/compare/v1.23.2...v1.24.0) (2026-03-29)


### Features

* **component:** add annotation-driven component auto-creation from sources and DNS CRs ([8e7a98f](https://github.com/golgoth31/sreportal/commit/8e7a98f7d79536ad4c29e453ed5e2da7b82939c4))
* **portal:** add per-portal feature toggles ([#161](https://github.com/golgoth31/sreportal/issues/161)) ([90987ab](https://github.com/golgoth31/sreportal/commit/90987ab7b9072a1c2f42054290348de170fa7dbd))
* **statuspage:** add status page with component, maintenance and incident CRDs ([#162](https://github.com/golgoth31/sreportal/issues/162)) ([5335690](https://github.com/golgoth31/sreportal/commit/533569011cd511a2d59511313eb6d221a6b2b411))


### Bug Fixes

* **component:** read incidents from k8s cache to eliminate race condition ([e7feeef](https://github.com/golgoth31/sreportal/commit/e7feeef8d91cda9372d508013cd14abd4e7295e2))

## [1.23.2](https://github.com/golgoth31/sreportal/compare/v1.23.1...v1.23.2) (2026-03-27)


### Bug Fixes

* **web:** move remote portal button to nav bar and add beta badge on impact tab ([013b243](https://github.com/golgoth31/sreportal/commit/013b2438c57441f77f9b2fdf7956ad25ff5c9837))

## [1.23.1](https://github.com/golgoth31/sreportal/compare/v1.23.0...v1.23.1) (2026-03-26)


### Bug Fixes

* **dns:** prevent DNS controller from overwriting remote DNS CR status ([b03c657](https://github.com/golgoth31/sreportal/commit/b03c657973240fc72dbf59c485d99ad803910c20))

## [1.23.0](https://github.com/golgoth31/sreportal/compare/v1.22.0...v1.23.0) (2026-03-26)


### Features

* add auth ([#157](https://github.com/golgoth31/sreportal/issues/157)) ([085b6c6](https://github.com/golgoth31/sreportal/commit/085b6c66c3b784f468f1f1dfc938977b1d67902f))
* refactor source controller, add helmify post-processor and hardcode auth env var ([be7c5c2](https://github.com/golgoth31/sreportal/commit/be7c5c2754dbbad404d74e15dceb16aa6ffa1a40))

## [1.22.0](https://github.com/golgoth31/sreportal/compare/v1.21.2...v1.22.0) (2026-03-25)


### Features

* improve memory caching ([#155](https://github.com/golgoth31/sreportal/issues/155)) ([59911cf](https://github.com/golgoth31/sreportal/commit/59911cfd92ff13b1331c43df9d8273d64add6ffb))

## [1.21.2](https://github.com/golgoth31/sreportal/compare/v1.21.1...v1.21.2) (2026-03-25)


### Bug Fixes

* **netpol:** add portal filtering and fix data mixing across portals ([6632c09](https://github.com/golgoth31/sreportal/commit/6632c0919ece2ac374c8a60cd327f7d056e25076))

## [1.21.1](https://github.com/golgoth31/sreportal/compare/v1.21.0...v1.21.1) (2026-03-25)


### Bug Fixes

* **netpol:** align remote ownership model with local chain ([4018f44](https://github.com/golgoth31/sreportal/commit/4018f44e20fc632d2a32fe9f1c020751ba6635bf))

## [1.21.0](https://github.com/golgoth31/sreportal/compare/v1.20.1...v1.21.0) (2026-03-24)


### Features

* **netpol:** add remote portal support for network policies ([6171127](https://github.com/golgoth31/sreportal/commit/6171127ed2407fd0bcbc6b8c525e5ef20f927128))

## [1.20.1](https://github.com/golgoth31/sreportal/compare/v1.20.0...v1.20.1) (2026-03-24)


### Bug Fixes

* **netpol:** add missing RBAC verbs for networkpolicies watch ([da3d739](https://github.com/golgoth31/sreportal/commit/da3d739a648346c80622bc455b7ea451d8535f28))

## [1.20.0](https://github.com/golgoth31/sreportal/compare/v1.19.2...v1.20.0) (2026-03-24)


### Features

* **netpol:** add network policy explorer ([#149](https://github.com/golgoth31/sreportal/issues/149)) ([d9c06b5](https://github.com/golgoth31/sreportal/commit/d9c06b5869f7bb5329439071504789ac252504a3))


### Bug Fixes

* **web:** constrain sidebar height to viewport instead of page content ([95a3076](https://github.com/golgoth31/sreportal/commit/95a3076b9c2aa6557007dfd62f8cef7c4cd7b376))

## [1.19.2](https://github.com/golgoth31/sreportal/compare/v1.19.1...v1.19.2) (2026-03-23)


### Bug Fixes

* **release:** show fallback link text when version is missing ([d854ec6](https://github.com/golgoth31/sreportal/commit/d854ec66ce566b1030f14a0b3ca8da0b369a0f89))

## [1.19.1](https://github.com/golgoth31/sreportal/compare/v1.19.0...v1.19.1) (2026-03-23)


### Bug Fixes

* **release:** replace link column with clickable version text ([e31cf9d](https://github.com/golgoth31/sreportal/commit/e31cf9d0d0a00362142b6eda4d54225bcedeb804))

## [1.19.0](https://github.com/golgoth31/sreportal/compare/v1.18.0...v1.19.0) (2026-03-23)


### Features

* **release:** make version field optional in ReleaseEntry ([117a560](https://github.com/golgoth31/sreportal/commit/117a5600341b068497eb183175a10d14cea9304b))

## [1.18.0](https://github.com/golgoth31/sreportal/compare/v1.17.0...v1.18.0) (2026-03-23)


### Features

* **release:** flatten AddRelease request payload ([0880bd5](https://github.com/golgoth31/sreportal/commit/0880bd50058bdb2d0435bf2a5bbff1e0526dd3ff))

## [1.17.0](https://github.com/golgoth31/sreportal/compare/v1.16.1...v1.17.0) (2026-03-22)


### Features

* **source:** add Crossplane Scaleway DNS Record source ([#98](https://github.com/golgoth31/sreportal/issues/98)) ([c58777a](https://github.com/golgoth31/sreportal/commit/c58777a1c11027d8c9403c30d78e0865edfdceb8))

## [1.16.1](https://github.com/golgoth31/sreportal/compare/v1.16.0...v1.16.1) (2026-03-22)


### Bug Fixes

* **release:** replace static type colors with server-driven config ([8275335](https://github.com/golgoth31/sreportal/commit/827533580c1783aba04405ae3f5acc19f0ff7c97))

## [1.16.0](https://github.com/golgoth31/sreportal/compare/v1.15.2...v1.16.0) (2026-03-22)


### Features

* **observability:** improve http request logging and switch web ui to grpc-web ([cc70655](https://github.com/golgoth31/sreportal/commit/cc7065550290b3c9ca71b1b17c53fa8b3e968df6))
* **release:** log info when a new release CR is created via gRPC ([4e16163](https://github.com/golgoth31/sreportal/commit/4e1616339ea9f8f9b02e3bd49912baae13d3f313))


### Bug Fixes

* **web:** add manual refresh button and centralize query defaults ([9c5c947](https://github.com/golgoth31/sreportal/commit/9c5c9474b6efd5cde3cc5db6d3361076efb0d2e7))

## [1.15.2](https://github.com/golgoth31/sreportal/compare/v1.15.1...v1.15.2) (2026-03-22)


### Bug Fixes

* **rbac:** align release permissions with controller reconcile needs ([8f33d60](https://github.com/golgoth31/sreportal/commit/8f33d60287ef712b2eb6be2f05993fdae70752e8))

## [1.15.1](https://github.com/golgoth31/sreportal/compare/v1.15.0...v1.15.1) (2026-03-22)


### Bug Fixes

* **webhook:** enforce release CR validation via admission webhooks ([ec6e9ee](https://github.com/golgoth31/sreportal/commit/ec6e9ee67f35771b92775f30704e87986d833873))

## [1.15.0](https://github.com/golgoth31/sreportal/compare/v1.14.1...v1.15.0) (2026-03-22)


### Features

* **metrics:** add metrics dashboard with gRPC API, MCP server, and web UI ([581e803](https://github.com/golgoth31/sreportal/commit/581e803f7d8edd20d52c2ce727bebebd627d2de1))
* **portal:** surface remote sync errors in MCP and web UI ([24acbfb](https://github.com/golgoth31/sreportal/commit/24acbfbcc7b462636b485a247aaef16d9237e88c))
* **release:** add date picker constraints, table UI, and type enforcement ([#139](https://github.com/golgoth31/sreportal/issues/139)) ([b2c23c6](https://github.com/golgoth31/sreportal/commit/b2c23c6c6ef66b117c6d860c5089e579135bed19))
* **release:** add release tracking CRD, API, MCP server, and web UI ([dec1857](https://github.com/golgoth31/sreportal/commit/dec18578a3739be8a1b06bd8666e65858a6bbb80))

## [1.14.1](https://github.com/golgoth31/sreportal/compare/v1.14.0...v1.14.1) (2026-03-20)


### Bug Fixes

* **metrics:** correct DNS FQDN and portal gauge calculations ([63a1e64](https://github.com/golgoth31/sreportal/commit/63a1e6413dc25fa9c44691bdc148886ba3b6de33))

## [1.14.0](https://github.com/golgoth31/sreportal/compare/v1.13.1...v1.14.0) (2026-03-20)


### Features

* add custom prometheus metrics and grafana dashboard ([2351c2c](https://github.com/golgoth31/sreportal/commit/2351c2c0766b33f68e1992a930e4c0efe48f8605))

## [1.13.1](https://github.com/golgoth31/sreportal/compare/v1.13.0...v1.13.1) (2026-03-19)


### Bug Fixes

* controller memory optimisations ([#125](https://github.com/golgoth31/sreportal/issues/125)) ([1b2f325](https://github.com/golgoth31/sreportal/commit/1b2f3257d530855300d265fab586dea74ef9a1ac))

## [1.13.0](https://github.com/golgoth31/sreportal/compare/v1.12.1...v1.13.0) (2026-03-16)


### Features

* source factory registry ([#122](https://github.com/golgoth31/sreportal/issues/122)) ([46fd16e](https://github.com/golgoth31/sreportal/commit/46fd16e601e048d0556a909a70d5071c347b1eeb))
* support gateway api resources ([#114](https://github.com/golgoth31/sreportal/issues/114)) ([64e7d2f](https://github.com/golgoth31/sreportal/commit/64e7d2fdfba3c1f11a314562262a6de50048997b))

## [1.12.1](https://github.com/golgoth31/sreportal/compare/v1.12.0...v1.12.1) (2026-03-12)


### Bug Fixes

* **alertmanager:** propagate silences and receivers from remote portals ([aa763a3](https://github.com/golgoth31/sreportal/commit/aa763a31932cc053a1e518e0c085782f204f4784))

## [1.12.0](https://github.com/golgoth31/sreportal/compare/v1.11.5...v1.12.0) (2026-03-11)


### Features

* **alertmanager:** show silencd and receivers ([f7f0144](https://github.com/golgoth31/sreportal/commit/f7f014443af57d1d7eb6a7346956661fdf135617))

## [1.11.5](https://github.com/golgoth31/sreportal/compare/v1.11.4...v1.11.5) (2026-03-09)


### Bug Fixes

* missing file ([0139f38](https://github.com/golgoth31/sreportal/commit/0139f38e16d221b151a116075cc7339217aca5cb))
* **web:** fix bugs ([021a53c](https://github.com/golgoth31/sreportal/commit/021a53c7bb9ad907c6fb09f810992871d8529f41))

## [1.11.4](https://github.com/golgoth31/sreportal/compare/v1.11.3...v1.11.4) (2026-03-09)


### Bug Fixes

* **alertmanager:** improve alert browsing via grouping by name ([ba3a621](https://github.com/golgoth31/sreportal/commit/ba3a621ca421ca54b3305879bde1b1e846e7963f))

## [1.11.3](https://github.com/golgoth31/sreportal/compare/v1.11.2...v1.11.3) (2026-03-08)


### Bug Fixes

* **remote-alertmanager:** select remote Alertmanager via label ([57ef45a](https://github.com/golgoth31/sreportal/commit/57ef45a3cafa6c0856710279e18ca9892da9dd41))

## [1.11.2](https://github.com/golgoth31/sreportal/compare/v1.11.1...v1.11.2) (2026-03-08)


### Bug Fixes

* **alertmanager:** surface remote Alertmanager URL for remote portals ([fc1b137](https://github.com/golgoth31/sreportal/commit/fc1b1373f0979e5d838ddca0beaf2230f0df6b21))

## [1.11.1](https://github.com/golgoth31/sreportal/compare/v1.11.0...v1.11.1) (2026-03-08)


### Bug Fixes

* **alertmanager:** align remote alert fetch with Portal TLS config ([c879702](https://github.com/golgoth31/sreportal/commit/c8797023703a04e77857975c42b38937cd077a3a))
* fix golangci-lint warnings ([fe97651](https://github.com/golgoth31/sreportal/commit/fe97651e61d6fd5644241532a38b1f897dbae262))
* **version:** expose build metadata via VersionService endpoint ([0a25b11](https://github.com/golgoth31/sreportal/commit/0a25b116de6ac4052dc52d4e30d41f4d5c9482c3))

## [1.11.0](https://github.com/golgoth31/sreportal/compare/v1.10.0...v1.11.0) (2026-03-08)


### Features

* **alertmanager:** support fetching alerts from remote portals ([3bfcd8b](https://github.com/golgoth31/sreportal/commit/3bfcd8b0c89b3dce8f752aab442f5847f88e69f7))


### Bug Fixes

* **alertmanager:** tighten alert state typing for safer filtering ([18dd776](https://github.com/golgoth31/sreportal/commit/18dd7762acfb6dcf8beb583d1be876d49fa6e917))

## [1.10.0](https://github.com/golgoth31/sreportal/compare/v1.9.4...v1.10.0) (2026-03-08)


### Features

* monitoring ([#106](https://github.com/golgoth31/sreportal/issues/106)) ([12ab827](https://github.com/golgoth31/sreportal/commit/12ab827bec9de772c95009d8369f93095a7ee78c))


### Bug Fixes

* **deps:** update dependency shadcn to v4 ([#105](https://github.com/golgoth31/sreportal/issues/105)) ([dc94da6](https://github.com/golgoth31/sreportal/commit/dc94da60f060ea189ad5983ead6029d83ebc86fa))
* **deps:** update module github.com/mark3labs/mcp-go to v0.45.0 ([#104](https://github.com/golgoth31/sreportal/issues/104)) ([885a0f1](https://github.com/golgoth31/sreportal/commit/885a0f1d7f25e9b8827d29217b6d42d9eb3419d1))
* **deps:** update module sigs.k8s.io/controller-runtime to v0.23.3 ([#99](https://github.com/golgoth31/sreportal/issues/99)) ([b2328c4](https://github.com/golgoth31/sreportal/commit/b2328c40ee57018d0938b869f3493b987549328c))

## [1.9.4](https://github.com/golgoth31/sreportal/compare/v1.9.3...v1.9.4) (2026-03-04)


### Bug Fixes

* **operator:** use status patch to avoid optimistic lock conflicts ([cc93fe8](https://github.com/golgoth31/sreportal/commit/cc93fe816485ff62ef4e97aac57e633de8648c7a))

## [1.9.3](https://github.com/golgoth31/sreportal/compare/v1.9.2...v1.9.3) (2026-03-04)


### Bug Fixes

* **operator:** disable debug logging in production mode ([862afd3](https://github.com/golgoth31/sreportal/commit/862afd33047abbafdf6229a8aae5123f74a85396))

## [1.9.2](https://github.com/golgoth31/sreportal/compare/v1.9.1...v1.9.2) (2026-03-02)


### Bug Fixes

* favicon ([ec61eff](https://github.com/golgoth31/sreportal/commit/ec61eff026cb817b2a8fc539e9fc8f7e5683b8bc))

## [1.9.1](https://github.com/golgoth31/sreportal/compare/v1.9.0...v1.9.1) (2026-03-01)


### Bug Fixes

* web ui select ([03da29e](https://github.com/golgoth31/sreportal/commit/03da29e45ee58045b9ccb09dd9e3a57efed7319e))

## [1.9.0](https://github.com/golgoth31/sreportal/compare/v1.8.0...v1.9.0) (2026-03-01)


### Features

* **grpc:** add shared FQDN cache, stream filters, and pagination ([d70063f](https://github.com/golgoth31/sreportal/commit/d70063fa48c00083b2905f03d0bb8d975743f571))
* **mcp:** expose sync_status in search_fqdns and get_fqdn_details tools ([3a70186](https://github.com/golgoth31/sreportal/commit/3a701864814093673c8b046d7b4f9feb703ca04f))
* **ui:** restructure portal nav with select lists ([0607cbd](https://github.com/golgoth31/sreportal/commit/0607cbdc5652598413831006a25b4a69c497b87a))


### Bug Fixes

* **dns:** normalise trailing dot in CNAME sync check ([84dba7b](https://github.com/golgoth31/sreportal/commit/84dba7b66443c7feade3e3a7e23ff4cbf958bee1))

## [1.8.0](https://github.com/golgoth31/sreportal/compare/v1.7.1...v1.8.0) (2026-02-28)


### Features

* add config to disable dns check ([db57dff](https://github.com/golgoth31/sreportal/commit/db57dffa5bcaad228f2242cbdc65ea2456ae863b))


### Bug Fixes

* fqdn dedup ([09dde68](https://github.com/golgoth31/sreportal/commit/09dde68771cbb60f3a4dad78f51369a34a55b870))

## [1.7.1](https://github.com/golgoth31/sreportal/compare/v1.7.0...v1.7.1) (2026-02-28)


### Bug Fixes

* remote portal view ([c77fbcb](https://github.com/golgoth31/sreportal/commit/c77fbcb7997c15e60aed6a7807c31440568e87c0))
* resolve six code-review bugs across grpc, source, and reconciler packages ([ea8f67c](https://github.com/golgoth31/sreportal/commit/ea8f67c9fde9d8e7cb57520d78bb902f4a50ae1e))

## [1.7.0](https://github.com/golgoth31/sreportal/compare/v1.6.0...v1.7.0) (2026-02-27)


### Features

* add priority between sources ([#83](https://github.com/golgoth31/sreportal/issues/83)) ([494f0f3](https://github.com/golgoth31/sreportal/commit/494f0f3c938337a3ba2c601e1da7762062092227))

## [1.6.0](https://github.com/golgoth31/sreportal/compare/v1.5.1...v1.6.0) (2026-02-27)


### Features

* react UI rewrite ([#73](https://github.com/golgoth31/sreportal/issues/73)) ([a0d3c37](https://github.com/golgoth31/sreportal/commit/a0d3c374d5bc69dcf5d65f4d8180177d163eb9db))


### Bug Fixes

* **deps:** update module github.com/mark3labs/mcp-go to v0.44.1 ([#76](https://github.com/golgoth31/sreportal/issues/76)) ([2dd2f21](https://github.com/golgoth31/sreportal/commit/2dd2f2133186a1372b1d3086f98928681ccb7067))

## [1.5.1](https://github.com/golgoth31/sreportal/compare/v1.5.0...v1.5.1) (2026-02-27)


### Bug Fixes

* **helm:** add syncStatus field to DNS CRD and fix values indentation ([13a4259](https://github.com/golgoth31/sreportal/commit/13a425926aa11d6a62056053f2402563bb2af2b2))

## [1.5.0](https://github.com/golgoth31/sreportal/compare/v1.4.1...v1.5.0) (2026-02-27)


### Features

* add DNS resolution verification with sync status indicator ([615db8d](https://github.com/golgoth31/sreportal/commit/615db8d67ac59ec7e069d916be60828b5bac8a56))


### Bug Fixes

* skip DNS resolution for remote portal FQDNs ([1efc2b1](https://github.com/golgoth31/sreportal/commit/1efc2b1a5bd6b0b25004f3bd8a065b5890a6104a))

## [1.4.1](https://github.com/golgoth31/sreportal/compare/v1.4.0...v1.4.1) (2026-02-27)


### Bug Fixes

* update ui theme ([8aa4604](https://github.com/golgoth31/sreportal/commit/8aa4604c1d8b6c6ab0ec275fc62324767edd78cf))

## [1.4.0](https://github.com/golgoth31/sreportal/compare/v1.3.1...v1.4.0) (2026-02-26)


### Features

* **web:** add overlay on button, rename mcp page as help ([b6ecb89](https://github.com/golgoth31/sreportal/commit/b6ecb89c34726fa4f5182399bd98bac7cf2d32fb))

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
