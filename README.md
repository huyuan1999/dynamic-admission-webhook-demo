#### 使用 k8s ca 签署证书
```shell
# 创建随机数种子文件
$ openssl rand -writerand .rnd

# 生成证书 key 文件
$ openssl genrsa -out server.key 2048

# 生成证书请求文件
$ openssl req -new -key server.key -out server.csr -config openssl.cnf

# 查看证书请求文件信息
$ openssl req -text -noout -in server.csr

# 签发证书
$ openssl x509 -req -in server.csr -CA /etc/kubernetes/pki/ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out server.crt -days 365 -extensions v3_req -extfile openssl.cnf 

# 查看证书信息
$ openssl x509 -in server.crt -noout -text 

# 验证证书
$ openssl verify -CAfile /etc/kubernetes/pki/ca.crt server.crt
```

#### 部署
```shell
# 创建 secret 资源
$ kubectl create secret generic webhook-server --from-file=server.key --from-file=server.crt

# 编译打包
$ CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build .

$ docker build -t hub.virtaitech.com/huyuan/admission-server:v1 .

$ kubectl apply -f deploy/admission-server-deploy.yaml
```

