---
title: results-forwarding
authors:
  - JAORMX
  - rhmdnd
reviewers: # Include a comment about what domain expertise a reviewer is expected to bring and what area of the enhancement you expect them to focus on. For example: - "@networkguru, for networking aspects, please look at IP bootstrapping aspect"
  - TBD
approvers:
  - TBD
api-approvers: # In case of new or modified APIs or API extensions (CRDs, aggregated apiservers, webhooks, finalizers). If there is no API change, use "None"
  - TBD
creation-date: 2022-07-15
last-updated: 2022-07-15
tracking-link: # link to the tracking ticket (for example: Jira Feature or Epic ticket) that corresponds to this enhancement
  - TBD
see-also:
  - "/enhancements/this-other-neat-thing.md"
replaces:
  - "/enhancements/that-less-than-great-idea.md"
superseded-by:
  - "/enhancements/our-past-effort.md"
---

# Compliance Results Forwarding

## Summary

This will allow for a flexible and extendible mechanism to forward compliance
results to a centralized service or storage endpoint.

## Motivation

While the current approach of generating results and remediations via CRDs
has worked so far for the Compliance Operator, this has several draw-backs that
prove painful in multi-cluster environments:

* `ComplianceCheckResults` and `ComplianceRemediations` are specific to the
  cluster being scanned. Comparing these CRDs across multiple clusters is
  cumbersome without some sort of filtering mechanism.

* `ComplianceCheckResults` and `ComplianceRemediations` take space in etcd. This
  space may be better used by other operation-critical components in
  resource-constrained deployments. These resources also scale with deployment
  infrastructure size. This can be a significant aspect to consider when
  running the Compliance Operator, or providing useful data for evidence and
  auditing.
  
