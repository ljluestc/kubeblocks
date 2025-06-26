# InstanceSet

InstanceSet is a Kubernetes custom resource in KubeBlocks that manages a set of identical Pods, similar to a StatefulSet or Deployment.

## InstanceSet Recovery

The `InstanceSet` controller supports recovery from an invalid template. If Pods are stuck in the `Pending` phase due to an unsatisfiable template (e.g., invalid anti-affinity rules), updating the `InstanceSet` to a valid template triggers the deletion of `Pending` Pods and their recreation with the new template, allowing all replicas to transition to the `Running` phase.

### Example

Apply an `InstanceSet` with a bad template:

```yaml
apiVersion: workloads.kubeblocks.io/v1alpha1
kind: InstanceSet
metadata:
  name: test-instanceset
  namespace: default
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: app
        image: nginx:latest
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: nonexistent
            topologyKey: kubernetes.io/hostname
```

If Pods are Pending, update the template to remove the anti-affinity:

```yaml
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx:latest
```

The controller reconciles the InstanceSet, deletes Pending Pods, and recreates them with the new template, ensuring all replicas are Running.
