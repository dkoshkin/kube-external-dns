# kube-external-dns

Thi is a Kubernetes service that watches (in all namespaces) for services created, updated, and deleted in the cluster and confiugres external DNS records in public DNS providers.

The service relies on the publicly reachable IPs created in supported clouds with the `LoadBalancer` Kubernetes service [type](https://kubernetes.io/docs/user-guide/services/#type-loadbalancer).

To use add a few annotations in your service resource
```
metadata:
  name: test-service
  annotations:
    external.dns.koshk.in/provider: "cloudflare"
    external.dns.koshk.in/root-domain: "koshk.in"
```
By default new records will be created in the form of `$service-name.$namespace.$root-domain`, from the example above that would be `test-service.default.koshk.in`  
It is also possible to override this behavior by specifying a custom `sub-domain` with the `external.dns.koshk.in/sub-domain` annotation

Then deploy it, run something similar with your own provider details
```
kubectl run kube-external-dns --image=arduima/kube-external-dns --env="CLOUDFLARE_EMAIL=$EMAIL" --env="CLOUDFLARE_KEY=$API_KEY"
```
*Each provider will require its own credentials and will require different env variables*

If you want to use multiple providers pass the credentials and use the appropriate annotation values from below

### List of Providers
* CloudFlare  
Requires: `CLOUDFLARE_EMAIL` and `CLOUDFLARE_KEY`   
Annotation: `external.dns.koshk.in/provider: "cloudflare"`  
* DNSimple
Requires: `DNSIMPLE_EMAIL` and `DNSIMPLE_TOKEN`   
Annotation: `external.dns.koshk.in/provider: "dnsimple"`  
* Route53
Requires: `AWS_REGION`, `AWS_ACCESS_KEY` and `AWS_SECRET_KEY`   
Annotation: `external.dns.koshk.in/provider: "route53"`  
* DigitalOcean  
Requires: `DO_PAT`  
Annotation: `external.dns.koshk.in/provider: "digitalocean"`  
