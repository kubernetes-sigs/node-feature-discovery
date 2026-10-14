# Contributing guidelines

## How to become a contributor and submit your own code

### Contributor License Agreements

We'd love to accept your patches! Before we can take them, you need to sign the
Cloud Native Computing Foundation (CNCF) Contributor License Agreement (CLA),
either as an individual or for your company.

After you open your first pull request, the EasyCLA bot replies with your CLA
status and a link to sign the CLA. See
[CLA.md](https://github.com/kubernetes/community/blob/main/CLA.md) in the
Kubernetes community repository for details. Once the CLA is signed, we can
accept your pull requests.

### Contributing a Patch

1. Submit an issue describing your proposed change to the repo in question.
1. The [repo owners](OWNERS) will respond to your issue promptly.
1. If your proposed change is accepted, and you haven't already done so, sign a
   Contributor License Agreement (see details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

### Adding dependencies

If your patch depends on new packages, add them with Go modules (`go get`), run
`go mod tidy` and commit the updated `go.mod` and `go.sum` files.
