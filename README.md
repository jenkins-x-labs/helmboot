# helmboot

[![Documentation](https://godoc.org/github.com/jenkins-x-labs/helmboot?status.svg)](http://godoc.org/github.com/jenkins-x-labs/helmboot)
[![Go Report Card](https://goreportcard.com/badge/github.com/jenkins-x-labs/helmboot)](https://goreportcard.com/report/github.com/jenkins-x-labs/helmboot)


Helmboot is a small command line tool for System Administrators install / upgrade either [Jenkins](https://jenkins.io/) and / or [Jenkins X](https://jenkins-x.io/) via GitOps and immutable infrastructure.

Helmboot is based on [helm 3.x](https://helm.sh/) and [helmfile](https://github.com/roboll/helmfile) under the covers.

Helmboot uses the [Jenkins Operator](https://jenkinsci.github.io/kubernetes-operator/) to setup as many Jenkins servers are required in whatever namespaces via GitOps.

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

Once that is done you need to run the install Job

### Running the boot Job

The `helmboot create` or `helmboot upgrade` command should list the command to run the boot job.

If not you can run the following (adding in your own values of `CLUSTER_NAME` and `GIT_URL`):

```
helm install jx-boot \
  --set boot.clusterName=$CLUSTER_NAME \
  --set secrets.gsm.enabled=true \
  --set boot.bootGitURL=$GIT_URL \
```

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
  