package client

import (
	"time"

	goclient "github.com/micro/go-micro/v3/client"
	"github.com/micro/go-micro/v3/registry"
	pb "github.com/micro/micro/v3/proto/registry"
	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/context"
	"github.com/micro/micro/v3/service/errors"
	"github.com/micro/micro/v3/service/registry/util"
)

var name = "registry"

type srv struct {
	opts registry.Options
	// address
	address []string
	// client to call registry
	client pb.RegistryService
}

func (s *srv) callOpts() []goclient.CallOption {
	opts := []goclient.CallOption{goclient.WithAuthToken()}

	// set registry address
	if len(s.address) > 0 {
		opts = append(opts, goclient.WithAddress(s.address...))
	}

	// set timeout
	if s.opts.Timeout > time.Duration(0) {
		opts = append(opts, goclient.WithRequestTimeout(s.opts.Timeout))
	}

	s.client = pb.NewRegistryService(name, client.DefaultClient)
	return opts
}

func (s *srv) Init(opts ...registry.Option) error {
	for _, o := range opts {
		o(&s.opts)
	}

	if len(s.opts.Addrs) > 0 {
		s.address = s.opts.Addrs
	}

	return nil
}

func (s *srv) Options() registry.Options {
	return s.opts
}

func (s *srv) Register(srv *registry.Service, opts ...registry.RegisterOption) error {
	var options registry.RegisterOptions
	for _, o := range opts {
		o(&options)
	}

	// encode srv into protobuf and pack TTL and domain into it
	pbSrv := util.ToProto(srv)
	pbSrv.Options.Ttl = int64(options.TTL.Seconds())
	pbSrv.Options.Domain = options.Domain

	// register the service
	_, err := s.client.Register(context.DefaultContext, pbSrv, s.callOpts()...)
	return err
}

func (s *srv) Deregister(srv *registry.Service, opts ...registry.DeregisterOption) error {
	var options registry.DeregisterOptions
	for _, o := range opts {
		o(&options)
	}

	// encode srv into protobuf and pack domain into it
	pbSrv := util.ToProto(srv)
	pbSrv.Options.Domain = options.Domain

	// deregister the service
	_, err := s.client.Deregister(context.DefaultContext, pbSrv, s.callOpts()...)
	return err
}

func (s *srv) GetService(name string, opts ...registry.GetOption) ([]*registry.Service, error) {
	var options registry.GetOptions
	for _, o := range opts {
		o(&options)
	}

	rsp, err := s.client.GetService(context.DefaultContext, &pb.GetRequest{
		Service: name, Options: &pb.Options{Domain: options.Domain},
	}, s.callOpts()...)

	if verr := errors.Parse(err); verr != nil && verr.Code == 404 {
		return nil, registry.ErrNotFound
	} else if err != nil {
		return nil, err
	}

	services := make([]*registry.Service, 0, len(rsp.Services))
	for _, service := range rsp.Services {
		services = append(services, util.ToService(service))
	}
	return services, nil
}

func (s *srv) ListServices(opts ...registry.ListOption) ([]*registry.Service, error) {
	var options registry.ListOptions
	for _, o := range opts {
		o(&options)
	}

	req := &pb.ListRequest{Options: &pb.Options{Domain: options.Domain}}
	rsp, err := s.client.ListServices(context.DefaultContext, req, s.callOpts()...)
	if err != nil {
		return nil, err
	}

	services := make([]*registry.Service, 0, len(rsp.Services))
	for _, service := range rsp.Services {
		services = append(services, util.ToService(service))
	}

	return services, nil
}

func (s *srv) Watch(opts ...registry.WatchOption) (registry.Watcher, error) {
	var options registry.WatchOptions
	for _, o := range opts {
		o(&options)
	}

	stream, err := s.client.Watch(context.DefaultContext, &pb.WatchRequest{
		Service: options.Service, Options: &pb.Options{Domain: options.Domain},
	}, s.callOpts()...)

	if err != nil {
		return nil, err
	}

	return newWatcher(stream), nil
}

func (s *srv) String() string {
	return "service"
}

// NewRegistry returns a new registry service client
func NewRegistry(opts ...registry.Option) registry.Registry {
	var options registry.Options
	for _, o := range opts {
		o(&options)
	}

	// the registry address
	addrs := options.Addrs

	// don't default the address if a proxy is being used, as the
	// address will take precedent, circumventing the proxy.
	if len(addrs) == 0 {
		addrs = []string{"127.0.0.1:8000"}
	}

	return &srv{
		opts:    options,
		address: addrs,
		client:  pb.NewRegistryService(name, client.DefaultClient),
	}
}
