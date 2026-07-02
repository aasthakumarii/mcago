# mcago

`mcago` is a Go/Kubebuilder based Kubernetes controller project. The main idea is that Kubernetes stores the desired state of an object, and this controller keeps checking that object and taking actions so the real world matches that desired state.

This README is written for junior developers and trainees. It explains not only how to run the project, but also what each important file is responsible for and how the controller flow works.

## What This Project Does

This project follows the Kubernetes operator/controller pattern.

In a normal application, a user may click a button or call an API and the application immediately performs an action. In a Kubernetes controller, the flow is different:

1. A user creates or updates a Kubernetes object.
2. Kubernetes stores that object in the API server.
3. The controller receives an event saying something changed.
4. The controller reads the object.
5. The controller compares the desired state with the current state.
6. The controller creates, updates, deletes, or records status as needed.
7. Kubernetes may call the controller again later, so the logic must be safe to run many times.

This repeated check-and-fix loop is called reconciliation.

## Important Concepts

### Kubernetes Controller

A controller is a long-running program that watches Kubernetes resources and reacts when they change.

In this project, controller logic lives under:

```text
internal/controller/
```

The currently relevant controller file is:

```text
internal/controller/team_controller.go
```

### Reconciliation

Reconciliation means making the actual state match the desired state.

For example, if a `Team` object says that a team should exist with certain information, the controller checks what currently exists and performs whatever work is needed.

The reconcile function should be:

- idempotent, meaning it is safe to run multiple times
- defensive, because Kubernetes objects can be changed or deleted at any moment
- clear about errors, because returning an error tells controller-runtime to retry
- careful with updates, because Kubernetes objects can be modified concurrently

### Custom Resource

A custom resource is a Kubernetes object type added by this project.

Kubebuilder projects usually define custom resource types under:

```text
api/
```

Look for files ending in:

```text
*_types.go
```

Those files describe the shape of the Kubernetes objects that users can create.

### Model

The project also contains application/domain models under:

```text
internal/models/
```

The currently relevant model file is:

```text
internal/models/team.go
```

A model usually represents business data used inside the application. In this project, the `Team` model is likely the central domain object around which the controller and service logic are organized.

### Service

Service logic lives under:

```text
internal/service/
```

The currently relevant service file is:

```text
internal/service/team_service.go
```

Services are useful because they keep business logic out of the controller. A controller should mostly coordinate Kubernetes reads and writes, while a service can handle rules such as how teams are created, validated, transformed, or synchronized.

## Project Structure

The project uses the common Kubebuilder layout:

```text
cmd/main.go
api/
internal/controller/
internal/models/
internal/service/
config/
Makefile
PROJECT
```

### `cmd/main.go`

This is the application entry point.

When the manager process starts, this file usually does the following:

1. Creates a controller-runtime manager.
2. Registers all API schemes.
3. Registers controllers.
4. Registers webhooks, if the project has any.
5. Starts the manager.

The manager is the runtime process that keeps the controller alive.

### `api/`

This directory contains Kubernetes API definitions.

Important things usually found here:

- `Spec` structs: what the user wants
- `Status` structs: what the controller reports back
- Kubebuilder markers: comments that generate CRDs, RBAC, validation rules, and status subresources

Do not edit generated files such as:

```text
zz_generated.deepcopy.go
```

If you change API type definitions or Kubebuilder markers, regenerate generated files:

```bash
make manifests
make generate
```

### `internal/controller/`

This directory contains reconciliation logic.

The controller is responsible for watching Kubernetes objects and reacting to changes.

Important file:

```text
internal/controller/team_controller.go
```

Controller code commonly includes:

- RBAC markers
- a reconciler struct
- the `Reconcile` method
- helper methods for creating or updating related resources
- setup code that tells the manager which resources to watch

### `internal/service/`

This directory contains business logic.

Important file:

```text
internal/service/team_service.go
```

