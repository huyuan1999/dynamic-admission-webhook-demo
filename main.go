// @Time    : 2021/12/17 11:45 上午
// @Author  : HuYuan
// @File    : main.go
// @Email   : huyuan@virtaitech.com

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"log"
	"net/http"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()

	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)
)

type patchType struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

var (
	tlsCertFile string
	tlsPrivateKeyFile string
)

func main() {
	flag.StringVar(&tlsCertFile, "tls-cert-file", "/etc/pki/server.crt", "File containing x509 Certificate used for serving HTTPS")
	flag.StringVar(&tlsPrivateKeyFile, "tls-private-key-file", "/etc/pki/server.key", "File containing x509 private key matching --tls-cert-file")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", admissionHandler)
	mux.HandleFunc("/mutate", admissionHandler)

	httpServer := &http.Server{
		Addr:    ":8999",
		Handler: mux,
	}

	if err := httpServer.ListenAndServeTLS(tlsCertFile, tlsPrivateKeyFile); err != nil {
		log.Fatalln(err.Error())
	}
}

func admissionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		glog.Error("request body is nil")
		http.Error(w, "request body is nil", http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		glog.Errorf("read request body error: %s", err.Error())
		http.Error(w, "read request body error", http.StatusInternalServerError)
		return
	}

	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1.AdmissionResponse
	ar := v1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		if r.URL.Path == "/validate" {
			admissionResponse = validateFunc(&ar)
		} else if r.URL.Path == "/mutate" {
			admissionResponse = mutateFunc(&ar)
		}
	}

	admissionReview := v1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	admissionReview.Kind = "AdmissionReview"
	admissionReview.APIVersion = "admission.k8s.io/v1"
	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func validateFunc(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	req := ar.Request
	if req.Kind.Kind != "Pod" {
		glog.Infof("Skipping validation for %s/%s due to policy check", req.Namespace, req.Name)
		return &v1.AdmissionResponse{
			Allowed: true,
		}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	podName, podNS, podMeta := pod.Name, pod.Namespace, pod.ObjectMeta
	availableLabels := pod.Labels
	glog.Infof("podName: %s podNS: %s podMeta: %v availableLabels: %v", podName, podNS, podMeta, availableLabels)

	glog.Infof("admission validateFunc response success")

	return &v1.AdmissionResponse{
		Allowed: true,
		Result:  nil,
	}
}

func mutateFunc(ar *v1.AdmissionReview) *v1.AdmissionResponse {
	req := ar.Request

	if req.Kind.Kind != "Pod" {
		glog.Infof("Skipping validation for %s/%s due to policy check", req.Namespace, req.Name)
		return &v1.AdmissionResponse{
			Allowed: true,
		}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	podName, podNS, podMeta := pod.Name, pod.Namespace, pod.ObjectMeta
	availableLabels := pod.Labels

	glog.Infof("podName: %s podNS: %s podMeta: %v availableLabels: %v", podName, podNS, podMeta, availableLabels)


	var patchData []patchType
	values := make(map[string]string)
	values["orion-vgpu"] = "true"
	patchData = append(patchData, patchType{
		Op:    "add",
		Path:  "/metadata/labels",
		Value: values,
	})

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		glog.Errorf("Could not marshal patch data: %v", err)
		return &v1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	glog.Infof("Patch data: %v", string(patchBytes))
	return &v1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1.PatchType {
			pt := v1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}
