# Release Process

The node-feature-discovery code is released on an as-needed basis. The process
is as follows:

1. An issue is filed to propose a new release with a changelog since the last
   release.
1. All [OWNERS](OWNERS) must LGTM the release proposal.
1. An OWNER runs `git tag -s $VERSION` and inserts the changelog and pushes the
   tag with `git push $VERSION`.
1. The release issue is closed with links to the tagged release.
1. An announcement email is sent to `kubernetes-dev@googlegroups.com` with the
   subject `[ANNOUNCE] node-feature-discovery $VERSION is released`
