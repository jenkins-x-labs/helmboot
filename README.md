# helmboot

**NOTE: This current experiment is now closed. The work done and feedback we have received will be used to enhance Jenkins X in future versions**

**This code should not be used in production, or be adopted for usage. It should only be used to provide feedback to the Labs team.**

[![Documentation](https://godoc.org/github.com/jenkins-x-labs/helmboot?status.svg)](https://pkg.go.dev/mod/github.com/jenkins-x-labs/helmboot)
[![Go Report Card](https://goreportcard.com/badge/github.com/jenkins-x-labs/helmboot)](https://goreportcard.com/report/github.com/jenkins-x-labs/helmboot)
[![Releases](https://img.shields.io/github/release-pre/jenkins-x-labs/helmboot.svg)](https://github.com/jenkins-x-labs/helmboot/releases)
[![LICENSE](https://img.shields.io/github/license/jenkins-x-labs/helmboot.svg)](https://github.com/jenkins-x-labs/helmboot/blob/master/LICENSE)
[![Slack Status](https://img.shields.io/badge/slack-join_chat-white.svg?logo=slack&style=social)](https://slack.k8s.io/)

Helmboot is a small command line tool for System Administrators install / upgrade either [Jenkins](https://jenkins.io/) and / or [Jenkins X](https://jenkins-x.io/) via GitOps and immutable infrastructure.

Helmboot is based on [helm 3.x](https://helm.sh/) and [helmfile](https://github.com/roboll/helmfile) under the covers.

## Using Jenkins and / or Jenkins X

You can choose whether to use purely Jenkins; purely Jenkins X or a combination of Jenkins and Jenkins X. You also have full flexibility in GitOps to decide which Apps are installed in which namespaces etc.


## Creating a new installation

First create your kubernetes cluster. If you are using Google Cloud you can try [using gcloud directly](https://github.com/jenkins-x-labs/jenkins-x-installer#prerequisits). 
 
If not try the [usual Jenkins X way](https://jenkins-x.io/docs/getting-started/setup/create-cluster/).

Now run the `helmboot create` command:

``` 
helmboot create
```

This will create a new git repository for your installation.

Once that is done you need to setup the secrets and then run the boot Job

### Setting up Secrets

You need to specify a few secret values before you can boot up. 

To populate the Secrets run:


```
helmboot secrets edit
```                  

This will prompt you to enter all the missing Secrets required.


#### Importing and exporting

You can export the current secrets to the file system via

```
helmboot secrets export -f /tmp/mysecrets.yaml
```                  

Or to view them on the terminal...

```
helmboot secrets export -c
```                  

If you have an existing secrets.yaml file on the file system that looks kinda like this (with the actual values included)...

```yaml
secrets:
  adminUser:
    username: 
    password: 
  hmacToken: 
  pipelineUser:
    username: 
    token: 
    email:  
```

Then you can import this YAML file via:

```
helmboot secrets import -f /tmp/mysecrets.yaml
```                  

### Running the boot Job

Once you have created your git repository via `helmboot create` or `helmboot upgrade` and populated the secrets as shown above you can run the boot `Job` via:

```
helmboot run --git-url=https://github.com/myorg/env-mycluster-dev.git
```

Once you have booted up once you can omit the `git-url` argument as it can be discovered from the `dev` `Environment` resource:

```
helmboot run
```

This will use helm to install the boot Job and tail the log of the pod so you can see the boot job run. It looks like the boot process is running locally on your laptop but really it is all running inside a Pod inside Kubernetes.

## Upgrading a `jx install` or `jx boot` cluster on helm 2.x

You can use the `helmboot upgrade` command to help upgrade your existing Jenkins X cluster to helm 3 and helmfile.

Connect to the cluster you wish to migrate and run:

``` 
helmboot upgrade
```

and follow the instructions.

If your cluster is using GitOps the command will clone the development git repository to a temporary directory, modify it and submit a pull request.

If your cluster is not using GitOps then a new git repository will be created.

### Upgrading your cluster

Once you have the git repository for the upgrade you need to run the boot Job in a clean empty cluster.

To simplify things you may want to create a new cluster, connect to that and then boot from there. If you are happy with the results you can scale down/destroy the old one
  
