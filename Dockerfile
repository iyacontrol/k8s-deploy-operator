FROM golang:1.12 as builder
WORKDIR /go/src/k8s.io/k8s-deploy-operator
ADD ./  /go/src/k8s.io/k8s-deploy-operator
RUN CGO_ENABLED=0 go build


FROM alpine:3.9
COPY --from=builder /go/src/k8s.io/k8s-deploy-operator/k8s-deploy-operator /usr/local/bin/k8s-deploy-operator
RUN chmod +x /usr/local/bin/k8s-deploy-operator
CMD ["k8s-deploy-operator"]


# FROM alpine:3.9
# COPY k8s-deploy-operator /usr/local/bin/k8s-deploy-operator
# RUN chmod +x /usr/local/bin/k8s-deploy-operator
# CMD ["k8s-deploy-operator"]