Service code should usually be easier to test than controller code because it does not need to know as much about Kubernetes internals.

### `internal/models/`

This directory contains internal Go data structures.

Important file:

```text
internal/models/team.go
```

Models are useful when the application needs a clean internal representation separate from Kubernetes API objects.

### `config/`

This directory contains Kubernetes manifests and Kustomize configuration generated or managed by Kubebuilder.

Common subdirectories:

```text
config/crd/
config/rbac/
config/manager/
config/samples/
```

Be careful with generated files under `config/`. Many of them are produced by Kubebuilder/controller-tools.

Do not manually edit generated CRDs or generated RBAC output. Instead, edit Go markers and run:

```bash
make manifests
```

### `Makefile`

The Makefile contains the standard developer commands.

Common commands:

```bash
make manifests
make generate
make test
make run
make docker-build
make deploy
make undeploy
```

## Runtime Flow

At a high level, the project runs like this:

```text
cmd/main.go
    starts the manager
        registers API schemes
        registers Team controller
        starts watching Kubernetes resources

Kubernetes object changes
    controller-runtime receives an event
        TeamReconciler.Reconcile is called
            controller loads the object
            controller calls service/model logic as needed
            controller updates Kubernetes resources or status
```

## How To Read The Code

For a trainee, the best reading order is:

1. Start with `cmd/main.go`.
   Understand how the manager starts and where controllers are registered.

2. Open `internal/controller/team_controller.go`.
   Find the reconciler struct and the `Reconcile` method. This is the heart of the controller.

3. Open `internal/service/team_service.go`.
   Understand what business logic is separated from the controller.

4. Open `internal/models/team.go`.
   Understand the internal data structures used by the service/controller.

5. Open API type files under `api/`.
   Understand what users define in Kubernetes and what the controller reports in status.

6. Open tests under `internal/controller/`.
   The file `internal/controller/team_controller_test.go` explains expected controller behavior.

## Local Development

### Prerequisites

Install the following tools:

- Go
- Docker
- kubectl
- kustomize
- controller-gen
- Kubebuilder
- Kind, if you want to run end-to-end tests in an isolated cluster

The exact versions should match the project configuration where possible.

### Run Unit Tests

```bash
make test
```

Kubebuilder controller tests often use envtest, which starts a local Kubernetes API server and etcd for tests.

### Run Locally

```bash
make run
```

This runs the controller on your machine using the current kubeconfig context.

Before using `make run`, always check which cluster your kubeconfig points to:

```bash
kubectl config current-context
```

### Generate Manifests

Run this after changing API markers, RBAC markers, or type definitions:

```bash
make manifests
```

This regenerates files such as CRDs and RBAC manifests.

### Generate DeepCopy Code

Run this after changing API structs:

```bash
make generate
```

This regenerates Kubernetes DeepCopy methods.

## Deployment

### Build And Push Image

Set the controller image:

```bash
export IMG=<registry>/<project>:<tag>
```

Build and push:

```bash
make docker-build docker-push IMG=$IMG
```

### Deploy To A Cluster

```bash
make deploy IMG=$IMG
```

### Apply Sample Resources

```bash
kubectl apply -k config/samples/
```

### View Controller Logs

The namespace and deployment name depend on the generated Kubebuilder configuration. Commonly, the controller runs in a namespace ending with `-system`.

Example:

```bash
kubectl logs -n <project>-system deployment/<project>-controller-manager -c manager -f
```

### Undeploy

```bash
make undeploy
```

## Testing Strategy

The main test file currently visible for the controller is:

```text
internal/controller/team_controller_test.go
```

Controller tests should verify behavior such as:

- the controller can read a `Team` resource
- expected dependent resources are created or updated
- status is updated correctly
- deletion/finalizer behavior works, if finalizers are used
- errors are handled correctly

When writing tests, prefer checking observable behavior instead of internal implementation details.

## Common Development Workflow

For normal Go code changes:

```bash
make lint-fix
make test
```

