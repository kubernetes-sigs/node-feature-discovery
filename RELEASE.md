# Release Process

The node-feature-discovery code is released on an as-needed basis. The process
is as follows:

1. An issue is filed to propose a new release with a changelog since the last
   release. Copy the following checklist into the issue text:

- [ ] All [OWNERS](OWNERS) must LGTM the release proposal.
- [ ] Update the [job template](node-feature-discovery-job.json.template) to use the new tagged container image
- [ ] An OWNER runs `git tag -s $VERSION` and insert the changelog into the tag description.
- [ ] [Build and push](https://github.com/kubernetes-sigs/node-feature-discovery#building-from-source) a container image with the same tag to [quay.io](https://quay.io/kubernetes_incubator).
- [ ] Update the `:latest` virtual tag in quay.io to track the last stable (this) release.
- [ ] An OWNER pushes the tag with `git push $VERSION`.
- [ ] Write the change log into the [Github release info](https://github.com/kubernetes-sigs/node-feature-discovery/releases).
- [ ] Add a link to the tagged release in this issue.
- [ ] An announcement email is sent to `kubernetes-dev@googlegroups.com` with the
   subject `[ANNOUNCE] node-feature-discovery $VERSION is released`
- [ ] Close this issue.
