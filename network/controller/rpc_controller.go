/*
 *    Copyright 2018 Insolar
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package controller

import (
	"context"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"github.com/insolar/insolar/component"
	"github.com/insolar/insolar/metrics"

	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/network"
	"github.com/insolar/insolar/network/cascade"
	"github.com/insolar/insolar/network/controller/common"
	"github.com/insolar/insolar/network/transport/packet/types"
	"github.com/pkg/errors"
)

type RPCController interface {
	component.Starter

	// hack for DI, else we receive ServiceNetwork injection in RPCController instead of rpcController that leads to stack overflow
	IAmRPCController()

	SendMessage(nodeID core.RecordRef, name string, msg core.Parcel) ([]byte, error)
	SendCascadeMessage(data core.Cascade, method string, msg core.Parcel) error
	RemoteProcedureRegister(name string, method core.RemoteProcedure)
}

type rpcController struct {
	Scheme core.PlatformCryptographyScheme `inject:""`

	options     *common.Options
	hostNetwork network.HostNetwork
	methodTable map[string]core.RemoteProcedure
}

type RequestRPC struct {
	Method string
	Data   [][]byte
}

type ResponseRPC struct {
	Success bool
	Result  []byte
	Error   string
}

type RequestCascade struct {
	TraceID string
	RPC     RequestRPC
	Cascade core.Cascade
}

type ResponseCascade struct {
	Success bool
	Error   string
}

func init() {
	gob.Register(&RequestRPC{})
	gob.Register(&ResponseRPC{})
	gob.Register(&RequestCascade{})
	gob.Register(&ResponseCascade{})
}

func (rpc *rpcController) IAmRPCController() {
	// hack for DI, else we receive ServiceNetwork injection in RPCController instead of rpcController that leads to stack overflow
}

func (rpc *rpcController) RemoteProcedureRegister(name string, method core.RemoteProcedure) {
	rpc.methodTable[name] = method
}

func (rpc *rpcController) invoke(ctx context.Context, name string, data [][]byte) ([]byte, error) {
	method, exists := rpc.methodTable[name]
	if !exists {
		return nil, errors.New(fmt.Sprintf("RPC with name %s is not registered", name))
	}
	return method(ctx, data)
}

func (rpc *rpcController) SendCascadeMessage(data core.Cascade, method string, msg core.Parcel) error {
	if msg == nil {
		return errors.New("message is nil")
	}
	ctx := msg.Context(context.Background())
	return rpc.initCascadeSendMessage(ctx, data, false, method, [][]byte{message.ParcelToBytes(msg)})
}

func (rpc *rpcController) initCascadeSendMessage(ctx context.Context, data core.Cascade,
	findCurrentNode bool, method string, args [][]byte) error {

	if len(data.NodeIds) == 0 {
		return errors.New("node IDs list should not be empty")
	}
	if data.ReplicationFactor == 0 {
		return errors.New("replication factor should not be zero")
	}

	var nextNodes []core.RecordRef
	var err error

	if findCurrentNode {
		nodeID := rpc.hostNetwork.GetNodeID()
		nextNodes, err = cascade.CalculateNextNodes(rpc.Scheme, data, &nodeID)
	} else {
		nextNodes, err = cascade.CalculateNextNodes(rpc.Scheme, data, nil)
	}
	if err != nil {
		return errors.Wrap(err, "Failed to CalculateNextNodes")
	}
	if len(nextNodes) == 0 {
		return nil
	}

	var failedNodes []string
	for _, nextNode := range nextNodes {
		err = rpc.requestCascadeSendMessage(ctx, data, nextNode, method, args)
		if err != nil {
			inslogger.FromContext(ctx).Warnf("Failed to send cascade message to node %s: %s", nextNode, err.Error())
			failedNodes = append(failedNodes, nextNode.String())
		}
	}

	if len(failedNodes) > 0 {
		return errors.New("Failed to send cascade message to nodes: " + strings.Join(failedNodes, ", "))
	}
	inslogger.FromContext(ctx).Debug("Cascade message successfully sent to all nodes of the next layer")
	return nil
}

func (rpc *rpcController) requestCascadeSendMessage(ctx context.Context, data core.Cascade, nodeID core.RecordRef,
	method string, args [][]byte) error {

	request := rpc.hostNetwork.NewRequestBuilder().Type(types.Cascade).Data(&RequestCascade{
		TraceID: inslogger.TraceID(ctx),
		RPC: RequestRPC{
			Method: method,
			Data:   args,
		},
		Cascade: data,
	}).Build()

	future, err := rpc.hostNetwork.SendRequest(ctx, request, nodeID)
	if err != nil {
		return err
	}

	go func(ctx context.Context, f network.Future, duration time.Duration) {
		response, err := f.GetResponse(duration)
		if err != nil {
			inslogger.FromContext(ctx).Warnf("Failed to get response to cascade message request from node %s: %s",
				future.GetRequest().GetSender(), err.Error())
			return
		}
		data := response.GetData().(*ResponseCascade)
		if !data.Success {
			inslogger.FromContext(ctx).Warnf("Error response to cascade message request from node %s: %s",
				response.GetSender(), data.Error)
			return
		}
	}(ctx, future, rpc.options.PacketTimeout)

	return nil
}

func (rpc *rpcController) SendMessage(nodeID core.RecordRef, name string, msg core.Parcel) ([]byte, error) {
	msgBytes := message.ParcelToBytes(msg)
	metrics.ParcelsSentSizeBytes.WithLabelValues(msg.Type().String()).Observe(float64(len(msgBytes)))
	request := rpc.hostNetwork.NewRequestBuilder().Type(types.RPC).Data(&RequestRPC{
		Method: name,
		Data:   [][]byte{msgBytes},
	}).Build()

	start := time.Now()
	ctx := msg.Context(context.Background())
	logger := inslogger.FromContext(ctx)
	logger.Debugf("SendParcel with nodeID = %s method = %s, message reference = %s, RequestID = %d", nodeID.String(),
		name, msg.DefaultTarget().String(), request.GetRequestID())
	future, err := rpc.hostNetwork.SendRequest(ctx, request, nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "Error sending RPC request to node %s", nodeID.String())
	}
	response, err := future.GetResponse(rpc.options.PacketTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "Error getting RPC response from node %s", nodeID.String())
	}
	data := response.GetData().(*ResponseRPC)
	logger.Debugf("Inside SendParcel: type - '%s', target - %s, caller - %s, targetRole - %s, time - %s",
		msg.Type(), msg.DefaultTarget(), msg.GetCaller(), msg.DefaultRole(), time.Since(start))
	if !data.Success {
		return nil, errors.New("RPC call returned error: " + data.Error)
	}
	metrics.ParcelsReplySizeBytes.WithLabelValues(msg.Type().String()).Observe(float64(len(data.Result)))
	metrics.NetworkParcelSentTotal.WithLabelValues(msg.Type().String()).Inc()
	return data.Result, nil
}

func (rpc *rpcController) processMessage(ctx context.Context, request network.Request) (network.Response, error) {
	payload := request.GetData().(*RequestRPC)
	result, err := rpc.invoke(ctx, payload.Method, payload.Data)
	metrics.NetworkParcelReceivedTotal.WithLabelValues(request.GetType().String()).Inc()
	if err != nil {
		return rpc.hostNetwork.BuildResponse(ctx, request, &ResponseRPC{Success: false, Error: err.Error()}), nil
	}
	return rpc.hostNetwork.BuildResponse(ctx, request, &ResponseRPC{Success: true, Result: result}), nil
}

func (rpc *rpcController) processCascade(ctx context.Context, request network.Request) (network.Response, error) {
	payload := request.GetData().(*RequestCascade)
	ctx, logger := inslogger.WithTraceField(ctx, payload.TraceID)

	generalError := ""
	_, invokeErr := rpc.invoke(ctx, payload.RPC.Method, payload.RPC.Data)
	if invokeErr != nil {
		logger.Debugf("failed to invoke RPC: %s", invokeErr.Error())
		generalError += invokeErr.Error() + "; "
	}
	sendErr := rpc.initCascadeSendMessage(ctx, payload.Cascade, true, payload.RPC.Method, payload.RPC.Data)
	if sendErr != nil {
		logger.Debugf("failed to send message to next cascade layer: %s", sendErr.Error())
		generalError += sendErr.Error()
	}

	if generalError != "" {
		return rpc.hostNetwork.BuildResponse(ctx, request, &ResponseCascade{Success: false, Error: generalError}), nil
	}
	return rpc.hostNetwork.BuildResponse(ctx, request, &ResponseCascade{Success: true}), nil
}

func (rpc *rpcController) Start(ctx context.Context) error {
	rpc.hostNetwork.RegisterRequestHandler(types.RPC, rpc.processMessage)
	rpc.hostNetwork.RegisterRequestHandler(types.Cascade, rpc.processCascade)
	return nil
}

func NewRPCController(options *common.Options, hostNetwork network.HostNetwork) RPCController {
	return &rpcController{options: options,
		hostNetwork: hostNetwork,
		methodTable: make(map[string]core.RemoteProcedure),
	}
}