For API type or marker changes:

```bash
make manifests
make generate
make lint-fix
make test
```

For deployment testing:

```bash
export IMG=<registry>/<project>:<tag>
make docker-build IMG=$IMG
kind load docker-image $IMG --name <kind-cluster-name>
make deploy IMG=$IMG
kubectl apply -k config/samples/
```

## Important Rules For Contributors

### Do Not Edit Generated Files

Do not manually edit:

```text
config/crd/bases/*.yaml
config/rbac/role.yaml
config/webhook/manifests.yaml
**/zz_generated.*.go
PROJECT
```

Change the source Go files or Kubebuilder markers, then regenerate.

### Do Not Remove Scaffold Markers

Do not delete comments like:

```go
// +kubebuilder:scaffold:...
```

Kubebuilder uses these markers when adding new APIs, controllers, and webhooks.

### Use Kubebuilder For Scaffolding

Use Kubebuilder commands to create APIs and webhooks.

Example:

```bash
kubebuilder create api --group <group> --version <version> --kind <Kind>
```

Do not manually create a new Kubernetes API from scratch unless there is a very specific reason.

## Logging Guidelines

Use structured logs.

Good:

```go
log.Info("Created Deployment", "name", deployment.Name)
log.Error(err, "Failed to create Deployment", "name", deployment.Name)
```

Avoid vague logs:

```go
log.Info("Done")
log.Error(err, "Failed")
```

Log messages should:

- start with a capital letter
- not end with a period
- mention the object type when useful
- include key-value fields for important context

## Controller Design Guidelines

### Keep Reconcile Idempotent

The same reconcile request may run many times. The controller should not break if it sees the same object repeatedly.

Bad behavior:

- creating duplicate resources every time reconciliation runs
- failing just because a resource already exists
- assuming status is always current

Good behavior:

- get the current object
- compare desired state and actual state
- create only if missing
- update only if needed
- handle deleted objects gracefully

### Re-Fetch Before Updates When Needed

Kubernetes objects can change between read and update. If the controller is about to update an object after doing other work, re-fetch it first when conflict risk is high.

### Use Owner References

If the controller creates child resources, set the parent object as the owner.

This lets Kubernetes garbage collect child resources when the parent is deleted.

### Use Status For Observed State

The `spec` says what the user wants.

The `status` says what the controller observed or did.

Do not write controller results back into `spec`.

## Glossary

### API Server

The Kubernetes component that stores and serves Kubernetes objects.

### CRD

CustomResourceDefinition. A CRD teaches Kubernetes about a new object type.

### Custom Resource

An instance of a CRD.

### Desired State

What the user says should exist.

### Actual State

What currently exists.

### Reconcile

The controller loop that makes actual state match desired state.

### RBAC

Role-Based Access Control. Permissions that allow the controller to read, create, update, or delete Kubernetes resources.

### Scheme

The Kubernetes runtime registry that teaches the manager which Go types represent which Kubernetes objects.

### Status

The part of a Kubernetes object where the controller reports current state.

## Quick Reference

```bash
# Run tests
make test

# Run controller locally
make run

# Regenerate CRDs/RBAC
make manifests

# Regenerate DeepCopy code
make generate

# Build image
make docker-build IMG=<registry>/<project>:<tag>

# Deploy controller
make deploy IMG=<registry>/<project>:<tag>

# Apply sample resources
kubectl apply -k config/samples/
```

## Suggested First Exercise For A Trainee

1. Run the tests:

   ```bash
   make test
   ```

2. Read `cmd/main.go` and find where the controller is registered.

3. Read `internal/controller/team_controller.go` and find the `Reconcile` method.

4. Read `internal/controller/team_controller_test.go` and identify what behavior is expected.

5. Add one small test for a simple expected behavior.

6. Run:

   ```bash
   make test
   ```

This exercise helps connect the code, tests, and controller behavior without needing to deploy to a real cluster first.
