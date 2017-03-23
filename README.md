# kube-external-dns

Thi is a Kubernetes service that watches (in all namespaces) for services created, updated, and deleted in the cluster and confiugres external DNS records in public DNS providers.

To use add a few annotations in your service resource
```
metadata:
  name: test-service
  annotations:
    kube.external.dns.io/provider: "cloudflare"
    kube.external.dns.io/root-domain: "koshk.in"
```
By default new records will be created in the form of `$service-name.$namespace.$root-domain`, from the example above that would be `test-service.default.koshk.in`  
It is also possible to override this behavior by specifying a custom `sub-domain` with the `kube.external.dns.io/sub-domain` annotation

Then deploy it, run something similar with your own provider details
```
kubectl run kube-external-dns --image=arduima/kube-external-dns --env="CLOUDFLARE_EMAIL=$EMAIL" --env="CLOUDFLARE_KEY=$API_KEY"
```
*Each provider will require its own credentials and will require different env variables*

If you want to use multiple providers pass the credentials and use the appropriate annotation values from below

### List of Providers
* CloudFlare  
Requires: `CLOUDFLARE_EMAIL` and `CLOUDFLARE_KEY`   
Annotation: `kube.external.dns.io/provider: "cloudflare"`  
* SimpleDNS
* Route53
* DigitalOcean
