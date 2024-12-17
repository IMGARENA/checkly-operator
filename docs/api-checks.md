# api-checks

See the [official checkly docs](https://www.checklyhq.com/docs/api-checks/) on what API checks are.

> ***Warning***
> The default HTTP method is GET for API Checks. Other methods like POST, PUT, and DELETE are supported but must be explicitly specified in the configuration.

API Checks resources are namespace scoped, meaning they need to be unique inside a namespace and you need to add a `metadata.namespace` field to them.

We can also create API Checks from `ingress` resources, see [ingress](ingress.md) for more details.

## Configuration options

The name of the API check derives from the `metadata.name` of the created kubernetes resource.

### Labels

Any `metadata.labels` specified will be transformed into tags, for example `environment: dev` label will be transformed to `environment:dev` tag, these tags then propagate to Prometheus metrics (if you're using [the checkly prometheus endpoint](https://www.checklyhq.com/docs/integrations/prometheus/)).

> ***Note***
> Labels from `Group` resources are automatically propagated to the API checks which are added to the check group, you don't need to duplicate the labels.

### Spec

| Option         | Details     | Default |
|--------------|-----------|------------|
| `endpoint` | String; Endpoint to run the check against | none (*required) |
| `success` | String; The expected success code | none (*required) |
| `group` | String; Name of the group to which the check belongs; Kubernetes `Group` resource name | none (*required) |
| `frequency` | Integer; Frequency of minutes between each check, possible values: 1,2,5,10,15,30,60,120,180 | `5` |
| `muted` | Bool; Is the check muted or not | `false` |
| `maxresponsetime` | Integer; Number of milliseconds to wait for a response | `15000` |
| `method` | String; HTTP method to use (e.g., GET, POST, PUT, DELETE) | `GET` |
| `body` | String; Payload for the HTTP request, if applicable | `""` (empty) |
| `bodyType` | String; Format of the body (e.g., json, graphql, raw data) | `""` (none) |
| `assertions` | Array; A list of conditions to validate the check’s response | none (*optional) |

### Example

```yaml
apiVersion: k8s.checklyhq.com/v1alpha1
kind: ApiCheck
metadata:
  name: checkly-operator-test-check-1
  namespace: default
  labels:
    service: "foo"
spec:
  endpoint: "https://foo.bar/baz"
  success: "200"
  frequency: 10 # Default 5
  muted: true # Default "false"
  group: "checkly-operator-test-group"
  method: "POST"
  body: '{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}'
  bodyType: "json"
  assertions:
    - source: "STATUS_CODE"
      comparison: "EQUALS"
      target: "200"
    - source: "JSON_BODY"
      property: "$.status"
      comparison: "NOT_NULL"
---
apiVersion: k8s.checklyhq.com/v1alpha1
kind: ApiCheck
metadata:
  name: checkly-operator-test-check-2
  namespace: default
  labels:
    service: "bar"
spec:
  endpoint: "https://foo.bar/baaz"
  success: "200"
  group: "checkly-operator-test-group"
  method: "GET"
  assertions:
    - source: "STATUS_CODE"
      comparison: "EQUALS"
      target: "200"
