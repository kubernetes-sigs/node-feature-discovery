---
title: "Node Feature client cmdline reference"
parent: "Reference"
layout: default
nav_order: 9
---

# Commandline flags of nfd client
{: .no_toc}

## Table of contents
{: .no_toc .text-delta}

1. TOC
{:toc}

---
**The client is in the experimental `v1alpha1` version.**

To quickly view available command line flags execute `nfd --help`.

### -h, --help

Print usage and exit.

## compat

Image Compatibility commands.

### validate-node

Perform node validation based on its associated image compatibility artifact.

#### --image

The `--image` flag specifies the URL of the image containing compatibility metadata.

#### --plain-http

The `--plain-http` flag forces the use of HTTP protocol for all registry communications.
Default: `false`

#### --platform

The `--platform` flag specifies the artifact platform in the format `os[/arch][/variant][:os_version]`.

#### --tags

The `--tags` flag specifies a list of tags that must match the tags
set on the compatibility objects.

#### --output-json

The `--output-json` flag prints the output as a JSON object.

#### --registry-username

The `--registry-username` flag specifies the username for the registry.

#### --registry-password-stdin

The `--registry-password-stdin` flag enables reading of registry password from stdin.

#### --registry-token-stdin

The `--registry-token-stdin` flag enables reading of registry token from stdin.
