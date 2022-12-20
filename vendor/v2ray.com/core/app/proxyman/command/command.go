package command

import (
	"context"

	grpc "google.golang.org/grpc"
	"v2ray.com/core"
	"v2ray.com/core/common"
	"v2ray.com/core/proxy"
)

// InboundOperation is the interface for operations that applies to inbound handlers.
type InboundOperation interface {
	// ApplyInbound applies this operation to the given inbound handler.
	ApplyInbound(context.Context, core.InboundHandler) error
}

// OutboundOperation is the interface for operations that applies to outbound handlers.
type OutboundOperation interface {
	// ApplyOutbound applies this operation to the given outbound handler.
	ApplyOutbound(context.Context, core.OutboundHandler) error
}

func getInbound(handler core.InboundHandler) (proxy.Inbound, error) {
	gi, ok := handler.(proxy.GetInbound)
	if !ok {
		return nil, newError("can't get inbound proxy from handler.")
	}
	return gi.GetInbound(), nil
}

// ApplyInbound implements InboundOperation.
func (op *AddUserOperation) ApplyInbound(ctx context.Context, handler core.InboundHandler) error {
	p, err := getInbound(handler)
	if err != nil {
		return err
	}
	um, ok := p.(proxy.UserManager)
	if !ok {
		return newError("proxy is not a UserManager")
	}
	return um.AddUser(ctx, op.User)
}

// ApplyInbound implements InboundOperation.
func (op *RemoveUserOperation) ApplyInbound(ctx context.Context, handler core.InboundHandler) error {
	p, err := getInbound(handler)
	if err != nil {
		return err
	}
	um, ok := p.(proxy.UserManager)
	if !ok {
		return newError("proxy is not a UserManager")
	}
	return um.RemoveUser(ctx, op.Email)
}

type handlerServer struct {
	s   *core.Instance
	ihm core.InboundHandlerManager
	ohm core.OutboundHandlerManager
}

func (s *handlerServer) AddInbound(ctx context.Context, request *AddInboundRequest) (*AddInboundResponse, error) {
	rawHandler, err := s.s.CreateObject(request.Inbound)
	if err != nil {
		return nil, err
	}
	handler, ok := rawHandler.(core.InboundHandler)
	if !ok {
		return nil, newError("not an InboundHandler.")
	}
	return &AddInboundResponse{}, s.ihm.AddHandler(ctx, handler)
}

func (s *handlerServer) RemoveInbound(ctx context.Context, request *RemoveInboundRequest) (*RemoveInboundResponse, error) {
	return &RemoveInboundResponse{}, s.ihm.RemoveHandler(ctx, request.Tag)
}

func (s *handlerServer) AlterInbound(ctx context.Context, request *AlterInboundRequest) (*AlterInboundResponse, error) {
	rawOperation, err := request.Operation.GetInstance()
	if err != nil {
		return nil, newError("unknown operation").Base(err)
	}
	operation, ok := rawOperation.(InboundOperation)
	if !ok {
		return nil, newError("not an inbound operation")
	}

	handler, err := s.ihm.GetHandler(ctx, request.Tag)
	if err != nil {
		return nil, newError("failed to get handler: ", request.Tag).Base(err)
	}

	return &AlterInboundResponse{}, operation.ApplyInbound(ctx, handler)
}

func (s *handlerServer) AddOutbound(ctx context.Context, request *AddOutboundRequest) (*AddOutboundResponse, error) {
	rawHandler, err := s.s.CreateObject(request.Outbound)
	if err != nil {
		return nil, err
	}
	handler, ok := rawHandler.(core.OutboundHandler)
	if !ok {
		return nil, newError("not an OutboundHandler.")
	}
	return &AddOutboundResponse{}, s.ohm.AddHandler(ctx, handler)
}

func (s *handlerServer) RemoveOutbound(ctx context.Context, request *RemoveOutboundRequest) (*RemoveOutboundResponse, error) {
	return &RemoveOutboundResponse{}, s.ohm.RemoveHandler(ctx, request.Tag)
}

func (s *handlerServer) AlterOutbound(ctx context.Context, request *AlterOutboundRequest) (*AlterOutboundResponse, error) {
	rawOperation, err := request.Operation.GetInstance()
	if err != nil {
		return nil, newError("unknown operation").Base(err)
	}
	operation, ok := rawOperation.(OutboundOperation)
	if !ok {
		return nil, newError("not an outbound operation")
	}

	handler := s.ohm.GetHandler(request.Tag)
	return &AlterOutboundResponse{}, operation.ApplyOutbound(ctx, handler)
}

type service struct {
	v *core.Instance
}

func (s *service) Register(server *grpc.Server) {
	RegisterHandlerServiceServer(server, &handlerServer{
		s:   s.v,
		ihm: s.v.InboundHandlerManager(),
		ohm: s.v.OutboundHandlerManager(),
	})
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, cfg interface{}) (interface{}, error) {
		s := core.MustFromContext(ctx)
		return &service{v: s}, nil
	}))
}
