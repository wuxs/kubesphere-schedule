## 安装手册


### 创建 namespace
```
kubectl create namespace kubesphere-schedule-sample 
```

### 安装 crane
```
cd charts/crane
helm install . --generate-name -n kubesphere-schedule-system
```

### 安装CRD
```
kustomize build config/crd | kubectl apply -f -
```