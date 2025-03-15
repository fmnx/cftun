package cfd

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	capnp "zombiezen.com/go/capnproto2"
	"zombiezen.com/go/capnproto2/rpc"
)

type TunnelAuth struct {
	AccountTag   string
	TunnelSecret []byte
}

type ClientInfo struct {
	ClientID []byte `capnp:"clientId"`
	Features []string
	Version  string
	Arch     string
}

type ConnectionOptions struct {
	Client          ClientInfo
	ReplaceExisting bool
}

type ConnectionDetails struct {
	UUID                    uuid.UUID
	Location                string
	TunnelIsRemotelyManaged bool
}

func (d *ConnectionDetails) UnmarshalCapnproto(s capnp.Struct) error {
	uuidPtr, err := s.Ptr(0)
	if err != nil {
		return err
	}
	if d.UUID, err = uuid.FromBytes(uuidPtr.Data()); err != nil {
		return err
	}
	locPtr, err := s.Ptr(1)
	if err != nil {
		return err
	}
	d.Location = locPtr.Text()
	d.TunnelIsRemotelyManaged = s.Bit(0)
	return nil
}

func RegisterConnection(ctx context.Context, stream io.ReadWriteCloser, connIndex byte, credentials *Credentials, connOptions *ConnectionOptions) (*ConnectionDetails, error) {
	client := rpc.NewConn(rpc.StreamTransport(stream), rpc.ConnLog(nil)).Bootstrap(ctx)
	call := &capnp.Call{
		Ctx: ctx,
		Method: capnp.Method{
			InterfaceID:   0xf71695ec7fe85497,
			MethodID:      0,
			InterfaceName: "tunnelrpc/proto/tunnelrpc.capnp:RegistrationServer",
			MethodName:    "registerConnection",
		},
		Options:    capnp.CallOptions{},
		ParamsSize: capnp.ObjectSize{DataSize: 1, PointerCount: 3},

		ParamsFunc: func(s capnp.Struct) error {
			authStruct, err := capnp.NewStruct(s.Segment(), capnp.ObjectSize{DataSize: 0, PointerCount: 2})
			if err != nil {
				return err
			}

			auth := credentials.Auth()

			_ = authStruct.SetText(0, auth.AccountTag)
			_ = authStruct.SetData(1, auth.TunnelSecret)

			if err = s.SetPtr(0, authStruct.ToPtr()); err != nil {
				return err
			}
			if err = s.SetData(1, credentials.TunnelID[:]); err != nil {
				return err
			}
			s.SetUint8(0, connIndex)
			optionsStruct, _ := capnp.NewStruct(s.Segment(), capnp.ObjectSize{DataSize: 1, PointerCount: 2})

			clientStruct, _ := capnp.NewStruct(optionsStruct.Segment(), capnp.ObjectSize{DataSize: 0, PointerCount: 4})
			c := connOptions.Client
			_ = clientStruct.SetData(0, c.ClientID)
			_ = clientStruct.SetText(2, c.Version)
			_ = clientStruct.SetText(3, c.Arch)

			_ = optionsStruct.SetPtr(0, clientStruct.ToPtr())
			optionsStruct.SetBit(1, connOptions.ReplaceExisting)
			return s.SetPtr(2, optionsStruct.ToPtr())
		},
	}

	respStruct, err := capnp.NewPipeline(client.Call(call)).GetPipeline(0).Struct()
	if err != nil {
		return nil, err
	}

	tag := respStruct.Uint16(0)
	ptr, err := respStruct.Ptr(0)
	if err != nil {
		return nil, err
	}
	if tag == 1 {
		var details ConnectionDetails
		if err := details.UnmarshalCapnproto(ptr.Struct()); err != nil {
			return nil, err
		}
		return &details, nil
	} else if tag == 0 {
		return nil, errors.New(ptr.Text())
	}
	return nil, fmt.Errorf("unknown result tag: %d", tag)
}
