package processor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/devopsext/events/common"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	admv1beta1 "k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimek8s "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var k8sProcessorRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_k8s_processor_requests",
	Help: "Count of all k8s processor requests",
}, []string{"k8s_processor_user", "k8s_processor_operation", "k8s_processor_channel", "k8s_processor_namespace", "k8s_processor_kind"})

type K8sProcessor struct {
	outputs *common.Outputs
}

type K8sUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type K8sData struct {
	Kind      string      `json:"kind"`
	Location  string      `json:"location"`
	Operation string      `json:"operation"`
	Object    interface{} `json:"object,omitempty"`
	User      *K8sUser    `json:"user"`
}

var (
	runtimeScheme = runtimek8s.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

func (p *K8sProcessor) prepareOperation(operation admv1beta1.Operation) string {

	return strings.Title(strings.ToLower(string(operation)))
}

func (p *K8sProcessor) sendEvent(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest, location string, o interface{}) {

	user := &K8sUser{Name: ar.UserInfo.Username, ID: ar.UserInfo.UID}

	e := common.Event{
		Channel: channel,
		Type:    "K8sEvent",
		Data: K8sData{
			Kind:      ar.Kind.Kind,
			Operation: p.prepareOperation(ar.Operation),
			Location:  location,
			Object:    o,
			User:      user,
		},
	}
	if span != nil {
		e.SetSpanContext(span.Context())
	}
	p.outputs.Send(&e)
}

func (p *K8sProcessor) processNamespace(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var namespace *corev1.Namespace

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &namespace); err != nil {

			log.Error("Couldn't unmarshal namespace object: %v", err)
		}
	}

	name := ar.Name

	if namespace != nil {
		name = namespace.Name
	}

	p.sendEvent(span, channel, ar, name, namespace)
}

func (p *K8sProcessor) processNode(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var node *corev1.Node

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &node); err != nil {

			log.Error("Couldn't unmarshal node object: %v", err)
		}
	}

	name := ar.Name

	if node != nil {
		name = node.Name
	}

	p.sendEvent(span, channel, ar, name, node)
}

func (p *K8sProcessor) processReplicaSet(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var replicaSet *appsv1.ReplicaSet

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &replicaSet); err != nil {

			log.Error("Couldn't unmarshal replicaSet object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if replicaSet != nil {
		name = replicaSet.Name
		namespace = replicaSet.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), replicaSet)
}

func (p *K8sProcessor) processStatefulSet(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var statefulSet *appsv1.StatefulSet

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &statefulSet); err != nil {

			log.Error("Couldn't unmarshal statefulSet object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if statefulSet != nil {
		name = statefulSet.Name
		namespace = statefulSet.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), statefulSet)
}

func (p *K8sProcessor) processDaemonSet(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var daemonSet *appsv1.DaemonSet

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &daemonSet); err != nil {

			log.Error("Couldn't unmarshal daemonSet object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if daemonSet != nil {

		name = daemonSet.Name
		namespace = daemonSet.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), daemonSet)
}

func (p *K8sProcessor) processSecret(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var secret *corev1.Secret

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &secret); err != nil {

			log.Error("Couldn't unmarshal secret object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if secret != nil {
		name = secret.Name
		namespace = secret.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), secret)
}

func (p *K8sProcessor) processIngress(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var ingress *netv1beta1.Ingress

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &ingress); err != nil {

			log.Error("Couldn't unmarshal ingress object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if ingress != nil {
		name = ingress.Name
		namespace = ingress.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), ingress)
}

func (p *K8sProcessor) processJob(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var job *batchv1.Job

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &job); err != nil {

			log.Error("Couldn't unmarshal job object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if job != nil {
		name = job.Name
		namespace = job.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), job)
}

func (p *K8sProcessor) processCronJob(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var cronJob *batchv1beta.CronJob

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &cronJob); err != nil {

			log.Error("Couldn't unmarshal cronjob object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if cronJob != nil {
		name = cronJob.Name
		namespace = cronJob.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), cronJob)
}

