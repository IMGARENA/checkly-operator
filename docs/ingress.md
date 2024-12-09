# ingress

Support for kubernetes native `ingress` resources. See [official docs](https://kubernetes.io/docs/concepts/services-networking/ingress/) for more details on what they are and what they do.

We pull out information with the use of `annotations` and use the built in spec. The information from the annotations is used to create `ApiCheck` resources, we make use of [ownerReferences](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/) to link ingress resources to ApiCheck resources.

> ***Warning***
> We currently only support API checks for ingress resources.

## Logic of discovery

We iterate over the ingress resource's specifications to work out what needs to be created. The operator creates one ApiCheck resource for each `host` + `path`, if in your ingress resource you have 2 hosts with 3 paths each, you'll end up with 6 ApiChecks created.

Specific annotations are optional, as we can't automatically discover the group you want the Checkly APIChecks to be deployd in.

## Configuration options

The name of the API Check derives from the `metadata.name` of the `ingress` resource and the corresponding API Check is created in the same namespace where the `ingress` object resides.

| Annotation         | Details     | Default |
|--------------------|-------------|---------|
| `k8s.checklyhq.com/enabled` | Bool; Should the operator read the annotations or not | `false` (*required) |
| `k8s.checklyhq.com/endpoint` | String; The host of the URL, for example `/` | Value of `spec.rules[0].Host`, defaults to `https://` |
| `k8s.checklyhq.com/group` | String; Name of the group to which the check belongs; Kubernetes `Group` resource name` | none (*required)|
| `k8s.checklyhq.com/muted` | String; Is the check muted or not | `true` |
| `k8s.checklyhq.com/path` | String; The URI to put after the `endpoint`, for example `/path` | ""|
| `k8s.checklyhq.com/success` | String; The expected success code | `200` |

### Example

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: checkly-operator-ingress
  annotations:
    k8s.checklyhq.com/enabled: "true"
    # k8s.checklyhq.com/path: "/baz" - Default read from spec.rules[0].http.paths[*].path
    # k8s.checklyhq.com/endpoint: "foo.baaz" - Default read from spec.rules[0].host
    # k8s.checklyhq.com/success: "200" - Default "200"
    k8s.checklyhq.com/group: "group-sample"
    # k8s.checklyhq.com/muted: "false" # If not set, default "true"
spec:
  rules:
    - host: "foo.bar"
      http:
        paths:
          - path: /foo
            pathType: ImplementationSpecific
            backend:
              service:
                name: test-service
                port:
                  number: 8080
          - path: /bar
            pathType: ImplementationSpecific
            backend:
              service:
                name: test-service
                port:
                  number: 8080
```
