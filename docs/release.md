# release

- crd release
- release spec:
  a list of objects:
    - type
    - version
    - origin
    - date
- 1 new CR release will be created by the operator for 1 day
- no reconciliation needed
- each CR release has a configurable ttl (30 days by default)
- http, grpc and mcp endpoint to add a new release object
- each new release object is appended to the CR release of the day (based on the release object date field)
- appending new object must be thread safe
- add a grpc and mcp endpoint to get releases objects
- get endpoint must be paginated by day
- each page must be stored in memory to limit kubernetes api calls