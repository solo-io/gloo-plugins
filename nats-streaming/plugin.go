package natsstreaming

import (
	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"

	"github.com/gogo/protobuf/types"
	"github.com/solo-io/gloo/pkg/protoutil"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/pkg/errors"
	"github.com/solo-io/gloo-api/pkg/api/types/v1"
	"github.com/solo-io/gloo/pkg/coreplugins/common"
	"github.com/solo-io/gloo/pkg/plugin"
)

//go:generate protoc -I=. -I=${GOPATH}/src/github.com/gogo/protobuf/ --gogo_out=. nats_streaming_filter.proto

func init() {
	plugin.Register(&Plugin{}, nil)
}

type Plugin struct {
	filters []plugin.StagedFilter
}

const (
	ServiceTypeNatsStreaming = "nats-streaming"

	// generic plugin info
	filterName  = "io.solo.nats_streaming"
	pluginStage = plugin.OutAuth

	clusterId      = "cluster_id"
	discoverPrefix = "discover_prefix"

	defaultClusterId      = "test-cluster"
	defaultDiscoverPrefix = "_STAN.discover"
)

type ServiceProperties struct {
	ClusterID      string `json:"cluster_id"`
	DiscoverPrefix string `json:"discover_prefix"`
}

func EncodeServiceProperties(props ServiceProperties) *types.Struct {
	s, err := protoutil.MarshalStruct(props)
	if err != nil {
		panic(err)
	}
	return s
}

func (p *Plugin) GetDependencies(cfg *v1.Config) *plugin.Dependencies {
	return nil
}

func (p *Plugin) HttpFilters(params *plugin.FilterPluginParams) []plugin.StagedFilter {
	filters := p.filters
	p.filters = nil
	return filters
}

func (p *Plugin) ProcessUpstream(params *plugin.UpstreamPluginParams, in *v1.Upstream, out *envoyapi.Cluster) error {
	if in.ServiceInfo == nil || in.ServiceInfo.Type != ServiceTypeNatsStreaming {
		return nil
	}
	var props ServiceProperties
	err := protoutil.UnmarshalStruct(in.ServiceInfo.Properties, &props)
	if err != nil {
		return errors.Wrap(err, "unmarshalling serviceinfo.properties")
	}

	cid := props.ClusterID
	if cid == "" {
		cid = defaultClusterId
	}
	dp := props.DiscoverPrefix
	if dp == "" {
		dp = defaultDiscoverPrefix
	}
	if out.Metadata == nil {
		out.Metadata = &envoycore.Metadata{}
	}
	common.InitFilterMetadataField(filterName, clusterId, out.Metadata).Kind = &types.Value_StringValue{StringValue: defaultClusterId}
	common.InitFilterMetadataField(filterName, discoverPrefix, out.Metadata).Kind = &types.Value_StringValue{StringValue: dp}

	p.filters = append(p.filters, plugin.StagedFilter{HttpFilter: &envoyhttp.HttpFilter{Name: filterName, Config: natsConfig(out.Name)}, Stage: pluginStage})

	return nil
}

func natsConfig(cluster string) *types.Struct {
	natsStreaming := NatsStreaming{
		MaxConnections: 1,
		Cluster:        cluster,
	}

	filterConfig, err := protoutil.MarshalStruct(&natsStreaming)
	if err != nil {
		runtime.HandleError(err)
		return nil
	}
	return filterConfig
}

func (p *Plugin) ParseFunctionSpec(params *plugin.FunctionPluginParams, in v1.FunctionSpec) (*types.Struct, error) {
	if params.ServiceType != ServiceTypeNatsStreaming {
		return nil, nil
	}
	return nil, errors.New("functions are not required for service type " + ServiceTypeNatsStreaming)
}
