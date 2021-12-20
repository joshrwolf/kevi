package webhook

import (
	"context"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"cattle.io/kevi/pkg/pack"
)

func AddPodRelocatorToManager(mgr manager.Manager, registry string) error {
	wh := &admission.Webhook{
		Handler: &podRelocatorHandler{
			registry: registry,
		},
	}

	server := mgr.GetWebhookServer()
	server.Register("/mutate", wh)
	server.StartedChecker()
	return nil
}

var _ admission.Handler = &podRelocatorHandler{}

type podRelocatorHandler struct {
	decoder  *admission.Decoder
	registry string
}

func (m *podRelocatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pod := &corev1.Pod{}

	if err := m.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	for i, c := range pod.Spec.InitContainers {
		rel, err := relocate(c.Image, m.registry)
		if err != nil {
			continue
		}
		pod.Spec.InitContainers[i].Image = rel
	}

	for i, c := range pod.Spec.Containers {
		rel, err := relocate(c.Image, m.registry)
		if err != nil {
			continue
		}
		pod.Spec.Containers[i].Image = rel
	}
	// TODO: Do we want this behavior?
	// If image exists in registry, relocate
	// if _, err := remote.Head(rel, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)); err == nil {
	// 	fmt.Println("relocating ", c.Image, " -> ", rel.Name())
	// 	c.Image = rel.Name()
	// }

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder injects the decoder.
func (m *podRelocatorHandler) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}

func relocate(original string, registry string) (string, error) {
	relocated, err := pack.Relocate(original, registry)
	if err != nil {
		return "", err
	}

	rel, err := name.ParseReference(relocated)
	if err != nil {
		return "", err
	}

	return rel.Name(), nil
}
