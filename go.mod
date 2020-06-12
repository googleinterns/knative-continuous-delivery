module github.com/googleinterns/knative-continuous-delivery

go 1.14

require (
	github.com/google/addlicense v0.0.0-20200422172452-68a83edd47bc // indirect
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/pkg v0.0.0-20200603222317-b79e4a24ca50
	knative.dev/test-infra v0.0.0-20200606045118-14ebc4a42974
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
)