* By using CRDs we depend on etcd, which is not a generic database and doesn't
  provide all the assurances that one would expect when querying persistent
  data stores. The following are a few short-comings of using etcd as a
  persistent store for compliance data:
  
  - It's non-trivial to do disaster recovery: An object snapshot contains
    CRD metadata which contains uniquely generated information (e.g., like
    UUIDs), making it hard to reproduce and replicate. S0me of this data is
    also irrelevant to compliance checks. Full etcd restore is a tedious and
    complicated
    [process](https://platform9.com/kb/kubernetes/restore-etcd-cluster-from-quorum-loss).

  - Values are limited to a size of 1.5Mb, imposing a limit on the data that
    can be persisted in etcd. This is especially applicable to evidence
    storage, which will be covered in a separate [enhancement
    proposal](https://github.com/ComplianceAsCode/compliance-operator/pull/66).

  - The current CRDs may be very verbose, resulting in large objects, which
    may interfer with known scalability
    [limitations](https://kubernetes.io/blog/2020/09/02/scaling-kubernetes-networking-with-endpointslices/#scalability-limitations-of-the-endpoints-api)
    in Kubernetes, causing performance and scalability issues.

While projects such as [StackRox](https://www.stackrox.io/) have successfully
integrated Compliance Operator using CRDs as the primary interface, we can
improve the experience of aggregating results by providing a forwarding
mechanism.

By sending results to a central store, we'll be able to:

* View compliance results and suggested fixes for multiple clusters in one place.

* Address the limitations mentioned above with etcd by choosing an alternative
  store (e.g., a relational database or a cloud-managed database.)

The Compliance Operator also stores raw results (Asset Reporting Format or ARF)
in a persistent volume claim by default. While this is useful, not everyone has
access to persistent storage. This issue is apparent in shared testing clusters
where automation attempts to delete the Compliance Operator namespace, only to
have the request fail because of a failed volume allocation.

### Goals

* Define and implement a stable API for forwarder implementations.

* Implement the ability to forward `ComplianceCheckResults` to an external endpoint.

* Implement the ability to forward `ComplianceRemediations` to an external endpoint.

* Implement a switch to disable in-cluster storage if forwarding is enabled and healthy.

### Non-Goals

* Deprecate the current CRD/PVC approach: This is still a use-case.

* Expand the "evidence" that's currently gathered by the Compliance Operator.
  This proposal focuses on forwarding `ComplianceCheckResults` and
  `ComplianceRemediations`.

* Implement a way to forward ARF reports. This may be solved by a separate
  [enhancement](https://github.com/ComplianceAsCode/compliance-operator/pull/66).

* Implement a way to forward XCCDF reports. This may be solved by a separate
  [enhancement](https://github.com/ComplianceAsCode/compliance-operator/pull/66).

## Proposal

The Compliance Operator will forward compliance results and remediation
recommendations if the supplied forwarding endpoint is valid. The Compliance
Operator will also continue to use CRDs and PVCs to store results,
remediations, and evidence as it does today.

A user may explicitly disable in-cluster storage if forwarding is enabled. The
Compliance Operator will not support disabling PVC storage without a valid
forwarding endpoint.

This will require changing the **aggregator** to always forward via a gRPC API
to a configured implementation. In this case, the **aggregator** will act as a
gRPC client. The **resultserver** then becomes a gRPC server, subject to the
evidence forwarding provider and will be renamed **evidence-persistor** and
responsible for writing results as CRDs. Alternatively, the **aggregator** may
forward results to an external system that uses the same gRPC API. This allows
users the ability to offload compliance results from the cluster.

Forwarder implementations would be configured via the `ScanSetting` and they'd
be called "providers". For backwards compatibility, we'd have defaults that
would point to the current mode of operation of the Compliance Operator (e.g.,
a `pvc` provider for evidence storage.)

For more detail on the proposed API, see the *API Extensions* section.

### User Stories

* As a Site Reliability Engineer managing multiple clusters, I'd like to
  offload compliance result storage to a dedicated system, instead of having
  each cluster own its own compliance data.

* As a Site Reliability Engineer managing multiple clusters, I'd like to have
  one place to view the compliance stance of my whole fleet.

### API Extensions

Given the scope of the change, we'd upgrade the `ScanSetting` version to
`v1alpha2`. The following sections describe the additional values needed by the
`ScanSetting` CRD.

#### `resultForwarding.provider`

Required: True
Type: string

The implementation to use for forwarding. The initial implementation will
support `grpc`.

#### `resultForwarding.grpc.endpoint`

Required: True
Type: string

Endpoint URL to a gRPC server for result storage. The service may be in or
outside the cluster. One potential implementation may be
[compserv](https://github.com/rhmdnd/compserv).

#### `resultForwarding.grpc.version`

Required: False
Default: `v1`
Type: string

#### `resultForwarding.grpc.tls.dynamic.caSecretName`

Required: True
Type: string

Reference to a CA certificate and key pair for TLS.

#### `resultForwarding.grpc.tls.dynamic.serverSecretName`

Required: True
Type: string

Reference to a server certificate/keypair secret for mutual TLS with the
forwarding endpoint.

#### `resultForwarding.grpc.tls.dynamic.clientSecretName`

Required: True if using mutual TLS
Type: string

Reference to a client certificate/keypair secret for mutual TLS.

#### `resultForwarding.grpc.authentication.serverCA`

Required: False
Type: string

An optional `ConfigMap` that provides a Certificate Authority (CA) bundle for
authenticating the server.

For mutual TLS authentication, the Compliance Operator will use the Certificate
Authority (CA) provided in the client certificate's PEM file, using
`mtlsClientCert`.

For token authentication, the `serverCA` should be provided to authenticate the
server's TLS connection. If the `serverCA` is not provided, the Compliance
Operator will rely on the default Certificate Authority (CA).

#### `resultForwarding.grpc.authentication.mtlsClientCert`

Required: False
Type: string

A reference to a Kubernetes `Secret` that stores the client certificate (PEM)
to authenticate the server.

#### `resultForwarding.grpc.authentication.token`

Required: False
Type: dict

A dictionary containing information that the Compliance Operator can use to
fetch a token for authentication.

#### `resultForwarding.grpc.authentication.token.secretName`

Required: False
Type: string

A reference to a Kubernetes `Secret` that stores the token.

#### `resultForwarding.grpc.authentication.token.secretNamespace`

Required: False
Type: string

Namespace of the Kubernetes `Secret` containing the token.

#### `resultForwarding.grpc.extraMetadata.clusterName`

Required: False
Type: string

The name of the cluster to use in result payloads.

#### `resultForwarding.grpc.extraMetadata.clusterType`

Required: False
Type: string

The type of the cluster to use in result payloads.

#### `resultForwarding.grpc.extraMetadata.randomKey`

Required: False
Type: string

This is simply and additional way to pass in data to the forwarding
implementation.

#### `ScanSetting` Examples

For reference, the following `ScanSetting` is available by default and
supported today with `v1alpha1` `ScanSetting` CRDs.

```yaml
apiVersion: compliance.openshift.io/v1alpha1
kind: ScanSetting
metadata:
  name: default
  namespace: openshift-compliance
rawResultStorage:
  pvAccessModes:
  - ReadWriteOnce 
  rotation: 3 
  size: 1Gi 
roles:
- worker 
- master 
scanTolerations: 
  default:
  - operator: Exists
  schedule: 0 1 * * * 
```

To forward results to a gRPC endpoint using mutual TLS:

```yaml
apiVersion: compliance.openshift.io/v1alpha1
kind: ScanSetting
metadata:
  name: default
  namespace: openshift-compliance
rawResultStorage:
  pvAccessModes:
  - ReadWriteOnce 
  rotation: 3 
  size: 1Gi 
resultForwarding:
  provider: grpc
  grpc:
    version: v1
    endpoint: https://compliance-endpoint.example.com
    tls:
      dynamic: 
        caSecretName: $caSecretName
        serverSecretName: $ref
        clientSecretName: $ref
    authentication:
      serverCA: $ConfigMap
      mtlsClientCert: $secretRef
    extraMetadata:
      clusterName: foo
      clusterType: bar
roles:
- worker 
- master 
scanTolerations: 
  default:
  - operator: Exists
  schedule: 0 1 * * * 
```

The following forwards results to a gRPC endpoint using token authentication.
It also sets additional metadata for the gRPC implementation.

```yaml
apiVersion: compliance.openshift.io/v1alpha1
kind: ScanSetting
metadata:
  name: default
  namespace: openshift-compliance
rawResultStorage:
  pvAccessModes:
  - ReadWriteOnce 
  rotation: 3 
  size: 1Gi 
resultForwarding:
  provider: grpc
  grpc:
    version: v1
    endpoint: https://compliance-endpoint.example.com
    tls:
      dynamic: 
        caSecretName: $caSecretName
    authentication:
      serverCA: $ConfigMap
      token:
        secretName: $secretRef
        secretNamespace: openshift-compliance
    extraMetadata:
      clusterName: foo
      clusterType: bar
      randomKey: random value
roles:
- worker 
- master 
scanTolerations: 
  default:
  - operator: Exists
  schedule: 0 1 * * * 
```

#### gRPC API

This section focuses solely on the details for implementing a forwarder for
gRPC.

```protobuf
message Result {
  string subject = 1;
  string control = 2;
  string rule = 3;
  string assessment_id = 4;
  string outcome = 5;
  string description = 6;
  string severity = 7;
  string instructions = 8;
  map extra = 9;
}
```

Example:

* `subject`: `cluster.example.com`
  - A unique name for the cluster
* `control`: `NIST-800-53: CM-6;CM-6(1)`
  - A list of controls this check is applicable to
* `rule`: `ocp4-api-server-admission-control-plugin-alwayspullimages`
  - The rule name as defined in the ComplianceAsCode/content
* `assessment_id`: `108268c9-af42-4004-bf43-2729648d63da`
  - The unique ID of the scan performed by the operator
* `outcome`: `PASS`
  - The status of the rule (e.g., `PASS`, `FAIL`, `MANUAL`)
* `description`: `Ensure that the Admission Control Plugin AlwaysPullImages is not set ...`
  - Description as defined in the ComplianceAsCode/content
* `severity`: `high`
  - How severe this finding is, if it's set by the ComplianceAsCode/content
* `instructions`: `Run the following command: $ oc -n openshift-kube-apiserver ...`
  - Instructions as defined in the ComplianceAsCode/content
* `extra`: `{'resourceType': 'cluster'}`
  - Additional data provided by the Compliance Operator or configured through the `ScanSetting`.

### Implementation Details/Notes/Constraints

#### Work Items

The following descibes the work needed to implement this feature, listed in
order of implementation.

1. Implement `ScanSetting` configuration options and bump the version to
   `v1alpha2`
2. Implement an interface for forwarding `ComplianceCheckResults` and
   `ComplianceRemediations`
3. Create a gRPC forwarding implementation
4. Implement an idempotent process that creates a globally-readable `ConfigMap`
   with the cluster ID
5. Add `aggregator` support to forward `ComplianceCheckResults` and
   `ComplianceRemediations` using the forwarding interface
6. Implement a webhook that converts `v1alpha1` `ScanSetting` objects to `v1alpha2`

### Risks and Mitigations

`v1alpha1` objects would be easily translated to `v1alpha2` objects, but not the
other way around. This needs to be thoroughly documented and communicated.

## Design Details

### Open Questions

1. If the configured provider for either the result forwarder or the evidence
   persistor would be unavailable, do we error the scan entirely?

### Test Plan

The base case of generating CRDs and storing evidence in PVCs would be covered
in our prow-based pre-existing CI.

New cases with forwarders would be introduced in more light-weight testing
environments. e.g. we could deploy a KinD cluster via a GitHub action and run
[MinIO](https://min.io/) to test the s3 provider. We'd then need to provide a
reference GRPC receiver for the test.

### Upgrade / Downgrade Strategy

Upgrade expectations:
- Existing `ScanSettings` should simply work and be seamlessly translated to `v1alpha2`

Downgrade expectations:
- Compliance Operator, as it is today, does not provide Downgrade options. This
  is not expected to change.

### Version Skew Strategy (TODO)

How will the component handle version skew with other components?
What are the guarantees? Make sure this is in the test plan.

Consider the following in developing a version skew strategy for this
enhancement:
- During an upgrade, we will always have skew among components, how will this impact your work?
- Does this enhancement involve coordinating behavior in the control plane and
  in the kubelet? How does an n-2 kubelet without this feature available behave
  when this feature is used?
- Will any other components on the node change? For example, changes to CSI, CRI
  or CNI may require updating that component before the kubelet.

### Operational Aspects of API Extensions (TODO)

Requires a network connection to forward results after each scan. The
Compliance Operator should validate the endpoint URL and fail early if it is
malformed. If the Compliance Operator cannot connect to the gRPC endpoint, it
should retry and issue an alert.

#### Failure Modes 

- If the gRPC forwarder or the evidence persistor error out due to the endpoint
  not being available for any reason outside of authentication or
  authorization; we'd need to output relevant Kubernetes events and perform
  retry logic with a reasonable timeout.

#### Support Procedures

In the event the `endpoint` is misconfigured, an SRE is expected to update it
accordingly. SRE will be able to detect this issue by monitoring alerts sent to
the `openshift-compliance` namespace.

SREs can remediate the issue by checking the `endpoint` and updating it by
editing the `ScanSetting` resource.

If establishing a connection to the gRPC service is problematic, SREs can still
store scan results as CRDs and on persistent volume claims.

## Implementation History

Major milestones in the life cycle of a proposal should be tracked in `Implementation
History`.

## Drawbacks

The idea is to find the best form of an argument why this enhancement should _not_ be implemented.

## Alternatives

Similar to the `Drawbacks` section the `Alternatives` section is used to
highlight and record other possible approaches to delivering the value proposed
by an enhancement.

## Infrastructure Needed [optional]

Use this section if you need things from the project. Examples include a new
subproject, repos requested, github details, and/or testing infrastructure.

Listing these here allows the community to get the process for these resources
started right away.

The testing for this feature can be accomplished on a Kubernetes or OpenShift
cluster. We may consider implementing a stub gRPC inferface simply for testing
purposes that lives in the same cluster. This testing implementation would
throw away the results.

Alternatively, we could consider deploying an actual compliance service, like
[compserv](https://github.com/rhmdnd/compserv), if it progresses to the point
where it can consume results from the Compliance Operator.
