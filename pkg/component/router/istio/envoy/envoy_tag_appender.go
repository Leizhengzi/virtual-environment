package envoy

import (
	"alibaba.com/virtual-env-operator/pkg/shared"
	"context"
	protobuftypes "github.com/gogo/protobuf/types"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	networkingv1alpha3api "istio.io/client-go/pkg/apis/networking/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// delete EnvoyFilter instance if it already exist
func DeleteTagAppenderIfExist(client client.Client, namespace string, name string) error {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, &networkingv1alpha3api.EnvoyFilter{})
	if err == nil {
		return shared.DeleteIns(client, namespace, name, &networkingv1alpha3api.EnvoyFilter{})
	}
	return nil
}

// check whether EnvoyFilter is different
func IsDifferentTagAppender(tagAppender *networkingv1alpha3api.EnvoyFilter, envLabel string, envHeader string) bool {
	return tagAppender.ObjectMeta.Labels["envLabel"] != envLabel || tagAppender.ObjectMeta.Labels["envHeader"] != envHeader
}

// generate EnvoyFilter to auto append env tag into HTTP header
func TagAppenderFilter(namespace string, name string, envLabel string, envHeader string) *networkingv1alpha3api.EnvoyFilter {
	return &networkingv1alpha3api.EnvoyFilter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"envLabel":  envLabel,
				"envHeader": envHeader,
			},
		},
		Spec: networkingv1alpha3.EnvoyFilter{
			Filters: []*networkingv1alpha3.EnvoyFilter_Filter{{
				ListenerMatch: &networkingv1alpha3.EnvoyFilter_DeprecatedListenerMatch{
					ListenerType: networkingv1alpha3.EnvoyFilter_DeprecatedListenerMatch_SIDECAR_OUTBOUND,
				},
				FilterName: "envoy.lua",
				FilterType: networkingv1alpha3.EnvoyFilter_Filter_HTTP,
				FilterConfig: &protobuftypes.Struct{
					Fields: map[string]*protobuftypes.Value{
						"inlineCode": {
							Kind: &protobuftypes.Value_StringValue{
								StringValue: luaScript(envLabel, envHeader),
							},
						},
					},
				},
			}},
		},
	}
}

// generate lua script to auto inject env tag from label to header
func luaScript(envLabel string, envHeader string) string {
	return `
      local envLabel = "` + envLabel + `"
      local envHeader = "` + envHeader + `"
      local labels = os.getenv("ISTIO_METAJSON_LABELS")
      if labels ~= nil then
        local beginPos, endPos, curEnv
        _, beginPos = string.find(labels, '","' .. envLabel .. '":"', nil, true)
        if beginPos ~= nil then
          endPos = string.find(labels, '"', beginPos + 1)
          if endPos ~= nil and endPos > beginPos then
            curEnv = string.sub(labels, beginPos + 1, endPos - 1)
          end
        end
      else
        curEnv = os.getenv("VIRTUAL_ENVIRONMENT_TAG")
      end
      function envoy_on_request(request_handle)
        local env = request_handle:headers()[envHeader]
        if env == nil and curEnv ~= nil then
          request_handle:headers():add(envHeader, curEnv)
        end
      end
	`
}
