---
sort: 11
---

# CRD Validation

## Description
 Operator supports validation admission webhook [docs](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
  
 It checks resources configuration and returns errors to caller before resource will be created at kubernetes api. 
 This should reduce errors and simplify debugging.
 
## Configuration

  Validation hooks at operator side must be enabled with flags:
```
--webhook.enable
# optional configuration for certDir and tls names.
--webhook.certDir=/tmp/k8s-webhook-server/serving-certs/
--webhook.keyName=tls.key
--webhook.certName=tls.crt
```

 You have to mount correct certificates at give directory. 
 It can be simplified with cert-manager and kustomize command: `kustomize build config/deployments/webhook/ `
  

## Requirements

 - Valid certificate with key must be provided to operator
 - Valid CABundle must be added to the `ValidatingWebhookConfiguration`


## Useful links
- [k8s admission webhooks](https://banzaicloud.com/blog/k8s-admission-webhooks/)
- [olm webhooks](https://docs.openshift.com/container-platform/4.5/operators/user/olm-webhooks.html)
