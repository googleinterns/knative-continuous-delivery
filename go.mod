module github.com/googleinterns/knative-continuous-delivery

go 1.14

require (
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.0
	knative.dev/pkg v0.0.0-20200708171447-5358179e7499
	knative.dev/serving v0.16.0
	knative.dev/test-infra v0.0.0-20200708165947-2cd922769fa4
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
)
