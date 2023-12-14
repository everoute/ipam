FROM golang:1.20

ENV ETCD_RELEASE_URL=https://github.com/etcd-io/etcd/releases/download
ENV ETCD_VERSION=v3.5.6
RUN curl -L ${ETCD_RELEASE_URL}/${ETCD_VERSION}/etcd-${ETCD_VERSION}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VERSION}-linux-amd64.tar && \
    tar -xf /tmp/etcd-${ETCD_VERSION}-linux-amd64.tar -C /usr/local/bin --strip-components=1 --extract etcd-${ETCD_VERSION}-linux-amd64/etcd

ENV KUBERNETES_RELEASE_URL=https://storage.googleapis.com/kubernetes-release/release
ENV KUBERNETES_VERSION=v1.24.17
RUN curl -L ${KUBERNETES_RELEASE_URL}/${KUBERNETES_VERSION}/bin/linux/amd64/kube-apiserver -o /usr/local/bin/kube-apiserver && \
    chmod +x /usr/local/bin/kube-apiserver
