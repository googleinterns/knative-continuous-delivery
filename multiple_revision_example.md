\# Handling Multiple Revisions Simultaneously (Example)

Suppose we have a policy that contains 4 rollout stages: 0.1%, 1%, 10%, and 100%. The threshold for going from 0.1% to 1% is 10 seconds, the threshold for going from 1% to 10% is 30 seconds, and the threshold for going from 10% to 100% is 50 seconds (time thresholds do not have to be uniform across all stages). This is reflected in the `Policy` Custom Resource below:

```yaml
apiVersion: delivery.knative.dev/v1alpha1
kind: Policy
metadata:
  namespace: default
  name: example-policy
spec:
  mode: time
  defaultThreshold: 60
  stages:
  - percent: 0.1
    threshold: 10
  - percent: 1
    threshold: 30
  - percent: 10
    threshold: 50
```

Suppose we have 5 Revisions that follow this policy: R1, R2, R3, R4, and R5. R1 is assumed to be already applied, so it begins with 100% traffic. R2, R3, R4, R5 enter the pool at time 0, 20, 40, and 60 respectively.

Below is a table that shows the state of the system until it stabilizes (R5 reaches 100%):

| Time(s) | R1(%) | R2(%) | R3(%) | R4(%) | R5(%) | Event(s) causing change |
|---------|-------|-------|-------|-------|-------|-------------------------|
| -       | 100   | -     | -     | -     | -     |                         |
| 0       | 99.9  | 0.1   | -     | -     | -     | R2 enter                |
| 10      | 99    | 1     | -     | -     | -     | R2 promote              |
| 20      | 98.9  | 1     | 0.1   | -     | -     | R3 enter                |
| 30      | 98    | 1     | 1     | -     | -     | R3 promote              |
| 40      | 88.9  | 10    | 1     | 0.1   | -     | R4 enter, R2 promote    |
| 50      | 88    | 10    | 1     | 1     | -     | R4 promote              |
| 60      | 78.9  | 10    | 10    | 1     | 0.1   | R5 enter, R3 promote    |
| 70      | 78    | 10    | 10    | 1     | 1     | R5 promote              |
| 80      | 69    | 10    | 10    | 10    | 1     | R4 promote              |
| 90      | 0     | 79    | 10    | 10    | 1     | R2 promote              |
| 100     | 0     | 70    | 10    | 10    | 10    | R5 promote              |
| 110     | 0     | 0     | 80    | 10    | 10    | R3 promote              |
| 120     | 0     | 0     | 80    | 10    | 10    | -                       |
| 130     | 0     | 0     | 0     | 90    | 10    | R4 promote              |
| 140     | 0     | 0     | 0     | 90    | 10    | -                       |
| 150     | 0     | 0     | 0     | 0     | 100   | R5 promote              |