func (p *K8sProcessor) processConfigMap(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var configMap *corev1.ConfigMap

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &configMap); err != nil {

			log.Error("Couldn't unmarshal configMap object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if configMap != nil {
		name = configMap.Name
		namespace = configMap.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), configMap)
}

func (p *K8sProcessor) processRole(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var role *rbacv1.Role

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &role); err != nil {

			log.Error("Couldn't unmarshal role object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if role != nil {
		name = role.Name
		namespace = role.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), role)
}

func (p *K8sProcessor) processDeployment(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var deployment *appsv1.Deployment

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &deployment); err != nil {

			log.Error("Couldn't unmarshal deployment object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if deployment != nil {
		name = deployment.Name
		namespace = deployment.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), deployment)
}

func (p *K8sProcessor) processService(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var service *corev1.Service

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &service); err != nil {

			log.Error("Couldn't unmarshal service object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if service != nil {
		name = service.Name
		namespace = service.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), service)
}

func (p *K8sProcessor) processPod(span opentracing.Span, channel string, ar *admv1beta1.AdmissionRequest) {

	var pod *corev1.Pod

	if ar.Object.Raw != nil {
		if err := json.Unmarshal(ar.Object.Raw, &pod); err != nil {

			log.Error("Couldn't unmarshal pod object: %v", err)
		}
	}

	name := ar.Name
	namespace := ar.Namespace

	if pod != nil {
		name = pod.Name
		namespace = pod.Namespace
	}

	p.sendEvent(span, channel, ar, fmt.Sprintf("%s.%s", namespace, name), pod)
}

func (p *K8sProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {

	var body []byte

	var span opentracing.Span
	spanConext, err := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	if err == nil {
		span = common.TracerStartSpanChildOf(spanConext)
		defer span.Finish()
	}

	if r.Body != nil {

		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {

		log.Error("Empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		common.TracerSpanError(span, err)
		return
	}

	log.Debug("Body => %s", body)

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {

		log.Error("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		common.TracerSpanError(span, err)
		//span.Finish()
		return
	}

	var admissionResponse *admv1beta1.AdmissionResponse
	ar := admv1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {

		log.Error("Can't decode body: %v", err)
		common.TracerSpanError(span, err)
		/*span.SetTag("error", true)
		span.LogFields(
			opentracingLog.Error(err),
		)*/

		admissionResponse = &admv1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {

		req := ar.Request
		channel := strings.TrimLeft(r.URL.Path, "/")

		switch req.Kind.Kind {
		case "Namespace":
			p.processNamespace(span, channel, req)
		case "Node":
			p.processNode(span, channel, req)
		case "ReplicaSet":
			p.processReplicaSet(span, channel, req)
		case "StatefulSet":
			p.processStatefulSet(span, channel, req)
		case "DaemonSet":
			p.processDaemonSet(span, channel, req)
		case "Secret":
			p.processSecret(span, channel, req)
		case "Ingress":
			p.processIngress(span, channel, req)
		case "Job":
			p.processJob(span, channel, req)
		case "CronJob":
			p.processCronJob(span, channel, req)
		case "ConfigMap":
			p.processConfigMap(span, channel, req)
		case "Role":
			p.processRole(span, channel, req)
		case "Deployment":
			p.processDeployment(span, channel, req)
		case "Service":
			p.processService(span, channel, req)
		case "Pod":
			p.processPod(span, channel, req)
		}

		k8sProcessorRequests.WithLabelValues(req.UserInfo.Username, string(req.Operation), channel, req.Namespace, req.Kind.Kind).Inc()

		admissionResponse = &admv1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	admissionReview := admv1beta1.AdmissionReview{}
	if admissionResponse != nil {

		admissionReview.Response = admissionResponse
		if ar.Request != nil {

			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {

		log.Error("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		common.TracerSpanError(span, err)
	}

	if _, err := w.Write(resp); err != nil {

		log.Error("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		common.TracerSpanError(span, err)
	}

	//span.Finish()
}

func NewK8sProcessor(outputs *common.Outputs) *K8sProcessor {
	return &K8sProcessor{outputs: outputs}
}

func init() {
	prometheus.Register(k8sProcessorRequests)
}
