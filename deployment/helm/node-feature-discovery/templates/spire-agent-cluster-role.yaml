# Required cluster role to allow spire-agent to query k8s API server
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-agent-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods","nodes","nodes/proxy"]
  verbs: ["get"]

---
# Binds above cluster role to spire-agent service account
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: spire-agent-cluster-role-binding
subjects:
- kind: ServiceAccount
  name: spire-agent
  namespace: spire
roleRef:
  kind: ClusterRole
  name: spire-agent-cluster-role
  apiGroup: rbac.authorization.k8s.io
