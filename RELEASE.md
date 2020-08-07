# Release Process

The node-feature-discovery code is released on an as-needed basis. The process
is as follows:

1. An issue is filed to propose a new release with a changelog since the last
   release. Copy the following checklist into the issue text:

- [ ] All [OWNERS](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/OWNERS) must LGTM the release proposal.
- [ ] Update the deployment templates ([master](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/nfd-master.yaml.template), [worker-daemonset](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/nfd-worker-daemonset.yaml.template), [worker-job](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/nfd-worker-job.yaml.template) and [combined](https://github.com/kubernetes-sigs/node-feature-discovery/blob/master/nfd-daemonset-combined.yaml.template))to use the new tagged container image
- [ ] An OWNER runs `git tag -s $VERSION` and insert the changelog into the tag description.
- [ ] An OWNER pushes the tag with `git push $VERSION` (this will also build and push a release container image to quay.io).
- [ ] An OWNER pulls the newly tagged image from quay.io, tags it with `gcr.io/k8s-staging-nfd/node-feature-discovery:$VERSION` and pushes it to `gcr.io/k8s-staging-nfd`
- [ ] Create a PR against [k8s.io](https://github.com/kubernetes/k8s.io), updading `k8s.gcr.io/images/k8s-staging-nfd/images.yaml` to promote the release image into production.
- [ ] Wait for the PR to be merged and verify that the image (`k8s.gcr.io/nfd/node-feature-discovery:$VERSION`) is available.
- [ ] Write the change log into the [Github release info](https://github.com/kubernetes-sigs/node-feature-discovery/releases).
- [ ] Add a link to the tagged release in this issue.
- [ ] An announcement email is sent to `kubernetes-dev@googlegroups.com` with the
   subject `[ANNOUNCE] node-feature-discovery $VERSION is released`. Add a link to the release announcement here.
- [ ] Close this issue.
