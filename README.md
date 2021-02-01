# Events

Http service which inmplements an endpoint to listen events in Kubernetes cluster (webhook), as well as alerts from Alertmanager. By receiving events and alerts, the service processes them based on their kind and generates human readable message which sends to Kafka, Telegram, Slack or Workchat.

[![GoDoc](https://godoc.org/github.com/devopsext/events?status.svg)](https://godoc.org/github.com/devopsext/events)
[![build status](https://img.shields.io/travis/devopsext/events/master.svg?style=flat-square)](https://travis-ci.org/devopsext/events)

## Features

- Consume events from Kubernetes API, support kinds: Namespace, Node, ReplicaSet, StatefulSet, DaemonSet, Secret, Ingress, CronJob, Job, ConfigMap, Role, Deployment, Service, Pod
- Consume alerts from Alertmanager and render alert images based on Grafana
- Support golang templates as patterns of messages for channels and channel selectors
- Template functions: regexReplaceAll, regexMatch, replaceAll, toLower, toTitle, toUpper, toJSON, split, join, isEmpty, getEnv, getVar, timeFormat, jsonEscape, toString
- Support channels like: Kafka, Telegram, Slack, Workchat. All templates in place
- Provide Prometheus metrics out of the box


## Build

```sh
git clone https://github.com/devopsext/events.git
cd events/
go build
```

## Example

<details>
  <summary>Create Kubernetes API event json file</summary>

```sh
cat <<EOF > k8s.json
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1beta1",
  "request": {
    "uid": "23172a7a-f4c6-11e9-953e-0050568aa55b",
    "kind": {
      "group": "",
      "version": "v1",
      "kind": "Pod"
    },
    "resource": {
      "group": "",
      "version": "v1",
      "resource": "pods"
    },
    "namespace": "nodegroup",
    "operation": "CREATE",
    "userInfo": {
      "username": "some-user",
      "uid": "380bb127-e96f-11e8-ae7d-0050568a9a8e",
      "groups": [
        "system:serviceaccounts",
        "system:serviceaccounts:kube-system",
        "system:authenticated"
      ]
    },
    "object": {
      "metadata": {
        "name": "someservice-php-order-1571746740-glbhp",
        "generateName": "someservice-php-order-1571746740-",
        "namespace": "nodegroup",
        "uid": "23171eb4-f4c6-11e9-953e-0050568aa55b",
        "creationTimestamp": "2019-10-22T12:19:04Z",
        "labels": {
          "controller-uid": "231132ee-f4c6-11e9-953e-0050568aa55b",
          "job-name": "someservice-php-order-1571746740",
          "k8s-app": "someservice-php-order",
          "platform.collector/injected": "true",
          "version": "v0.4"
        },
        "annotations": {
          "app": "someservice-php-order",
          "prometheus.io/path": "/metrics",
          "prometheus.io/port": "60000",
          "prometheus.io/scrape": "true"
        },
        "ownerReferences": [
          {
            "apiVersion": "batch/v1",
            "kind": "Job",
            "name": "someservice-php-order-1571746740",
            "uid": "231132ee-f4c6-11e9-953e-0050568aa55b",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ]
      },
      "spec": {
        "volumes": [
          {
            "name": "someservice-php-env-file-volume",
            "configMap": {
              "name": "someservice-php-env-file",
              "defaultMode": 420
            }
          },
          {
            "name": "default-token-mn7zd",
            "secret": {
              "secretName": "default-token-mn7zd",
              "defaultMode": 420
            }
          },
          {
            "name": "dockersock",
            "hostPath": {
              "path": "/var/run/docker.sock",
              "type": ""
            }
          },
          {
            "name": "platform-collector-token",
            "secret": {
              "secretName": "platform-collector-token",
              "defaultMode": 420
            }
          }
        ],
        "containers": [
          {
            "name": "someservice-php-order",
            "image": "someregistry.com/someservice-php:v0.4",
            "command": [
              "/bin/bash",
              "-c",
              "cd /var/www ; php -d memory_limit=512M artisan transform:order; echo \"Done\"; sleep 3"
            ],
            "env": [
              {
                "name": "POD_NAME",
                "valueFrom": {
                  "fieldRef": {
                    "apiVersion": "v1",
                    "fieldPath": "metadata.name"
                  }
                }
              }
            ],
            "resources": {
              "limits": {
                "cpu": "1",
                "memory": "200Mi"
              },
              "requests": {
                "cpu": "500m",
                "memory": "150Mi"
              }
            },
            "volumeMounts": [
              {
                "name": "someservice-php-env-file-volume",
                "mountPath": "/env"
              },
              {
                "name": "default-token-mn7zd",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "IfNotPresent"
          },
          {
            "name": "collector",
            "image": "collector/pod:1.9.3.11-1.1.0",
            "env": [
              {
                "name": "KAFKA_BROKERS",
                "value": "broker:9092"
              },
              {
                "name": "POD_NAME",
                "valueFrom": {
                  "fieldRef": {
                    "apiVersion": "v1",
                    "fieldPath": "metadata.name"
                  }
                }
              },
              {
                "name": "COLLECTOR_GLOBAL_TAGS_ORCHESTRATION",
                "value": "k8s.test.env"
              }
            ],
            "resources": {},
            "volumeMounts": [
              {
                "name": "platform-collector-token",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              },
              {
                "name": "dockersock",
                "readOnly": true,
                "mountPath": "/var/run/docker.sock"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "Always"
          }
        ],
        "restartPolicy": "Never",
        "terminationGracePeriodSeconds": 30,
        "dnsPolicy": "ClusterFirst",
        "nodeSelector": {
          "platform.isolation/nodegroup": "nodegroup"
        },
        "serviceAccountName": "default",
        "serviceAccount": "default",
        "securityContext": {},
        "imagePullSecrets": [
          {
            "name": "registry.exness.io"
          }
        ],
        "schedulerName": "default-scheduler",
        "tolerations": [
          {
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          },
          {
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          }
        ],
        "priority": 0
      },
      "status": {
        "phase": "Pending",
        "qosClass": "Burstable"
      }
    },
    "oldObject": null,
    "dryRun": false
  }
}
EOF
```
</details>

<details>
  <summary>Create Alertmanager alert json file</summary>

```sh
cat <<EOF > 
{
  "receiver": "events",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "Process Open FDS 2",
        "app": "prometheus",
        "instance": "10.42.0.5:9090",
        "kubernetes_namespace": "default",
        "kubernetes_pod_name": "prometheus-0",
        "severity": "some",
        "unit": "short",
        "minutes": "10",
        "statefulset_kubernetes_io_pod_name": "prometheus-0"
      },
      "annotations": {
        "summary": "High process Open FDS"
      },
      "startsAt": "2020-12-22T16:42:47.056441315Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus-0:9090/graph?g0.expr=rate(process_cpu_seconds_total[1m]) > 0.004&g0.tab=1",
      "fingerprint": "f8767e67485c740c"
    }
  ],
  "groupLabels": {
    "alertname": "Process Open FDS"
  },
  "commonLabels": {
    "alertname": "Process Open FDS",
    "app": "prometheus",
    "instance": "10.42.0.5:9090",
    "kubernetes_namespace": "default",
    "kubernetes_pod_name": "prometheus-0",
    "severity": "some",
    "statefulset_kubernetes_io_pod_name": "prometheus-0"
  },
  "commonAnnotations": {
    "summary": "High process Open FDS"
  },
  "externalURL": "http://alertmanager-db66d4578-dm696:9093",
  "version": "4",
  "groupKey": "{}/{}:{alertname=\"Process Open FDS\"}"
}
EOF
```
</details>

<details>
  <summary>Provide environment common variable</summary>
<br>
In a case of Grafana images based on alert rules which come from Alertmanager, setup environment variables for convinience. You can use command switches for that, but for the sake of simplicity, environment variables should be provided. 

```sh
export EVENTS_GRAFANA_URL="Place Grafana URL in case of Alertmanger images or leave it empty"
export EVENTS_GRAFANA_API_KEY="Place Grafana API key"
export EVENTS_LOG_FORMAT="stdout"
export EVENTS_LOG_LEVEL="debug"
export EVENTS_LOG_TEMPLATE="{{.msg}}"
```
</details>

<details>
  <summary>Run Events with Telegram channel</summary>

```sh
export TELEGRAM_BOT="Place Telegram bot"
export TELEGRAM_CHAT_ID="Place Telegram chat ID"
```

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --telegram-url "https://api.telegram.org/bot${TELEGRAM_BOT}/sendMessage?chat_id=${TELEGRAM_CHAT_ID}" \
         --telegram-message-template "{{- define \"telegram-message\"}}{{ toJSON . }}{{- end}}"
```

or

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --telegram-url "https://api.telegram.org/bot${TELEGRAM_BOT}/sendMessage?chat_id=${TELEGRAM_CHAT_ID}" \
         --telegram-message-template "telegram.message"
```

</details>

<details>
  <summary>Run Events with Slack channel</summary>

```sh
export SLACK_TOKEN="Place Slack token"
export SLACK_CHANNELS="Place Slack channels"
```

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --slack-url "https://slack.com/api/files.upload?token=${SLACK_TOKEN}&channels=${SLACK_CHANNELS}" \
         --slack-message-template "{{- define \"slack-message\"}}{{ toJSON . }}{{- end}}"
```

or

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --slack-url "https://slack.com/api/files.upload?token=${SLACK_TOKEN}&channels=${SLACK_CHANNELS}" \
         --slack-message-template "slack.message"
```
</details>

<details>
  <summary>Run Events with Workchat channel</summary>

```sh
export WORKCHAT_TOKEN="Place Workchat access token"
export WORKCHAT_RECIPIENT="Place Wotkchat thread group"
```

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --workchat-url "https://graph.workplace.com/v9.0/me/messages?access_token=${WORKCHAT_TOKEN}&recipient=%7B%22thread_key%22%3A%22${WORKCHAT_RECIPIENT}%22%7D" \
         --workchat-message-template "{{- define \"workchat-message\"}}{{ replaceAll \"\\\"\" \"\" (toJSON .) }}{{- end}}"
```

or

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --workchat-url "https://graph.workplace.com/v9.0/me/messages?access_token=${WORKCHAT_TOKEN}&recipient=%7B%22thread_key%22%3A%22${WORKCHAT_RECIPIENT}%22%7D" \
         --workchat-message-template "workchat.message"
```

</details>

<details>
  <summary>Run Events with Telegram, Slack and Workchat simultaniously</summary>

```sh
./events --http-listen :8081 --http-k8s-url /k8s --http-alertmanager-url /alertmanager \
         --telegram-url "https://api.telegram.org/bot${TELEGRAM_BOT}/sendMessage?chat_id=${TELEGRAM_CHAT_ID}" \
         --telegram-message-template "telegram.message" \
         --slack-url "https://slack.com/api/files.upload?token=${SLACK_TOKEN}&channels=${SLACK_CHANNELS}" \
         --slack-message-template "slack.message" \
         --workchat-url "https://graph.workplace.com/v9.0/me/messages?access_token=${WORKCHAT_TOKEN}&recipient=%7B%22thread_key%22%3A%22${WORKCHAT_RECIPIENT}%22%7D" \
         --workchat-message-template "workchat.message"
```

</details>

<details>
  <summary>Test Alertmanager endpoint</summary>

```sh
curl -X POST -H 'Content-type: application/json' -d @alertmanager.json http://127.0.0.1:8081/alertmanager
```

```json
{"Message":"OK"}
```
</details>

<details>
  <summary>Test Kubernetes API endpoint</summary>

```sh
curl -X POST -H 'Content-type: application/json' -d @k8s.json http://127.0.0.1:8081/k8s
```

```json
{"response":{"uid":"23172a7a-f4c6-11e9-953e-0050568aa55b","allowed":true}}
```
</details>

## Usage

```
Events command

Usage:
  events [flags]
  events [command]

Available Commands:
  help        Help about any command
  version     Print the version number

Flags:
      --grafana-api-key string                 Grafana API key
      --grafana-datasource string              Grafana datasource (default "Prometheus")
      --grafana-image-height int               Grafan image height (default 640)
      --grafana-image-width int                Grafan image width (default 1280)
      --grafana-org string                     Grafana org (default "1")
      --grafana-period int                     Grafana period in minutes (default 60)
      --grafana-timeout int                    Grafan timeout (default 60)
      --grafana-url string                     Grafana URL
  -h, --help                                   help for events
      --http-alertmanager-url string           Http Alertmanager url
      --http-cert string                       Http cert file or content
      --http-chain string                      Http CA chain file or content
      --http-k8s-url string                    Http K8s url
      --http-key string                        Http key file or content
      --http-listen string                     Http listen (default ":80")
      --http-rancher-url string                Http Rancher url
      --http-tls                               Http TLS
      --kafka-brokers string                   Kafka brokers
      --kafka-client-id string                 Kafka client id (default "events_kafka")
      --kafka-flush-frequency int              Kafka Producer flush frequency (default 1)
      --kafka-flush-max-messages int           Kafka Producer flush max messages (default 100)
      --kafka-message-template string          Kafka message template
      --kafka-net-dial-timeout int             Kafka Net dial timeout (default 30)
      --kafka-net-max-open-requests int        Kafka Net max open requests (default 5)
      --kafka-net-read-timeout int             Kafka Net read timeout (default 30)
      --kafka-net-write-timeout int            Kafka Net write timeout (default 30)
      --kafka-topic string                     Kafka topic (default "events")
      --log-format string                      Log format: json, text, stdout (default "stdout")
      --log-level string                       Log level: info, warn, error, debug, panic (default "debug")
      --log-template string                    Log template (default "{{.msg}}")
      --prometheus-listen string               Prometheus listen (default "127.0.0.1:8080")
      --prometheus-url string                  Prometheus endpoint url (default "/metrics")
      --slack-alert-expression string          Slack alert expression (default "g0.expr")
      --slack-message-template string          Slack message template
      --slack-selector-template string         Slack selector template
      --slack-timeout int                      Slack timeout (default 30)
      --slack-url string                       Slack URL
      --telegram-alert-expression string       Telegram alert expression (default "g0.expr")
      --telegram-disable-notification string   Telegram disable notification (default "false")
      --telegram-message-template string       Telegram message template
      --telegram-selector-template string      Telegram selector template
      --telegram-timeout int                   Telegram timeout (default 30)
      --telegram-url string                    Telegram URL
      --template-time-format string            Template time format (default "2006-01-02T15:04:05.999Z")
      --workchat-alert-expression string       Workchat alert expression (default "g0.expr")
      --workchat-message-template string       Workchat message template
      --workchat-notification-type string      Workchat notification type (default "REGULAR")
      --workchat-selector-template string      Workchat selector template
      --workchat-timeout int                   Workchat timeout (default 30)
      --workchat-url string                    Workchat URL

```

<details>
  <summary>Environment variables</summary>
<br>
For containerization purpose all command switches have environment variables analogs.

- EVENTS_LOG_FORMAT
- EVENTS_LOG_LEVEL
- EVENTS_LOG_TEMPLATE

- EVENTS_PROMETHEUS_URL
- EVENTS_PROMETHEUS_LISTEN

- EVENTS_TEMPLATE_TIME_FORMAT
- EVENTS_HTTP_K8S_URL
- EVENTS_HTTP_RANCHER_URL
- EVENTS_HTTP_ALERTMANAGER_URL
- EVENTS_HTTP_LISTEN
- EVENTS_HTTP_TLS
- EVENTS_HTTP_CERT
- EVENTS_HTTP_KEY
- EVENTS_HTTP_CHAIN

- EVENTS_COLLECTOR_ADDRESS
- EVENTS_COLLECTOR_MESSAGE_TEMPLATE

- EVENTS_KAFKA_CLIEND_ID
- EVENTS_KAFKA_MESSAGE_TEMPLATE
- EVENTS_KAFKA_BROKERS
- EVENTS_KAFKA_TOPIC
- EVENTS_KAFKA_FLUSH_FREQUENCY
- EVENTS_KAFKA_FLUSH_MAX_MESSAGES
- EVENTS_KAFKA_NET_MAX_OPEN_REQUESTS
- EVENTS_KAFKA_NET_DIAL_TIMEOUT
- EVENTS_KAFKA_NET_READ_TIMEOUT
- EVENTS_KAFKA_NET_WRITE_TIMEOUT

- EVENTS_TELEGRAM_MESSAGE_TEMPLATE
- EVENTS_TELEGRAM_SELECTOR_TEMPLATE
- EVENTS_TELEGRAM_URL
- EVENTS_TELEGRAM_TIMEOUT
- EVENTS_TELEGRAM_ALERT_EXPRESSION
- EVENTS_TELEGRAM_DISABLE_NOTIFICATION

- EVENTS_SLACK_MESSAGE_TEMPLATE
- EVENTS_SLACK_SELECTOR_TEMPLATE
- EVENTS_SLACK_URL
- EVENTS_SLACK_TIMEOUT
- EVENTS_SLACK_ALERT_EXPRESSION

- EVENTS_WORKCHAT_MESSAGE_TEMPLATE
- EVENTS_WORKCHAT_SELECTOR_TEMPLATE
- EVENTS_WORKCHAT_URL
- EVENTS_WORKCHAT_TIMEOUT
- EVENTS_WORKCHAT_ALERT_EXPRESSION
- EVENTS_WORKCHAT_NOTIFICATION_TYPE

- EVENTS_GRAFANA_URL
- EVENTS_GRAFANA_TIMEOUT
- EVENTS_GRAFANA_DATASOURCE
- EVENTS_GRAFANA_API_KEY
- EVENTS_GRAFANA_ORG
- EVENTS_GRAFANA_PERIOD
- EVENTS_GRAFANA_IMAGE_WIDTH
- EVENTS_GRAFANA_IMAGE_HEIGHT
</details>
